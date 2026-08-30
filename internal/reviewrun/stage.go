package reviewrun

import (
	"fmt"
	"strings"
)

// Job stop stages for majordomo run review --until.
const (
	StageClone    = "clone"
	StageSA       = "sa"
	StagePrep     = "prep"
	StageWaves    = "waves"
	StageFinalize = "finalize"
	StageProse    = "prose"
	StageSynth    = "synth"
	StageReport   = "report"
	StagePublish  = "publish"
)

var jobRank = map[string]int{
	StageClone:    1,
	StageSA:       2,
	StagePrep:     3,
	StageWaves:    4,
	StageFinalize: 5,
	StageProse:    6,
	StageSynth:    7,
	StageReport:   8,
	StagePublish:  9,
}

// ParseUntil validates --until for the review job.
func ParseUntil(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return "", nil
	case StageClone, StageSA, StagePrep, StageWaves, StageFinalize, StageProse, StageSynth, StageReport, StagePublish:
		return s, nil
	default:
		return "", fmt.Errorf("unknown until stage %q (clone|sa|prep|waves|finalize|prose|synth|report|publish)", s)
	}
}

func shouldRun(until, stage string) bool {
	if until == "" {
		return true
	}
	ur, okU := jobRank[until]
	sr, okS := jobRank[stage]
	if !okU || !okS {
		return true
	}
	return sr <= ur
}

func orchestrateUntil(until string) string {
	switch until {
	case "", StagePublish:
		return ""
	case StageClone, StageSA:
		return ""
	default:
		return until
	}
}
