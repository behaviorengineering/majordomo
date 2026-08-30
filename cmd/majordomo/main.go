// Command majordomo is the control-plane CLI for repository operations.
// See docs/PLAN-control-tower-github-go.md for the target architecture.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
	"github.com/behaviorengineering/majordomo/internal/cli"
	"github.com/behaviorengineering/majordomo/internal/observability"
	"github.com/behaviorengineering/majordomo/internal/staging"
)

func main() {
	defer func() {
		aigateway.ShutdownGlobal()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observability.Flush(ctx)
		_ = observability.Shutdown(ctx)
	}()

	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, staging.ErrNothingToReview) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
