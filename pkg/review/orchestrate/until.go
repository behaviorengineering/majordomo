package orchestrate

import (
	"fmt"
	"strings"
)

// Orchestrate stop stages (subset of majordomo run review --until).
const (
	StagePrep     = "prep"
	StageWaves    = "waves"
	StageFinalize = "finalize"
	StageProse    = "prose"
	StageSynth    = "synth"
	StageReport   = "report"
)

var orchestrateRank = map[string]int{
	StagePrep:     1,
	StageWaves:    2,
	StageFinalize: 3,
	StageProse:    4,
	StageSynth:    5,
	StageReport:   6,
}

// NormalizeUntil validates --until for orchestrate.
// Empty and "publish" mean run through report. clone/sa belong to run review.
func NormalizeUntil(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "publish":
		return "", nil
	case "clone", "sa":
		return "", fmt.Errorf("until %q is not an orchestrate stage (use majordomo run review)", s)
	case StagePrep, StageWaves, StageFinalize, StageProse, StageSynth, StageReport:
		return s, nil
	default:
		return "", fmt.Errorf("unknown until stage %q (prep|waves|finalize|prose|synth|report)", s)
	}
}

// shouldRun reports whether stage should execute given Until (empty = full run).
func shouldRun(until, stage string) bool {
	if until == "" {
		return true
	}
	ur, okU := orchestrateRank[until]
	sr, okS := orchestrateRank[stage]
	if !okU || !okS {
		return true
	}
	return sr <= ur
}
