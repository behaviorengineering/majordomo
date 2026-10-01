// Command majordomo is the control-plane CLI for repository operations.
// See docs/PLAN-control-tower-github-go.md for the target architecture.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/behaviorengineering/majordomo/internal/ops/cli"
	"github.com/behaviorengineering/majordomo/pkg/platform/aigateway"
	"github.com/behaviorengineering/majordomo/pkg/platform/observability"
	"github.com/behaviorengineering/majordomo/pkg/review/staging"
)

func main() {
	runCtx, runCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer runCancel()
	root := cli.NewRoot()
	root.SetContext(runCtx)

	defer func() {
		aigateway.ShutdownGlobal()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := observability.Flush(shutdownCtx); err != nil {
			if _, writeErr := fmt.Fprintf(os.Stderr, "observability flush: %v\n", err); writeErr != nil {
				return
			}
		}
		if err := observability.Shutdown(shutdownCtx); err != nil {
			if _, writeErr := fmt.Fprintf(os.Stderr, "observability shutdown: %v\n", err); writeErr != nil {
				return
			}
		}
	}()

	if err := root.ExecuteContext(runCtx); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(1)
		}
		if _, writeErr := fmt.Fprintln(os.Stderr, root.UsageString()); writeErr != nil {
			os.Exit(1)
		}
		if errors.Is(err, staging.ErrNothingToReview) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
