// Command majordomo is the control-plane CLI for repository operations.
// See docs/PLAN-control-tower-github-go.md for the target architecture.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/behaviorengineering/majordomo/internal/ops/cli"
	"github.com/behaviorengineering/majordomo/pkg/platform/aigateway"
	"github.com/behaviorengineering/majordomo/pkg/platform/observability"
	"github.com/behaviorengineering/majordomo/pkg/review/staging"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	defer func() {
		aigateway.ShutdownGlobal()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := observability.Flush(shutdownCtx); err != nil {
			log.Printf("observability flush: %v", err)
		}
		if err := observability.Shutdown(shutdownCtx); err != nil {
			log.Printf("observability shutdown: %v", err)
		}
	}()

	if err := cli.NewRoot().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, staging.ErrNothingToReview) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
