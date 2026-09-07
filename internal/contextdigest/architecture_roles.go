package contextdigest

import (
	"strings"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

const (
	storyArchitectureRoleMarker    = "Teaching story."
	typologyArchitectureRoleMarker = "Typology seed evidence."
)

// storyArchitectureBanner labels root architecture.md as the living teaching story.
const storyArchitectureBanner = "> **Teaching story.** Living project architecture for humans and review grounding. " +
	"Not Typology seed evidence (see `evidence/typology/" + contextstore.TypologyArchitectureBriefPath + "`).\n"

// typologyArchitectureBanner labels the evidence brief as Typology proposal output.
const typologyArchitectureBanner = "> **Typology seed evidence.** Post-survey/refine architecture brief for this context digest proposal. " +
	"Not the teaching-story root `architecture.md`, and not the confirmed `.typology/` catalog.\n"

func ensureStoryArchitectureBanner(md string) string {
	return ensureRoleBanner(md, storyArchitectureRoleMarker, storyArchitectureBanner)
}

func ensureTypologyArchitectureBanner(md string) string {
	return ensureRoleBanner(md, typologyArchitectureRoleMarker, typologyArchitectureBanner)
}

// ensureRoleBanner inserts a blockquote role callout after the first H1 when missing.
func ensureRoleBanner(md, marker, banner string) string {
	body := strings.TrimSpace(md)
	if body == "" {
		return strings.TrimSpace(banner) + "\n"
	}
	if strings.Contains(body, marker) {
		return body + "\n"
	}
	lines := strings.Split(body, "\n")
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "#") {
		var b strings.Builder
		b.WriteString(strings.TrimRight(lines[0], "\r"))
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(banner))
		b.WriteString("\n")
		if len(lines) > 1 {
			rest := strings.TrimLeft(strings.Join(lines[1:], "\n"), "\n")
			if rest != "" {
				b.WriteString("\n")
				b.WriteString(rest)
				if !strings.HasSuffix(rest, "\n") {
					b.WriteString("\n")
				}
			}
		}
		return b.String()
	}
	return strings.TrimSpace(banner) + "\n\n" + body + "\n"
}
