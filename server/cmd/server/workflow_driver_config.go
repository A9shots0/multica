package main

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// workflowDriverFromEnv builds the workflow driver, or nil when it is not
// enabled for this deployment.
//
// Off by default on purpose. Enabling the driver makes the server dispatch
// stalled work on its own, which is the correct behaviour but also a visible
// change in what a board does while nobody is watching it. A deployment opts
// in explicitly:
//
//	MULTICA_WORKFLOW_DRIVER=on        dispatch for real
//	MULTICA_WORKFLOW_DRIVER=dry       log what it would do, change nothing
//	MULTICA_WORKFLOW_DRIVER_ISSUES    comma-separated issue numbers to limit to
//	MULTICA_WORKFLOW_DRIVER_INTERVAL  Go duration between passes
//	MULTICA_WORKFLOW_DRIVER_ATTEMPTS  resumes of one stage before asking the owner
//	MULTICA_WORKFLOW_DRIVER_ISSUE_TOKEN_BUDGET
//	                                  billable tokens one issue may consume
//	                                  before the driver pauses it and asks the
//	                                  owner. Negative disables the cap.
//	MULTICA_WORKFLOW_DRIVER_CAPACITY_BACKOFF
//	                                  how long to leave a stage alone after its
//	                                  run died on provider quota. Lower it after
//	                                  moving agents to a different provider, so
//	                                  work is not held back by an outage on a
//	                                  provider it no longer uses.
func workflowDriverFromEnv(pool *pgxpool.Pool, queries *db.Queries, taskSvc *service.TaskService, bus *events.Bus) *service.WorkflowDriver {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("MULTICA_WORKFLOW_DRIVER")))
	if mode != "on" && mode != "dry" {
		return nil
	}
	driver := &service.WorkflowDriver{
		Pool:        pool,
		Queries:     queries,
		TaskService: taskSvc,
		Bus:         bus,
		DryRun:      mode == "dry",
	}
	if raw := strings.TrimSpace(os.Getenv("MULTICA_WORKFLOW_DRIVER_ISSUES")); raw != "" {
		for _, field := range strings.Split(raw, ",") {
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			number, err := strconv.ParseInt(field, 10, 32)
			if err != nil {
				// A typo here would silently widen the driver to the whole
				// board, which is the opposite of what the variable is for.
				slog.Error("ignoring invalid MULTICA_WORKFLOW_DRIVER_ISSUES entry", "value", field, "error", err)
				continue
			}
			driver.OnlyIssues = append(driver.OnlyIssues, int32(number))
		}
	}
	if raw := strings.TrimSpace(os.Getenv("MULTICA_WORKFLOW_DRIVER_INTERVAL")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			driver.Interval = parsed
		} else {
			slog.Error("ignoring invalid MULTICA_WORKFLOW_DRIVER_INTERVAL", "value", raw, "error", err)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("MULTICA_WORKFLOW_DRIVER_CAPACITY_BACKOFF")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			driver.CapacityBackoff = parsed
		} else {
			slog.Error("ignoring invalid MULTICA_WORKFLOW_DRIVER_CAPACITY_BACKOFF", "value", raw, "error", err)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("MULTICA_WORKFLOW_DRIVER_ISSUE_TOKEN_BUDGET")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			driver.IssueTokenBudget = parsed
		} else {
			slog.Error("ignoring invalid MULTICA_WORKFLOW_DRIVER_ISSUE_TOKEN_BUDGET", "value", raw, "error", err)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("MULTICA_WORKFLOW_DRIVER_ATTEMPTS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			driver.MaxAttempts = parsed
		} else {
			slog.Error("ignoring invalid MULTICA_WORKFLOW_DRIVER_ATTEMPTS", "value", raw, "error", err)
		}
	}
	return driver
}
