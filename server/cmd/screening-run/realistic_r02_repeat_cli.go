package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func realisticR02RepeatPlan() (map[string]any, error) {
	repoRoot, err := findRealisticRepositoryRoot()
	if err != nil {
		return nil, err
	}
	admission, admissionHash, err := agentqualification.CurrentR02RepeatAdmission()
	if err != nil {
		return nil, err
	}
	evidenceErr := agentqualification.VerifyR02RepeatAdmissionEvidence(repoRoot, admission)
	matrix, matrixErr := agentqualification.R02RepeatBoundedMatrix(admission)
	freezeErr := agentqualification.RequireCompleteRealisticFreeze()
	providerReady := evidenceErr == nil && matrixErr == nil && freezeErr == nil && admission.Status == "frozen_reviewed" && admission.ProviderCallsAllowed
	return map[string]any{
		"status":                 "prevalidated_review_pending",
		"provider_called":        false,
		"provider_ready":         providerReady,
		"provider_calls_allowed": admission.ProviderCallsAllowed,
		"admission_id":           admission.AdmissionID,
		"admission_sha256":       admissionHash,
		"matrix_id":              admission.MatrixID,
		"matrix_sha256":          admission.MatrixSHA256,
		"configuration_id":       admission.ConfigurationID,
		"model":                  admission.Model,
		"effort":                 admission.Effort,
		"serving_provider":       admission.ServingProvider,
		"case_id":                admission.CaseID,
		"max_actions":            admission.MaxActions,
		"max_total_tokens":       admission.MaxTotalTokens,
		"max_generated_tokens":   admission.MaxGeneratedTokens,
		"max_spend_micros":       admission.MaxSpendMicros,
		"retries":                admission.Retries,
		"provider_fallbacks":     admission.ProviderFallbacks,
		"owner_interventions":    admission.OwnerInterventions,
		"global_halt_reasons":    admission.GlobalHaltReasons,
		"local_invalid_reasons":  admission.ObservationLocalInvalidReasons,
		"accepted_evidence":      admission.AcceptedEvidence,
		"evidence_error":         errorString(evidenceErr),
		"matrix_error":           errorString(matrixErr),
		"freeze_error":           errorString(freezeErr),
		"next_assignment":        matrix.Candidates,
	}, nil
}

func findRealisticRepositoryRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	current, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	for {
		if info, statErr := os.Stat(filepath.Join(current, "server", "go.mod")); statErr == nil && !info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("could not locate realistic benchmark repository root")
		}
		current = parent
	}
}
