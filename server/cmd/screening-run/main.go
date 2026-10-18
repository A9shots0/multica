package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

// screening-run is deliberately plan-only. A paid executor must not exist
// until it consumes the single fail-closed admission path, writes a durable
// provenance-complete receipt, strips unrelated environment values, and can
// prevent (rather than merely notice) tool execution.
func main() {
	var candidateID string
	var caseID string
	var executePaid bool
	var printSuite bool
	var openRouterPlan bool
	var runtimePreflight bool
	var artifactRoot string
	var reconcileOpenRouterDir string
	var gradeOpenRouterDir string
	var finalizeOpenRouterDir string
	var finalizeOpenRouterResponseFailureDir string
	var realisticPlanOnly bool
	var realisticExecute bool
	var realisticResume bool
	var realisticBrowserServe bool
	var realisticR02RepeatPlanOnly bool
	var realisticR02RepeatExecute bool
	var realisticR02RepeatResume bool
	var realisticR02RepeatInvalidateBrowser bool
	var realisticCohort string
	var realisticMatrixID string
	var topologyPlanOnly bool
	var topologyExecute bool
	var topologyObservationID string
	var topologyRecover bool
	var topologyReclassifyPreventiveBudget bool
	var topologyReclassifyPrecallHarness bool
	var claudeCalibrationPlanOnly bool
	var claudeCalibrationExecute bool
	var realisticNextPlanOnly bool
	var realisticNextExecute bool
	var item14OpenRouterDirectMode string
	var item14OpenRouterDirectV3Mode string
	var item14OpenRouterDirectV4Mode string
	flag.StringVar(&candidateID, "candidate", "", "candidate id; empty selects the seeded first candidate")
	flag.StringVar(&caseID, "case", "T12", "screening case id")
	flag.BoolVar(&executePaid, "execute", false, "reserved; paid execution is fail-closed and currently disabled")
	flag.BoolVar(&printSuite, "suite", false, "print the calibrated minimum-suite manifest without calling a provider")
	flag.BoolVar(&openRouterPlan, "openrouter-plan", false, "print the immutable capped OpenRouter batch and seeded first plan without calling a provider")
	flag.BoolVar(&runtimePreflight, "runtime-preflight", false, "resolve and validate the isolated OpenCode runtime without calling a model")
	flag.StringVar(&artifactRoot, "artifact-root", "", "absolute write-once evidence root for the dedicated OpenRouter execution path")
	flag.StringVar(&reconcileOpenRouterDir, "reconcile-openrouter-dir", "", "normalize an existing sealed OpenRouter observation without a provider call")
	flag.StringVar(&gradeOpenRouterDir, "grade-openrouter-dir", "", "blind-grade an existing sealed OpenRouter observation without a provider call")
	flag.StringVar(&finalizeOpenRouterDir, "finalize-openrouter-dir", "", "close a receipted and graded OpenRouter observation into the batch ledger")
	flag.StringVar(&finalizeOpenRouterResponseFailureDir, "finalize-openrouter-response-contract-failure-dir", "", "close sealed invalid-JSON OpenRouter output as a candidate failure without another provider call")
	flag.BoolVar(&realisticPlanOnly, "realistic-plan", false, "show the next frozen realistic assignments without a provider call")
	flag.BoolVar(&realisticExecute, "realistic-execute", false, "execute one admitted realistic observation")
	flag.BoolVar(&realisticResume, "realistic-resume", false, "resume one durable realistic interruption or Browser handoff")
	flag.BoolVar(&realisticBrowserServe, "realistic-browser-serve", false, "serve one pending R02 fixture on loopback for the in-app Browser")
	flag.BoolVar(&realisticR02RepeatPlanOnly, "realistic-r02-repeat-plan", false, "validate the generation-owned R02 repeat admission without calling a provider")
	flag.BoolVar(&realisticR02RepeatExecute, "realistic-r02-repeat-execute", false, "execute the exact active generation-owned R02 repeat")
	flag.BoolVar(&realisticR02RepeatResume, "realistic-r02-repeat-resume", false, "resume the exact active generation-owned R02 repeat")
	flag.BoolVar(&realisticR02RepeatInvalidateBrowser, "realistic-r02-repeat-invalidate-browser-capability", false, "seal the exact Browser-blocked R02 repeat invalid without retry or provider call")
	flag.StringVar(&realisticCohort, "realistic-cohort", "", "comma-separated exact candidate ids for a bounded follow-up cohort")
	flag.StringVar(&realisticMatrixID, "realistic-matrix", "realistic-foundation-screen-v1", "exact frozen realistic matrix id")
	flag.BoolVar(&topologyPlanOnly, "topology-plan", false, "show the next frozen topology observation without a provider call")
	flag.BoolVar(&topologyExecute, "topology-execute", false, "execute the next exact topology observation")
	flag.StringVar(&topologyObservationID, "topology-observation", "", "exact next topology observation id")
	flag.BoolVar(&topologyRecover, "topology-recover", false, "preserve and invalidate one crash-stranded topology observation without retrying a provider")
	flag.BoolVar(&topologyReclassifyPreventiveBudget, "topology-reclassify-preventive-budget", false, "reclassify a below-ceiling preventive reservation stop as one invalid observation without retry")
	flag.BoolVar(&topologyReclassifyPrecallHarness, "topology-reclassify-precall-harness", false, "reclassify the validated zero-call reviewer transport registration defect without retry")
	flag.BoolVar(&claudeCalibrationPlanOnly, "claude-calibration-plan", false, "show the frozen one-call Claude subscription calibration without calling a provider")
	flag.BoolVar(&claudeCalibrationExecute, "claude-calibration-execute", false, "execute the exact frozen one-call Claude subscription calibration")
	flag.BoolVar(&realisticNextPlanOnly, "realistic-next-plan", false, "show the pending or frozen R05/R06 capability matrix without calling a provider")
	flag.BoolVar(&realisticNextExecute, "realistic-next-execute", false, "execute the exact reviewed and frozen R05/R06 capability matrix")
	flag.StringVar(&item14OpenRouterDirectMode, "item14-openrouter-direct-v2", "", "Item 14 direct OpenRouter shadow mode: plan, fake, prepare, or execute")
	flag.StringVar(&item14OpenRouterDirectV3Mode, "item14-openrouter-direct-v3", "", "Item 14 direct OpenRouter v3 mode: plan, fake, prepare, or execute")
	flag.StringVar(&item14OpenRouterDirectV4Mode, "item14-openrouter-direct-v4", "", "Item 14 direct OpenRouter v4 mode: plan, fake, prepare, or execute")
	flag.Parse()
	if item14OpenRouterDirectV4Mode != "" {
		var result any
		var err error
		switch item14OpenRouterDirectV4Mode {
		case "plan":
			result, err = item14OpenRouterDirectPlanV4()
		case "fake":
			result, err = item14OpenRouterDirectFakeV4()
		case "prepare":
			result, err = item14OpenRouterDirectPrepareV4()
		case "execute":
			if artifactRoot == "" {
				var cwd string
				cwd, err = os.Getwd()
				if err == nil {
					artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "item14-openrouter-direct-shadow-v4")
				}
			}
			if err == nil {
				artifactRoot, err = filepath.Abs(artifactRoot)
			}
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()
				result, err = executeItem14OpenRouterDirectRealV4(ctx, artifactRoot)
			}
		default:
			err = fmt.Errorf("unknown Item 14 OpenRouter direct v4 mode %q", item14OpenRouterDirectV4Mode)
		}
		fatalIf(err)
		writeJSON(result)
		return
	}
	if item14OpenRouterDirectV3Mode != "" {
		var result any
		var err error
		switch item14OpenRouterDirectV3Mode {
		case "plan":
			result, err = item14OpenRouterDirectPlanV3()
		case "fake":
			result, err = item14OpenRouterDirectFakeV3()
		case "prepare":
			result, err = item14OpenRouterDirectPrepareV3()
		case "execute":
			if artifactRoot == "" {
				var cwd string
				cwd, err = os.Getwd()
				if err == nil {
					artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "item14-openrouter-direct-shadow-v3")
				}
			}
			if err == nil {
				artifactRoot, err = filepath.Abs(artifactRoot)
			}
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()
				result, err = executeItem14OpenRouterDirectRealV3(ctx, artifactRoot)
			}
		case "reconcile":
			if artifactRoot == "" {
				var cwd string
				cwd, err = os.Getwd()
				if err == nil {
					artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "item14-openrouter-direct-shadow-v3")
				}
			}
			if err == nil {
				artifactRoot, err = filepath.Abs(artifactRoot)
			}
			if err == nil {
				result, err = reconcileItem14OpenRouterDirectV3(artifactRoot)
			}
		default:
			err = fmt.Errorf("unknown Item 14 OpenRouter direct v3 mode %q", item14OpenRouterDirectV3Mode)
		}
		fatalIf(err)
		writeJSON(result)
		return
	}
	if item14OpenRouterDirectMode != "" {
		var result any
		var err error
		switch item14OpenRouterDirectMode {
		case "plan":
			result, err = item14OpenRouterDirectPlanV2()
		case "fake":
			result, err = item14OpenRouterDirectFakeV2()
		case "prepare":
			result, err = item14OpenRouterDirectPrepareV2()
		case "execute":
			if artifactRoot == "" {
				var cwd string
				cwd, err = os.Getwd()
				if err == nil {
					artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "item14-openrouter-direct-shadow-v2")
				}
			}
			if err == nil {
				artifactRoot, err = filepath.Abs(artifactRoot)
			}
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()
				result, err = executeItem14OpenRouterDirectRealV2(ctx, artifactRoot)
			}
		case "reconcile":
			if artifactRoot == "" {
				var cwd string
				cwd, err = os.Getwd()
				if err == nil {
					artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "item14-openrouter-direct-shadow-v2")
				}
			}
			if err == nil {
				artifactRoot, err = filepath.Abs(artifactRoot)
			}
			if err == nil {
				result, err = reconcileItem14OpenRouterDirectV2(artifactRoot)
			}
		default:
			err = fmt.Errorf("unknown Item 14 OpenRouter direct v2 mode %q", item14OpenRouterDirectMode)
		}
		fatalIf(err)
		writeJSON(result)
		return
	}
	if realisticNextPlanOnly || realisticNextExecute {
		if realisticNextPlanOnly {
			result, err := realisticNextPlan()
			fatalIf(err)
			writeJSON(result)
			return
		}
		if artifactRoot == "" {
			cwd, err := os.Getwd()
			fatalIf(err)
			artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "realistic-next-v1")
		}
		var err error
		artifactRoot, err = filepath.Abs(artifactRoot)
		fatalIf(err)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		result, executionErr := executeRealisticNextMatrix(ctx, artifactRoot)
		fatalIf(executionErr)
		writeJSON(result)
		return
	}
	if claudeCalibrationPlanOnly || claudeCalibrationExecute {
		if claudeCalibrationPlanOnly {
			result, err := claudeCalibrationPlan()
			fatalIf(err)
			writeJSON(result)
			return
		}
		if artifactRoot == "" {
			cwd, err := os.Getwd()
			fatalIf(err)
			artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "claude-subscription-v29")
		}
		var err error
		artifactRoot, err = filepath.Abs(artifactRoot)
		fatalIf(err)
		ctx, cancel := context.WithTimeout(context.Background(), 125*time.Second)
		defer cancel()
		result, executionErr := executeClaudeCalibration(ctx, artifactRoot)
		fatalIf(executionErr)
		writeJSON(result)
		return
	}
	if realisticR02RepeatPlanOnly {
		result, err := realisticR02RepeatPlan()
		fatalIf(err)
		writeJSON(result)
		return
	}
	if realisticR02RepeatInvalidateBrowser {
		if artifactRoot == "" {
			cwd, err := os.Getwd()
			fatalIf(err)
			artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "realistic-r02-repeat-v1")
		}
		var err error
		artifactRoot, err = filepath.Abs(artifactRoot)
		fatalIf(err)
		result, correctionErr := invalidateR02RepeatForBrowserCapability(artifactRoot)
		fatalIf(correctionErr)
		writeJSON(result)
		return
	}
	if realisticR02RepeatExecute || realisticR02RepeatResume {
		if artifactRoot == "" {
			cwd, err := os.Getwd()
			fatalIf(err)
			artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "realistic-r02-repeat-v1")
		}
		var err error
		artifactRoot, err = filepath.Abs(artifactRoot)
		fatalIf(err)
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
		defer cancel()
		result, executionErr := executeR02RepeatObservation(ctx, artifactRoot, realisticR02RepeatResume)
		fatalIf(executionErr)
		writeJSON(result)
		return
	}
	if topologyPlanOnly || topologyExecute || topologyRecover || topologyReclassifyPreventiveBudget || topologyReclassifyPrecallHarness {
		if artifactRoot == "" {
			cwd, err := os.Getwd()
			fatalIf(err)
			artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "topology-v1")
		}
		var err error
		artifactRoot, err = filepath.Abs(artifactRoot)
		fatalIf(err)
		if topologyPlanOnly {
			result, planErr := topologyPlan(artifactRoot)
			fatalIf(planErr)
			writeJSON(result)
			return
		}
		if topologyRecover {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			result, recoveryErr := recoverTopologyObservation(ctx, artifactRoot)
			fatalIf(recoveryErr)
			writeJSON(result)
			return
		}
		if topologyReclassifyPreventiveBudget {
			result, correctionErr := reclassifyTopologyPreventiveBudgetHalt(artifactRoot)
			fatalIf(correctionErr)
			writeJSON(result)
			return
		}
		if topologyReclassifyPrecallHarness {
			result, correctionErr := reclassifyTopologyPrecallHarnessHalt(artifactRoot)
			fatalIf(correctionErr)
			writeJSON(result)
			return
		}
		if topologyObservationID == "" {
			fatalIf(fmt.Errorf("topology execution requires the exact next observation id"))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		result, executionErr := executeTopologyObservation(ctx, artifactRoot, topologyObservationID)
		fatalIf(executionErr)
		writeJSON(result)
		return
	}
	if realisticPlanOnly || realisticExecute || realisticResume || realisticBrowserServe {
		if artifactRoot == "" {
			cwd, err := os.Getwd()
			fatalIf(err)
			artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "realistic-v1")
		}
		var err error
		artifactRoot, err = filepath.Abs(artifactRoot)
		fatalIf(err)
		cohort := splitExactCandidateIDs(realisticCohort)
		if realisticPlanOnly {
			result, planErr := realisticPlanForMatrix(artifactRoot, realisticMatrixID, cohort)
			fatalIf(planErr)
			writeJSON(result)
			return
		}
		if realisticBrowserServe {
			if candidateID == "" {
				fatalIf(fmt.Errorf("Browser preview requires exact candidate"))
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fatalIf(serveRealisticBrowserPreview(ctx, artifactRoot, candidateID))
			return
		}
		if candidateID == "" || caseID == "" {
			fatalIf(fmt.Errorf("realistic execution requires exact candidate and case"))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
		defer cancel()
		result, executionErr := executeRealisticObservationForMatrix(ctx, artifactRoot, candidateID, caseID, realisticResume, realisticMatrixID, cohort)
		fatalIf(executionErr)
		writeJSON(result)
		return
	}
	if printSuite {
		suite, err := agentqualification.CurrentMinimumScreeningSuite()
		fatalIf(err)
		writeJSON(suite)
		return
	}
	if openRouterPlan {
		batch, err := agentqualification.OpenRouterScreeningBatch()
		fatalIf(err)
		assignments, err := agentqualification.ScreeningAssignments(batch)
		fatalIf(err)
		var screeningCase agentqualification.ScreeningCase
		for _, candidateCase := range batch.Cases {
			if candidateCase.CaseID == assignments[0].CaseID && candidateCase.CaseVersion == assignments[0].CaseVersion {
				screeningCase = candidateCase
				break
			}
		}
		plan, err := agentqualification.BuildOpenCodePlanForTransport(batch.CandidateTransport, assignments[0].Configuration, screeningCase)
		fatalIf(err)
		if finalizeOpenRouterResponseFailureDir != "" {
			finalizeOpenRouterResponseFailureDir, err = filepath.Abs(finalizeOpenRouterResponseFailureDir)
			fatalIf(err)
			reservationPlan, planErr := openRouterPlanForReservationDir(finalizeOpenRouterResponseFailureDir, batch)
			fatalIf(planErr)
			record, observationPath, finalizeErr := agentqualification.FinalizeSealedOpenCodeResponseContractFailure(finalizeOpenRouterResponseFailureDir, batch, openRouterExecutionAdapter(reservationPlan))
			fatalIf(finalizeErr)
			writeJSON(map[string]any{"status": "finalized_response_contract_failure", "provider_called": false, "candidate_id": record.CandidateID, "trial_status": record.TrialStatus, "disqualifiers": record.Disqualifiers, "observation_path": observationPath})
			return
		}
		if finalizeOpenRouterDir != "" {
			finalizeOpenRouterDir, err = filepath.Abs(finalizeOpenRouterDir)
			fatalIf(err)
			record, observationPath, finalizeErr := agentqualification.FinalizeSealedOpenCodeObservation(finalizeOpenRouterDir, batch)
			fatalIf(finalizeErr)
			writeJSON(map[string]any{"status": "finalized", "provider_called": false, "candidate_id": record.CandidateID, "trial_status": record.TrialStatus, "observation_path": observationPath})
			return
		}
		if gradeOpenRouterDir != "" {
			gradeOpenRouterDir, err = filepath.Abs(gradeOpenRouterDir)
			fatalIf(err)
			grade, gradePath, gradeErr := agentqualification.GradeSealedOpenCodeEvidenceForBatch(gradeOpenRouterDir, batch)
			fatalIf(gradeErr)
			writeJSON(map[string]any{"status": "graded", "provider_called": false, "verdict": grade.Verdict, "passed_checks": grade.PassedChecks, "failed_checks": grade.FailedChecks, "disqualifiers": grade.Disqualifiers, "grade_path": gradePath})
			return
		}
		if reconcileOpenRouterDir != "" {
			reconcileOpenRouterDir, err = filepath.Abs(reconcileOpenRouterDir)
			fatalIf(err)
			reservationPlan, planErr := openRouterPlanForReservationDir(reconcileOpenRouterDir, batch)
			fatalIf(planErr)
			completed, reconcileErr := agentqualification.ReconcileOpenCodeReservedRun(reconcileOpenRouterDir, batch, openRouterExecutionAdapter(reservationPlan))
			fatalIf(reconcileErr)
			writeJSON(map[string]any{"status": "reconciled", "provider_called": false, "model": completed.Receipt.ReportedModel, "actual_serving_provider": completed.Receipt.ActualServingProvider, "serving_provider_evidence": completed.Receipt.ServingProviderEvidence, "observation_id": completed.Receipt.ObservationID, "execution_receipt_path": completed.ExecutionReceiptPath, "provider_reported_cost_micros": completed.Receipt.Usage.ProviderReportedCostMicros})
			return
		}
		resolvedConfigHash := "not_run"
		resolvedConfigHashes := map[string]string{}
		if runtimePreflight {
			for _, candidate := range batch.Candidates {
				candidatePlan, planErr := agentqualification.BuildOpenCodePlanForTransport(batch.CandidateTransport, candidate, batch.Cases[0])
				fatalIf(planErr)
				runtimeState, runtimeErr := newOpenCodeCommandRuntime(candidatePlan)
				fatalIf(runtimeErr)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				resolvedConfig, resolveErr := runtimeState.resolvedConfig(ctx)
				cancel()
				if resolveErr != nil {
					fatalIf(fmt.Errorf("candidate %s preflight: %w", candidate.ID, resolveErr))
				}
				hash, validateErr := agentqualification.ValidateOpenCodeResolvedConfig(candidatePlan, resolvedConfig)
				fatalIf(validateErr)
				resolvedConfigHashes[candidate.ID] = hash
			}
			resolvedConfigHash = resolvedConfigHashes[assignments[0].Configuration.ID]
		}
		if executePaid {
			if artifactRoot == "" {
				cwd, cwdErr := os.Getwd()
				fatalIf(cwdErr)
				artifactRoot = filepath.Join(cwd, "..", ".qualification-evidence", "openrouter-v26")
			}
			artifactRoot, err = filepath.Abs(artifactRoot)
			fatalIf(err)
			ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
			defer cancel()
			completed, executionErr := executeOneOpenRouterObservation(ctx, artifactRoot)
			fatalIf(executionErr)
			writeJSON(map[string]any{
				"status": "completed_once", "paid_model_called": true, "automatic_retry": false,
				"batch_id": completed.Receipt.BatchID, "candidate_id": completed.Prepared.Assignment.Configuration.ID,
				"model": completed.Receipt.ReportedModel, "reasoning_effort": completed.Receipt.RequestedEffort,
				"observation_id": completed.Receipt.ObservationID, "provider_reported_cost_micros": completed.Receipt.Usage.ProviderReportedCostMicros,
				"input_tokens": completed.Receipt.Usage.InputTokens, "output_tokens": completed.Receipt.Usage.OutputTokens,
				"reasoning_tokens": completed.Receipt.Usage.ReasoningTokens, "reservation_dir": completed.ReservationDir,
				"execution_receipt_path": completed.ExecutionReceiptPath,
			})
			return
		}
		writeJSON(map[string]any{
			"status":                              "dry_run_valid",
			"paid_model_called":                   false,
			"paid_execution_enabled":              false,
			"batch_id":                            batch.BatchID,
			"planned_observations":                len(assignments),
			"seeded_first_candidate":              assignments[0].Configuration.ID,
			"model":                               plan.Model,
			"reasoning_effort":                    plan.Effort,
			"case_id":                             plan.CaseID,
			"case_version":                        plan.CaseVersion,
			"prompt_sha256":                       plan.PromptSHA256,
			"output_schema_sha256":                plan.SchemaSHA256,
			"inline_config_sha256":                plan.InlineConfigSHA256,
			"resolved_config_sha256":              resolvedConfigHash,
			"resolved_config_sha256_by_candidate": resolvedConfigHashes,
			"runtime_preflight_ran":               runtimePreflight,
			"token_ceiling":                       batch.TokenCeiling,
			"api_spend_ceiling_micros":            batch.APIBudgets[0].MaxSpendMicros,
			"provider_call_allowed":               false,
		})
		return
	}

	batch, err := agentqualification.CurrentScreeningBatch()
	fatalIf(err)
	if candidateID == "" {
		assignments, assignmentErr := agentqualification.ScreeningAssignments(batch)
		fatalIf(assignmentErr)
		candidateID = assignments[0].Configuration.ID
	}
	plan, err := agentqualification.BuildDirectScreeningPlan(batch, candidateID, caseID)
	fatalIf(err)
	if executePaid {
		fatalIf(paidExecutionDisabled())
	}
	writeJSON(map[string]any{
		"status":                 "dry_run_valid",
		"paid_model_called":      false,
		"paid_execution_enabled": false,
		"batch_id":               plan.BatchID,
		"candidate_id":           plan.CandidateID,
		"case_id":                plan.CaseID,
		"model":                  plan.Model,
		"reasoning_effort":       plan.Effort,
		"prompt_sha256":          plan.PromptSHA256,
		"output_schema_sha256":   plan.SchemaSHA256,
		"prompt_bytes":           len([]byte(plan.Prompt)),
		"args_template":          plan.ArgsTemplate,
		"required_tool_policy":   "none",
		"runtime_enforcement":    "not_implemented",
		"execution_blockers": []string{
			"atomic reservation is not integrated with a provider adapter",
			"preventive tool isolation transport is not selected",
			"provider events are not normalized into the sealed receipt",
		},
	})
}

func paidExecutionDisabled() error {
	return fmt.Errorf("paid execution disabled: admission, isolation, and provenance gates are not complete")
}

func writeJSON(value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	fatalIf(err)
	fmt.Println(string(data))
}

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
