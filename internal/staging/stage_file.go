package staging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const addedFileGuidance = "=== REVIEW GUIDANCE ===\n" +
	"This file was ADDED in this PR (no prior version exists in the repository).\n" +
	"First assess: does this look like a bulk or automated import (generated code,\n" +
	"synced documentation, vendored content, migration output)?\n" +
	"  → If yes: provide a brief high-level overview of what it adds. Skip line-by-line review.\n" +
	"  → If no:  perform a full detailed review as normal.\n" +
	"=== END REVIEW GUIDANCE ===\n\n"

// Task is one reviewable staging task.
type Task map[string]any

// CollectSAFindings embeds matching .sa/*.txt lines for a file.
func CollectSAFindings(file, saDir string) string {
	info, err := os.Stat(saDir)
	if err != nil || !info.IsDir() {
		return ""
	}
	matchCandidates := []string{file, "/workspace/" + file}
	entries, err := os.ReadDir(saDir)
	if err != nil {
		return ""
	}
	names := make([]string, 0)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	findings := make([]string, 0)
	for _, name := range names {
		tool := strings.TrimSuffix(name, ".txt")
		data, err := os.ReadFile(filepath.Join(saDir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			for _, candidate := range matchCandidates {
				if strings.Contains(line, candidate) {
					findings = append(findings, tool+": "+line)
					break
				}
			}
		}
	}
	if len(findings) == 0 {
		return ""
	}
	return "\n=== STATIC ANALYSIS ===\n" + strings.Join(findings, "\n") + "\n=== END STATIC ANALYSIS ===\n"
}

// DetectSADir returns .sa if present under workDir.
func DetectSADir(workDir string) string {
	sa := ".sa"
	if workDir != "" {
		sa = filepath.Join(workDir, ".sa")
	}
	info, err := os.Stat(sa)
	if err != nil || !info.IsDir() {
		logf("INFO", "Static analysis: .sa/ not present — skipping SA embedding")
		return ""
	}
	entries, _ := os.ReadDir(sa)
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			n++
		}
	}
	logf("INFO", "Static analysis: %d tool output(s) found in .sa/", n)
	return sa
}

// StageFile writes staging input(s) for one file and returns task dicts.
func StageFile(
	g *GitRunner,
	file, refspec, stagingDir, repoRoot, agent, saDir, status string,
) ([]Task, error) {
	fullPath := filepath.Join(repoRoot, filepath.FromSlash(file))
	if _, err := os.Stat(fullPath); err != nil {
		logf("WARN", "Skipping deleted/missing file: %s", file)
		return nil, nil
	}
	slug := FileSlug(file)
	fileDiff, err := g.diff(refspec, file)
	if err != nil {
		return nil, err
	}
	diffLines := strings.Count(fileDiff, "\n")
	fileContent := g.showHEAD(file)
	contentLines := strings.Count(fileContent, "\n")
	combinedLines := contentLines + diffLines

	saSection := ""
	if saDir != "" {
		saSection = CollectSAFindings(file, saDir)
	}
	guidance := ""
	if status == "A" {
		guidance = addedFileGuidance
	}

	tasks := make([]Task, 0, 1)
	if combinedLines <= MaxCombinedLines {
		logf("INFO", "  %s: full_and_diff (%d lines)", file, combinedLines)
		inputText := guidance + "=== CURRENT FILE (" + file + ") ===\n" + fileContent + "\n=== DIFF ===\n" + fileDiff + saSection
		inputFile := BuildStagingFilename(slug, "")
		if err := os.WriteFile(filepath.Join(stagingDir, inputFile), []byte(inputText), 0o644); err != nil {
			return nil, err
		}
		tasks = append(tasks, Task{
			"file": file, "slug": slug, "mode": ModeFullAndDiff,
			"chunk": nil, "total_chunks": nil, "input_file": inputFile,
			"agent": agent, "status": status,
		})
	} else if diffLines <= MaxDiffLines {
		logf("INFO", "  %s: diff_only (file %d lines, diff %d lines)", file, contentLines, diffLines)
		inputFile := BuildStagingFilename(slug, "")
		if err := os.WriteFile(filepath.Join(stagingDir, inputFile), []byte(guidance+fileDiff+saSection), 0o644); err != nil {
			return nil, err
		}
		tasks = append(tasks, Task{
			"file": file, "slug": slug, "mode": ModeDiffOnly,
			"chunk": nil, "total_chunks": nil, "input_file": inputFile,
			"agent": agent, "status": status,
		})
	} else {
		chunks := ChunkLines(fileDiff, MaxDiffLines)
		total := len(chunks)
		logf("INFO", "  %s: diff_chunk — %d chunks (file %d lines, diff %d lines)", file, total, contentLines, diffLines)
		for idx, chunkText := range chunks {
			i := idx + 1
			inputFile := BuildStagingFilename(slug, fmt.Sprintf("-chunk%03d", i))
			chunkSA := ""
			if i == total {
				chunkSA = saSection
			}
			chunkGuidance := ""
			if i == 1 {
				chunkGuidance = guidance
			}
			if err := os.WriteFile(filepath.Join(stagingDir, inputFile), []byte(chunkGuidance+chunkText+chunkSA), 0o644); err != nil {
				return nil, err
			}
			tasks = append(tasks, Task{
				"file": file, "slug": slug, "mode": ModeDiffChunk,
				"chunk": i, "total_chunks": total, "input_file": inputFile,
				"agent": agent, "status": status,
			})
		}
	}
	return tasks, nil
}
