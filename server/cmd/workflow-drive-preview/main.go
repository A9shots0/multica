// Command workflow-drive-preview reports what the workflow driver would do
// against a live database without changing anything.
//
// It exists so routing rules can be reviewed against real board state before
// any agent is woken: it constructs the driver in DryRun mode, so no plan is
// installed, no task is enqueued, and no Inbox item is written.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := db.New(pool)
	driver := &service.WorkflowDriver{
		Pool:        pool,
		Queries:     queries,
		QuietPeriod: time.Second,
		// TaskService stays nil: DryRun never reaches a dispatch call, and a
		// nil here guarantees that stays true rather than relying on the flag.
		DryRun: true,
	}
	report, err := driver.DriveOnce(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drive:", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))
}
