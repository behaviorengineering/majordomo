// Command majordomo is the control-plane CLI for repository operations.
// See docs/PLAN-control-tower-github-go.md for the target architecture.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/behaviorengineering/majordomo/internal/ops/cli"
	"github.com/behaviorengineering/majordomo/pkg/platform/aigateway"
	"github.com/behaviorengineering/majordomo/pkg/platform/observability"
)

func main() {
	os.Exit(run())
}

func run() int {
	defer func() {
		aigateway.ShutdownGlobal()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observability.Flush(ctx)
		_ = observability.Shutdown(ctx)
	}()

	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	root := cli.NewRoot()
	if err := root.ExecuteContext(runCtx); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			return 1
		}
		if usageErr := root.Usage(); usageErr != nil {
			if _, writeErr := fmt.Fprintln(os.Stderr, usageErr); writeErr != nil {
				return 1
			}
		}
		if cli.IsNothingToReview(err) {
			return 2
		}
		return 1
	}
	return 0
}
