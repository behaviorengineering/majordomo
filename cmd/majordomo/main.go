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
	root := cli.NewRoot()
	runCtx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
	defer cancel()
	root.SetContext(runCtx)

	defer func() {
		aigateway.ShutdownGlobal()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observability.Flush(ctx)
		_ = observability.Shutdown(ctx)
	}()

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, staging.ErrNothingToReview) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
