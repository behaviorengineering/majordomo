package contextgate

import (
	"strings"
)

// Action is a parsed @majordomo gate comment.
type Action int

const (
	ActionIgnore Action = iota
	ActionReject
	ActionDone
	ActionWhy
)

// Comment is one forge PR comment relevant to gate parsing.
type Comment struct {
	Body     string
	Author   string
	PostedAt string
}

// ParsedComment is the result of parsing one comment body.
type ParsedComment struct {
	Action  Action
	Payload string // reason for reject/why; empty for done
}

// DefaultPrefix is the gate comment prefix.
const DefaultPrefix = "@majordomo"

// ParseComment interprets a PR comment body. Only lines starting with prefix count.
func ParseComment(body, prefix string) ParsedComment {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	line := strings.TrimSpace(body)
	if !strings.HasPrefix(line, prefix) {
		return ParsedComment{Action: ActionIgnore}
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	switch {
	case strings.HasPrefix(rest, "reject "):
		return ParsedComment{Action: ActionReject, Payload: strings.TrimSpace(strings.TrimPrefix(rest, "reject "))}
	case rest == "done" || strings.HasPrefix(rest, "done "):
		return ParsedComment{Action: ActionDone}
	case strings.HasPrefix(rest, "why "):
		return ParsedComment{Action: ActionWhy, Payload: strings.TrimSpace(strings.TrimPrefix(rest, "why "))}
	default:
		return ParsedComment{Action: ActionIgnore}
	}
}

// ApplyComments folds chronological comments into the latest gate-relevant action per kind.
func ApplyComments(comments []Comment, prefix string) (rejectReason string, done bool, why string) {
	for _, c := range comments {
		p := ParseComment(c.Body, prefix)
		switch p.Action {
		case ActionReject:
			if strings.TrimSpace(p.Payload) != "" {
				rejectReason = p.Payload
				done = false
			}
		case ActionDone:
			done = true
		case ActionWhy:
			if strings.TrimSpace(p.Payload) != "" {
				why = p.Payload
			}
		}
	}
	return rejectReason, done, why
}
