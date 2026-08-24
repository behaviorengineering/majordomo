package staging

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/cluster"
)

// StageCrossSkillBatches stages summary / technical / blast-radius batch_000.
func StageCrossSkillBatches(
	g *GitRunner,
	allFiles []string,
	saDir, stagingDir, repoRoot, baseBranch, refspec string,
	fileStatus map[string]string,
	statusPairs [][2]string,
	agentContext AgentContext,
	extraExcl []*regexp.Regexp,
	summaryConfig map[string]any,
) ([]BatchEntry, []string, error) {
	summarySkill := "pr-review-summary"
	technicalSkill := "pr-review-technical"
	blastRadiusSkill := "pr-review-blast-radius"

	summaryStaging := filepath.Join(stagingDir, summarySkill, CrossSkillBatchDir)
	if err := os.MkdirAll(summaryStaging, 0o755); err != nil {
		return nil, nil, err
	}

	summaryTasks := []Task{}
	for _, file := range allFiles {
		if IsExcludedWithExtra(file, extraExcl) {
			continue
		}
		full := filepath.Join(repoRoot, filepath.FromSlash(file))
		if _, err := os.Stat(full); err != nil {
			continue
		}
		slug := FileSlug(file)
		fileDiff, err := g.diff(refspec, file)
		if err != nil {
			logf("WARN", "  summary: git error for %s: %v", file, err)
			continue
		}
		if strings.TrimSpace(fileDiff) == "" {
			continue
		}
		inputFile := BuildStagingFilename(slug, "")
		if err := os.WriteFile(filepath.Join(summaryStaging, inputFile), []byte(fileDiff), 0o644); err != nil {
			return nil, nil, err
		}
		ctx, err := ContextForFile(file, agentContext, repoRoot)
		if err != nil {
			return nil, nil, err
		}
		status := fileStatus[file]
		if status == "" {
			status = "M"
		}
		summaryTasks = append(summaryTasks, Task{
			"file": file, "slug": slug, "mode": ModeDiffOnly,
			"chunk": nil, "total_chunks": nil, "input_file": inputFile,
			"agent": summarySkill, "status": status, "agent_context": ctx,
		})
	}

	summaryFiles := make([]string, 0, len(summaryTasks))
	for _, t := range summaryTasks {
		summaryFiles = append(summaryFiles, t["file"].(string))
	}
	summaryClusters := cluster.ClusterFiles(summaryFiles, repoRoot)
	summaryDepClusters := [][]string{}
	for _, c := range summaryClusters {
		if len(c) > 1 {
			summaryDepClusters = append(summaryDepClusters, c)
		}
	}
	logf("INFO", "Summary dep clusters: %d multi-file cluster(s)", len(summaryDepClusters))

	summaryReverseDeps := cluster.ReverseDeps(summaryFiles, repoRoot)
	logf("INFO", "Summary reverse deps: %d changed file(s) have external importers", len(summaryReverseDeps))

	summarySA := map[string]string{}
	if saDir != "" {
		entries, _ := os.ReadDir(saDir)
		names := []string{}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
				names = append(names, e.Name())
			}
		}
		sortStrings(names)
		for _, name := range names {
			data, err := os.ReadFile(filepath.Join(saDir, name))
			if err != nil {
				continue
			}
			summarySA[strings.TrimSuffix(name, ".txt")] = string(data)
		}
		if len(summarySA) > 0 {
			logf("INFO", "Summary SA: %d tool output(s) embedded in manifest", len(summarySA))
		}
	}

	statusBreakdown := map[string]int{}
	for _, p := range statusPairs {
		statusBreakdown[p[0]]++
	}
	sc := summaryConfig
	if sc == nil {
		sc = map[string]any{}
	}
	summaryManifest := map[string]any{
		"base_branch":      baseBranch,
		"refspec":          refspec,
		"skill_dir":        summarySkill,
		"review_agents":    map[string][]string{summarySkill: summaryFiles},
		"reviewable":       summaryTasks,
		"excluded":         []string{},
		"dep_clusters":     summaryDepClusters,
		"reverse_deps":     summaryReverseDeps,
		"static_analysis":  summarySA,
		"status_breakdown": statusBreakdown,
		"summary_config":   sc,
	}
	if err := writeJSON(filepath.Join(summaryStaging, "manifest.json"), summaryManifest); err != nil {
		return nil, nil, err
	}
	logf("INFO", "Summary batch: %d file(s) staged in %s", len(summaryTasks), summaryStaging)

	blastEntries := []BatchEntry{}
	if len(summaryReverseDeps) > 0 {
		blastStaging := filepath.Join(stagingDir, blastRadiusSkill, CrossSkillBatchDir)
		if err := os.MkdirAll(blastStaging, 0o755); err != nil {
			return nil, nil, err
		}
		for _, task := range summaryTasks {
			inputFile := task["input_file"].(string)
			src := filepath.Join(summaryStaging, inputFile)
			dst := filepath.Join(blastStaging, inputFile)
			if _, err := os.Stat(dst); err == nil {
				continue
			}
			_ = copyFile(src, dst)
		}
		blastManifest := map[string]any{
			"base_branch":     baseBranch,
			"refspec":         refspec,
			"skill_dir":       blastRadiusSkill,
			"review_agents":   map[string][]string{blastRadiusSkill: summaryFiles},
			"reviewable":      summaryTasks,
			"excluded":        []string{},
			"dep_clusters":    summaryDepClusters,
			"reverse_deps":    summaryReverseDeps,
			"static_analysis": summarySA,
		}
		if err := writeJSON(filepath.Join(blastStaging, "manifest.json"), blastManifest); err != nil {
			return nil, nil, err
		}
		logf("INFO", "Blast radius batch: %d file(s) staged in %s", len(summaryTasks), blastStaging)
		blastEntries = append(blastEntries, BatchEntry{
			Skill: blastRadiusSkill, BatchNum: CrossSkillBatchNum,
			TaskCount: len(summaryTasks), StagingDir: blastStaging,
		})
	} else {
		logf("INFO", "Blast radius batch: skipped — no reverse dependencies found")
	}

	technicalStaging := filepath.Join(stagingDir, technicalSkill, CrossSkillBatchDir)
	if err := os.MkdirAll(technicalStaging, 0o755); err != nil {
		return nil, nil, err
	}
	for _, task := range summaryTasks {
		inputFile := task["input_file"].(string)
		src := filepath.Join(summaryStaging, inputFile)
		dst := filepath.Join(technicalStaging, inputFile)
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		_ = copyFile(src, dst)
	}
	technicalManifest := map[string]any{
		"base_branch":   baseBranch,
		"refspec":       refspec,
		"skill_dir":     technicalSkill,
		"review_agents": map[string][]string{technicalSkill: summaryFiles},
		"reviewable":    summaryTasks,
		"excluded":      []string{},
		"dep_clusters":  summaryDepClusters,
		"reverse_deps":  summaryReverseDeps,
	}
	if err := writeJSON(filepath.Join(technicalStaging, "manifest.json"), technicalManifest); err != nil {
		return nil, nil, err
	}
	logf("INFO", "Technical review batch: %d file(s) staged in %s", len(summaryTasks), technicalStaging)

	extraEntries := []BatchEntry{}
	extraSkills := []string{}
	if len(blastEntries) > 0 {
		extraEntries = append(extraEntries, blastEntries...)
		extraSkills = append(extraSkills, blastRadiusSkill)
	}
	technicalEntry := BatchEntry{
		Skill: technicalSkill, BatchNum: CrossSkillBatchNum,
		TaskCount: len(summaryTasks), StagingDir: technicalStaging,
	}
	extraEntries = append([]BatchEntry{technicalEntry}, extraEntries...)
	extraSkills = append([]string{technicalSkill}, extraSkills...)

	summaryEntry := BatchEntry{
		Skill: summarySkill, BatchNum: CrossSkillBatchNum,
		TaskCount: len(summaryTasks), StagingDir: summaryStaging,
	}
	extraEntries = append([]BatchEntry{summaryEntry}, extraEntries...)
	extraSkills = append([]string{summarySkill}, extraSkills...)

	return extraEntries, extraSkills, nil
}

// WriteBatchPlan writes batch-plan.json.
func WriteBatchPlan(entries []BatchEntry, skills []string, stagingDir string) error {
	plan := map[string]any{
		"batches":       entries,
		"skills":        skills,
		"total_batches": len(entries),
	}
	path := filepath.Join(stagingDir, "batch-plan.json")
	if err := writeJSON(path, plan); err != nil {
		return err
	}
	logf("INFO", "Batch plan: %d batch(es) → %s", len(entries), path)
	return nil
}
