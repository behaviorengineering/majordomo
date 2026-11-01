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
	defer func() {
		aigateway.ShutdownGlobal()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observability.Flush(ctx)
		_ = observability.Shutdown(ctx)
	}()

	root := cli.NewRoot()
	if err := root.Execute(); err != nil {
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
