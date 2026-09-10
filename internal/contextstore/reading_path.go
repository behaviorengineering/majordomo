package contextstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	readingNavStart = "<!-- majordomo-reading-nav:start -->"
	readingNavEnd   = "<!-- majordomo-reading-nav:end -->"
	readingTOCStart = "<!-- majordomo-reading-toc:start -->"
	readingTOCEnd   = "<!-- majordomo-reading-toc:end -->"

	TypologyReadingIndexPath = "README.md"
)

// StoryReadingOrder is the guided teaching-story sequence on the context branch root.
var StoryReadingOrder = []string{
	"README.md",
	"mission.md",
	"architecture.md",
	"conventions.md",
	"weaknesses.md",
	"chronology.md",
}

// TypologyReadingOrder is the guided briefing sequence under evidence/typology/.
// pr_priority.md is optional and is skipped when absent.
var TypologyReadingOrder = []string{
	TypologyReadingIndexPath,
	TypologyArchitectureBriefPath,
	"cluster_proposal.md",
	"journey.md",
	"human_intervention.md",
	"pr_priority.md",
}

// TypologyAppendixFiles are machine/reference artifacts listed in the typology TOC only.
var TypologyAppendixFiles = []string{
	"package_roles.yaml",
	"package_contracts.md",
	"graph.txt",
	"refined_snapshot.yaml",
	"snapshot.yaml",
	"manifest.yaml",
}

// EnsureReadingNav inserts or replaces the reading-path banner near the top of markdown.
func EnsureReadingNav(md string, prevLabel, prevHref, nextLabel, nextHref, tocHref string) string {
	body := stripReadingNav(md)
	banner := formatReadingNav(prevLabel, prevHref, nextLabel, nextHref, tocHref)
	return insertAfterFirstHeading(body, banner)
}

// EnsureRootReadingTOC inserts or replaces the root Reading order section.
func EnsureRootReadingTOC(md string, typologyPresent bool) string {
	body := stripMarkedSection(md, readingTOCStart, readingTOCEnd)
	section := formatRootReadingTOC(typologyPresent)
	return insertAfterFirstHeading(body, section)
}

// ApplyReadingPath writes typology/README.md when evidence exists and applies
// Prev/Next banners plus the root TOC to guided markdown files.
func ApplyReadingPath(ctxDir string) error {
	if strings.TrimSpace(ctxDir) == "" {
		return fmt.Errorf("reading path: ctx dir is required")
	}
	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	typologyPresent := dirExists(evidenceDir)
	if typologyPresent {
		if err := writeTypologyReadingIndex(evidenceDir); err != nil {
			return err
		}
	}

	readmePath := filepath.Join(ctxDir, "README.md")
	if fileExists(readmePath) {
		data, err := os.ReadFile(readmePath)
		if err != nil {
			return fmt.Errorf("reading path read README: %w", err)
		}
		updated := EnsureRootReadingTOC(string(data), typologyPresent)
		if err := os.WriteFile(readmePath, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("reading path write README toc: %w", err)
		}
	}

	storyChain := existingRelPaths(ctxDir, StoryReadingOrder)
	if typologyPresent {
		storyChain = append(storyChain, filepath.ToSlash(filepath.Join("evidence", "typology", TypologyReadingIndexPath)))
	}
	if err := applyNavChain(ctxDir, storyChain, "README.md", false); err != nil {
		return err
	}

	if typologyPresent {
		typoChain := existingRelPaths(evidenceDir, TypologyReadingOrder)
		// Handoff back to the story TOC after the last typology briefing file.
		absChain := make([]string, 0, len(typoChain)+1)
		for _, rel := range typoChain {
			absChain = append(absChain, filepath.ToSlash(filepath.Join("evidence", "typology", rel)))
		}
		absChain = append(absChain, "README.md")
		if err := applyNavChain(ctxDir, absChain, filepath.ToSlash(filepath.Join("evidence", "typology", TypologyReadingIndexPath)), true); err != nil {
			return err
		}
	}

	return verifyReadingNav(ctxDir, typologyPresent)
}

func writeTypologyReadingIndex(evidenceDir string) error {
	var b strings.Builder
	b.WriteString("# Typology seed evidence\n\n")
	b.WriteString("Briefing for this digest proposal. Not the teaching-story root files and not a confirmed `.typology/` catalog.\n\n")
	b.WriteString("## Reading order\n\n")
	b.WriteString("1. This index (`README.md`)\n")
	b.WriteString("2. [`architecture_brief.md`](architecture_brief.md) — observed map / architecture brief\n")
	b.WriteString("3. [`cluster_proposal.md`](cluster_proposal.md) — optional grouping overlay\n")
	b.WriteString("4. [`journey.md`](journey.md) — refine decisions and debt\n")
	b.WriteString("5. [`human_intervention.md`](human_intervention.md) — operator priorities\n")
	b.WriteString("6. [`pr_priority.md`](pr_priority.md) — when present; cold-reader PR counsel\n")
	b.WriteString("7. Back to the [story TOC](../../README.md)\n\n")
	b.WriteString("## Appendix (reference, no Prev/Next)\n\n")
	for _, name := range TypologyAppendixFiles {
		if fileExists(filepath.Join(evidenceDir, name)) {
			fmt.Fprintf(&b, "- [`%s`](%s)\n", name, name)
		}
	}
	b.WriteString("\n")
	path := filepath.Join(evidenceDir, TypologyReadingIndexPath)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func formatRootReadingTOC(typologyPresent bool) string {
	var b strings.Builder
	b.WriteString(readingTOCStart)
	b.WriteString("\n## Reading order\n\n")
	b.WriteString("1. [README.md](README.md) (this file)\n")
	b.WriteString("2. [mission.md](mission.md)\n")
	b.WriteString("3. [architecture.md](architecture.md)\n")
	b.WriteString("4. [conventions.md](conventions.md)\n")
	b.WriteString("5. [weaknesses.md](weaknesses.md)\n")
	b.WriteString("6. [chronology.md](chronology.md)\n")
	if typologyPresent {
		b.WriteString("7. [evidence/typology/README.md](evidence/typology/README.md) — Typology seed briefing\n")
	} else {
		b.WriteString("7. `evidence/typology/` — appears after a Typology survey seed\n")
	}
	b.WriteString("\n")
	b.WriteString(readingTOCEnd)
	b.WriteString("\n")
	return b.String()
}

func formatReadingNav(prevLabel, prevHref, nextLabel, nextHref, tocHref string) string {
	var parts []string
	if strings.TrimSpace(prevHref) != "" {
		label := strings.TrimSpace(prevLabel)
		if label == "" {
			label = prevHref
		}
		parts = append(parts, fmt.Sprintf("[Prev: %s](%s)", label, prevHref))
	}
	if strings.TrimSpace(nextHref) != "" {
		label := strings.TrimSpace(nextLabel)
		if label == "" {
			label = nextHref
		}
		parts = append(parts, fmt.Sprintf("[Next: %s](%s)", label, nextHref))
	}
	toc := strings.TrimSpace(tocHref)
	if toc == "" {
		toc = "README.md"
	}
	parts = append(parts, fmt.Sprintf("[TOC](%s)", toc))
	var b strings.Builder
	b.WriteString(readingNavStart)
	b.WriteString("\n**Reading path:** ")
	b.WriteString(strings.Join(parts, " · "))
	b.WriteString("\n")
	b.WriteString(readingNavEnd)
	b.WriteString("\n")
	return b.String()
}

func applyNavChain(ctxDir string, relChain []string, tocRel string, typologyLinks bool) error {
	for i, rel := range relChain {
		// Terminal handoff to root README is only a Next target for typology files;
		// do not rewrite root README a second time in the typology pass.
		if typologyLinks && rel == "README.md" {
			continue
		}
		path := filepath.Join(ctxDir, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading path read %s: %w", rel, err)
		}
		var prevLabel, prevHref, nextLabel, nextHref string
		if i > 0 {
			prevHref = relativeMarkdownLink(rel, relChain[i-1])
			prevLabel = filepath.Base(relChain[i-1])
		}
		if i+1 < len(relChain) {
			nextHref = relativeMarkdownLink(rel, relChain[i+1])
			nextLabel = filepath.Base(relChain[i+1])
		}
		tocHref := relativeMarkdownLink(rel, tocRel)
		updated := EnsureReadingNav(string(data), prevLabel, prevHref, nextLabel, nextHref, tocHref)
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("reading path write %s: %w", rel, err)
		}
	}
	return nil
}

func verifyReadingNav(ctxDir string, typologyPresent bool) error {
	check := func(rel string) error {
		path := filepath.Join(ctxDir, filepath.FromSlash(rel))
		if !fileExists(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(data), readingNavStart) {
			return fmt.Errorf("reading path: guided file %s missing nav banner", rel)
		}
		return nil
	}
	for _, rel := range StoryReadingOrder {
		if rel == "README.md" {
			// Root README carries the TOC section; nav is still applied.
		}
		if err := check(rel); err != nil {
			return err
		}
	}
	if !typologyPresent {
		return nil
	}
	for _, name := range TypologyReadingOrder {
		if name == "pr_priority.md" && !fileExists(filepath.Join(ctxDir, "evidence", "typology", name)) {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("evidence", "typology", name))
		if !fileExists(filepath.Join(ctxDir, filepath.FromSlash(rel))) {
			continue
		}
		if err := check(rel); err != nil {
			return err
		}
	}
	return nil
}

func relativeMarkdownLink(fromRel, toRel string) string {
	fromDir := filepath.ToSlash(filepath.Dir(fromRel))
	if fromDir == "." {
		fromDir = ""
	}
	to := filepath.ToSlash(toRel)
	if fromDir == "" {
		return to
	}
	rel, err := filepath.Rel(fromDir, to)
	if err != nil {
		return to
	}
	return filepath.ToSlash(rel)
}

func existingRelPaths(base string, names []string) []string {
	var out []string
	for _, name := range names {
		if fileExists(filepath.Join(base, name)) {
			out = append(out, name)
		}
	}
	return out
}

func stripReadingNav(md string) string {
	return stripMarkedSection(md, readingNavStart, readingNavEnd)
}

func stripMarkedSection(md, start, end string) string {
	for {
		s := strings.Index(md, start)
		if s < 0 {
			return md
		}
		e := strings.Index(md[s:], end)
		if e < 0 {
			return strings.TrimSpace(md[:s]) + "\n"
		}
		e = s + e + len(end)
		for e < len(md) && (md[e] == '\n' || md[e] == '\r') {
			e++
		}
		md = md[:s] + md[e:]
	}
}

func insertAfterFirstHeading(md, block string) string {
	body := strings.TrimSpace(md)
	block = strings.TrimSpace(block) + "\n"
	if body == "" {
		return block
	}
	lines := strings.Split(body, "\n")
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "#") {
		var b strings.Builder
		b.WriteString(strings.TrimRight(lines[0], "\r"))
		b.WriteString("\n\n")
		b.WriteString(block)
		if !strings.HasSuffix(block, "\n") {
			b.WriteString("\n")
		}
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
	return block + "\n" + body + "\n"
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
