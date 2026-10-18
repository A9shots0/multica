package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const MinimumScreeningSuiteSchemaVersion = "silver-mediums.minimum-screening-suite.v1"

type MinimumScreeningSuite struct {
	SchemaVersion     string          `json:"schema_version"`
	SuiteID           string          `json:"suite_id"`
	Status            string          `json:"status"`
	ProgressionPolicy string          `json:"progression_policy"`
	Cases             []ScreeningCase `json:"cases"`
}

//go:embed minimum_screening_suite_v1.json
var minimumScreeningSuiteJSON []byte

func CurrentMinimumScreeningSuite() (MinimumScreeningSuite, error) {
	var suite MinimumScreeningSuite
	if err := json.Unmarshal(minimumScreeningSuiteJSON, &suite); err != nil {
		return suite, fmt.Errorf("decode minimum suite: %w", err)
	}
	if err := ValidateMinimumScreeningSuite(suite); err != nil {
		return MinimumScreeningSuite{}, err
	}
	return suite, nil
}

func ValidateMinimumScreeningSuite(suite MinimumScreeningSuite) error {
	if suite.SchemaVersion != MinimumScreeningSuiteSchemaVersion || suite.SuiteID == "" {
		return fmt.Errorf("minimum suite identity is incomplete")
	}
	if suite.Status != "calibrated_partially_executed" {
		return fmt.Errorf("minimum suite cannot imply execution")
	}
	if suite.ProgressionPolicy != "t12_pair_then_survivors_remaining_cases" {
		return fmt.Errorf("minimum suite progression is not usage protected")
	}
	want := []string{"T12:v4", "T11:v4", "T08:v2", "T16:v2"}
	if len(suite.Cases) != len(want) {
		return fmt.Errorf("minimum suite must contain four cases")
	}
	for index, screeningCase := range suite.Cases {
		if screeningCase.CaseID+":"+screeningCase.CaseVersion != want[index] {
			return fmt.Errorf("minimum suite case order changed")
		}
		candidateHash, err := EmbeddedScreeningPacketVersionSHA256(screeningCase.CaseID, screeningCase.CaseVersion, "candidate")
		if err != nil || candidateHash != screeningCase.CandidatePacketSHA256 {
			return fmt.Errorf("case %s candidate hash mismatch", screeningCase.CaseID)
		}
		graderHash, err := EmbeddedScreeningPacketVersionSHA256(screeningCase.CaseID, screeningCase.CaseVersion, "grader")
		if err != nil || graderHash != screeningCase.GraderPacketSHA256 {
			return fmt.Errorf("case %s grader hash mismatch", screeningCase.CaseID)
		}
		if err := validatePublicResponseContract(screeningCase.CaseID, screeningCase.CaseVersion); err != nil {
			return err
		}
	}
	return nil
}
