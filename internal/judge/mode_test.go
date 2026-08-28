package judge_test

import (
	"errors"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/judge"
)

func TestResolveModeDefault(t *testing.T) {
	t.Setenv(judge.EnvJudge, "")
	mode, err := judge.ResolveMode()
	if err != nil || mode != judge.ModeOpencode {
		t.Fatalf("mode=%q err=%v", mode, err)
	}
}

func TestResolveModeStrop(t *testing.T) {
	t.Setenv(judge.EnvJudge, "strop")
	mode, err := judge.ResolveMode()
	if err != nil || mode != judge.ModeStrop {
		t.Fatalf("mode=%q err=%v", mode, err)
	}
}

func TestResolveModeInvalid(t *testing.T) {
	t.Setenv(judge.EnvJudge, "both")
	_, err := judge.ResolveMode()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureOpencodeDriver(t *testing.T) {
	t.Setenv(judge.EnvJudge, "")
	if err := judge.EnsureOpencodeDriver(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(judge.EnvJudge, "strop")
	err := judge.EnsureOpencodeDriver()
	if !errors.Is(err, judge.ErrStropJudgeNotReady) {
		t.Fatalf("got %v", err)
	}
}
