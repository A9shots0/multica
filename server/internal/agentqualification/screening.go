package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type ScreeningAssignment struct {
	BatchID               string          `json:"batch_id"`
	CaseID                string          `json:"case_id"`
	CaseVersion           string          `json:"case_version"`
	Attempt               int             `json:"attempt"`
	Configuration         Configuration   `json:"configuration"`
	CandidatePacket       json.RawMessage `json:"candidate_packet"`
	CandidatePacketSHA256 string          `json:"candidate_packet_sha256"`
	EvaluationBinding     string          `json:"evaluation_binding"`
}

type BlindGradingSubmission struct {
	SchemaVersion         string          `json:"schema_version"`
	SubmissionID          string          `json:"submission_id"`
	BatchID               string          `json:"batch_id"`
	CaseID                string          `json:"case_id"`
	CaseVersion           string          `json:"case_version"`
	GraderVersion         string          `json:"grader_version"`
	CandidatePacketSHA256 string          `json:"candidate_packet_sha256"`
	ResponseSHA256        string          `json:"response_sha256"`
	CandidateResponse     json.RawMessage `json:"candidate_response"`
}

// ScreeningSubmissionBinding is stored outside the grader payload. Joining it
// before the outcome verdict is sealed defeats candidate-blind scoring.
type ScreeningSubmissionBinding struct {
	SubmissionID        string `json:"submission_id"`
	CandidateID         string `json:"candidate_id"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	EvaluationBinding   string `json:"evaluation_binding"`
}

type PreparedScreeningLaunch struct {
	Admission  ScreeningLaunchAdmission `json:"admission"`
	Assignment *ScreeningAssignment     `json:"assignment,omitempty"`
}

// PrepareNextScreeningLaunch is the only adapter that may hand a paid
// assignment to a runtime. It first applies the complete budget/capacity gate,
// then selects the first seeded assignment that has no recorded observation.
func PrepareNextScreeningLaunch(batch ScreeningBatchDefinition, ledger BatchUsageLedger, baseline, current CapacitySnapshot, evaluatedAt string) (PreparedScreeningLaunch, error) {
	return prepareScreeningAssignment(batch, ledger, EvaluateScreeningLaunch(batch, ledger, baseline, current, evaluatedAt))
}

func PrepareNextScreeningAPILaunch(batch ScreeningBatchDefinition, ledger BatchUsageLedger, snapshot APISpendSnapshot, evaluatedAt string) (PreparedScreeningLaunch, error) {
	return prepareScreeningAssignment(batch, ledger, EvaluateScreeningAPILaunch(batch, ledger, snapshot, evaluatedAt))
}

func prepareScreeningAssignment(batch ScreeningBatchDefinition, ledger BatchUsageLedger, admission ScreeningLaunchAdmission) (PreparedScreeningLaunch, error) {
	prepared := PreparedScreeningLaunch{Admission: admission}
	if !prepared.Admission.CanLaunch {
		return prepared, nil
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		return PreparedScreeningLaunch{}, err
	}
	completed := make(map[string]struct{}, len(ledger.ObservationReceipts))
	eliminatedCandidates := make(map[string]struct{})
	for _, observation := range ledger.ObservationReceipts {
		if observation.EvaluationBinding != "" {
			completed[observation.EvaluationBinding] = struct{}{}
		}
		validity := ValidateObservation(observation)
		if validity.Valid && observation.TrialStatus == "failed" && (observation.FailureClass == "model" || len(observation.Disqualifiers) > 0) {
			eliminatedCandidates[observation.CandidateID] = struct{}{}
		}
	}
	for _, assignment := range assignments {
		if _, alreadyCompleted := completed[assignment.EvaluationBinding]; alreadyCompleted {
			continue
		}
		if _, eliminated := eliminatedCandidates[assignment.Configuration.ID]; eliminated {
			continue
		}
		copy := assignment
		prepared.Assignment = &copy
		return prepared, nil
	}
	return PreparedScreeningLaunch{}, fmt.Errorf("launch admission passed without a remaining assignment")
}

func ScreeningAssignments(batch ScreeningBatchDefinition) ([]ScreeningAssignment, error) {
	if err := ValidateScreeningBatch(batch); err != nil {
		return nil, err
	}
	type orderedCandidate struct {
		configuration Configuration
		key           string
	}
	ordered := make([]orderedCandidate, 0, len(batch.Candidates))
	for _, candidate := range batch.Candidates {
		sum := sha256.Sum256([]byte(batch.ExecutionOrderSeed + ":" + candidate.ID))
		ordered = append(ordered, orderedCandidate{configuration: candidate, key: hex.EncodeToString(sum[:])})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].key == ordered[j].key {
			return ordered[i].configuration.ID < ordered[j].configuration.ID
		}
		return ordered[i].key < ordered[j].key
	})

	assignments := make([]ScreeningAssignment, 0, len(batch.Cases)*len(batch.Candidates)*batch.Repeats)
	for attempt := 1; attempt <= batch.Repeats; attempt++ {
		for _, screeningCase := range batch.Cases {
			packet, err := CandidateScreeningPacketVersion(screeningCase.CaseID, screeningCase.CaseVersion)
			if err != nil {
				return nil, err
			}
			for _, candidate := range ordered {
				binding, err := screeningEvaluationBinding(batch, screeningCase, candidate.configuration, attempt)
				if err != nil {
					return nil, err
				}
				assignments = append(assignments, ScreeningAssignment{
					BatchID: batch.BatchID, CaseID: screeningCase.CaseID, CaseVersion: screeningCase.CaseVersion,
					Attempt: attempt, Configuration: candidate.configuration, CandidatePacket: append(json.RawMessage(nil), packet...),
					CandidatePacketSHA256: screeningCase.CandidatePacketSHA256, EvaluationBinding: binding,
				})
			}
		}
	}
	return assignments, nil
}

func BuildBlindGradingSubmission(batch ScreeningBatchDefinition, assignment ScreeningAssignment, candidateResponse []byte) (BlindGradingSubmission, ScreeningSubmissionBinding, error) {
	if !json.Valid(candidateResponse) {
		return BlindGradingSubmission{}, ScreeningSubmissionBinding{}, fmt.Errorf("candidate response must be valid JSON")
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		return BlindGradingSubmission{}, ScreeningSubmissionBinding{}, err
	}
	known := false
	for _, planned := range assignments {
		if planned.EvaluationBinding == assignment.EvaluationBinding && planned.Configuration.ID == assignment.Configuration.ID {
			known = true
			break
		}
	}
	if !known {
		return BlindGradingSubmission{}, ScreeningSubmissionBinding{}, fmt.Errorf("assignment is not in the immutable batch")
	}
	responseSum := sha256.Sum256(candidateResponse)
	responseHash := hex.EncodeToString(responseSum[:])
	submissionSum := sha256.Sum256([]byte(assignment.EvaluationBinding + ":" + responseHash))
	submissionID := hex.EncodeToString(submissionSum[:])
	configurationHash, err := ConfigurationSHA256(assignment.Configuration)
	if err != nil {
		return BlindGradingSubmission{}, ScreeningSubmissionBinding{}, err
	}
	return BlindGradingSubmission{
			SchemaVersion: "silver-mediums.blind-grading-submission.v1", SubmissionID: submissionID,
			BatchID: batch.BatchID, CaseID: assignment.CaseID, CaseVersion: assignment.CaseVersion,
			GraderVersion: batch.GraderConfigurationID, CandidatePacketSHA256: assignment.CandidatePacketSHA256,
			ResponseSHA256: responseHash, CandidateResponse: append(json.RawMessage(nil), candidateResponse...),
		}, ScreeningSubmissionBinding{
			SubmissionID: submissionID, CandidateID: assignment.Configuration.ID,
			ConfigurationSHA256: configurationHash, EvaluationBinding: assignment.EvaluationBinding,
		}, nil
}

func screeningEvaluationBinding(batch ScreeningBatchDefinition, screeningCase ScreeningCase, configuration Configuration, attempt int) (string, error) {
	configurationHash, err := ConfigurationSHA256(configuration)
	if err != nil {
		return "", err
	}
	payload := struct {
		BatchID               string `json:"batch_id"`
		CaseID                string `json:"case_id"`
		CaseVersion           string `json:"case_version"`
		Attempt               int    `json:"attempt"`
		ConfigurationSHA256   string `json:"configuration_sha256"`
		CandidatePacketSHA256 string `json:"candidate_packet_sha256"`
		GraderPacketSHA256    string `json:"grader_packet_sha256"`
	}{batch.BatchID, screeningCase.CaseID, screeningCase.CaseVersion, attempt, configurationHash, screeningCase.CandidatePacketSHA256, screeningCase.GraderPacketSHA256}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
