package judge

import (
	"fmt"
	"os"
	"strings"
)

// Mode selects which Judge driver runs a job. Never both in one job.
type Mode string

const (
	ModeOpencode Mode = "opencode"
	ModeStrop    Mode = "strop"
)

// EnvJudge is the cutover environment variable.
const EnvJudge = "MAJORDOMO_JUDGE"

// ErrStropJudgeNotReady is returned when ModeStrop is selected but generator
// modules are not registered yet. Default remains ModeOpencode.
var ErrStropJudgeNotReady = fmt.Errorf("judge: MAJORDOMO_JUDGE=strop is not ready (no registered generator modules); use opencode or unset %s", EnvJudge)

// ResolveMode reads MAJORDOMO_JUDGE. Empty or unset → opencode.
func ResolveMode() (Mode, error) {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(EnvJudge)))
	switch raw {
	case "", string(ModeOpencode):
		return ModeOpencode, nil
	case string(ModeStrop):
		return ModeStrop, nil
	default:
		return "", fmt.Errorf("judge: invalid %s=%q (want opencode|strop)", EnvJudge, os.Getenv(EnvJudge))
	}
}

// EnsureOpencodeDriver fails if the process requested strop (or an invalid mode).
// Call at the start of the OpenCode protocol path so the two drivers never mix.
func EnsureOpencodeDriver() error {
	mode, err := ResolveMode()
	if err != nil {
		return err
	}
	if mode == ModeStrop {
		return ErrStropJudgeNotReady
	}
	return nil
}
