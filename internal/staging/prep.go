package staging

import (
	"os"
	"path/filepath"
	"regexp"
)

// Run executes the full prep pipeline.
func Run(opts Options) error {
	if opts.BatchSize <= 0 {
		opts.BatchSize = batchSizeFromEnv()
	}
	workDir := opts.RepoRoot
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return fatalf("cannot get cwd: %v", err)
		}
	}

	if err := os.MkdirAll(opts.StagingDir, 0o755); err != nil {
		return fatalf("cannot create staging dir: %v", err)
	}

	routing, personaPaths, err := LoadRouting(opts.RoutingPath)
	if err != nil {
		return err
	}
	summaryConfig, err := LoadSummaryConfig(opts.SummaryConfigPath)
	if err != nil {
		return err
	}

	setup, err := SetupGit(opts.BaseBranch, opts.StagingDir, workDir)
	if err != nil {
		return err
	}

	agentContext, err := LoadAgentContextConfig(opts.AgentContextPath)
	if err != nil {
		return err
	}
	personas, err := ResolveRoutingPersonas(personaPaths, setup.RepoRoot)
	if err != nil {
		return err
	}

	reviewable, excluded := classifyFiles(setup.AllFiles, routing, setup.ExtraExcl)
	if len(reviewable) == 0 {
		logf("WARN", "All changed files excluded — nothing to review")
		return ErrNothingToReview
	}

	saDir := DetectSADir(workDir)
	g := &GitRunner{Dir: workDir}
	tasks, reviewAgents, excluded, err := stageReviewableFiles(
		g, reviewable, excluded, routing, setup.Refspec, opts.StagingDir,
		setup.RepoRoot, saDir, opts.BaseBranch, setup.FileStatus, agentContext, personas,
	)
	if err != nil {
		return err
	}

	batchEntries, codeSkillNames, err := StageSkillBatches(
		tasks, reviewAgents, excluded, opts.StagingDir, setup.RepoRoot,
		opts.BaseBranch, setup.Refspec, opts.BatchSize,
	)
	if err != nil {
		return err
	}

	crossEntries, crossSkills, err := StageCrossSkillBatches(
		g, setup.AllFiles, saDir, opts.StagingDir, setup.RepoRoot,
		opts.BaseBranch, setup.Refspec, setup.FileStatus, setup.StatusPairs,
		agentContext, setup.ExtraExcl, summaryConfig,
	)
	if err != nil {
		return err
	}
	for i := len(crossEntries) - 1; i >= 0; i-- {
		batchEntries = append([]BatchEntry{crossEntries[i]}, batchEntries...)
	}
	batchPlanSkills := append(append([]string{}, crossSkills...), codeSkillNames...)
	return WriteBatchPlan(batchEntries, batchPlanSkills, opts.StagingDir)
}

func classifyFiles(allFiles []string, routing []RoutingRule, extra []*regexp.Regexp) (reviewable, excluded []string) {
	patternExcluded := []string{}
	unrouted := []string{}
	for _, file := range allFiles {
		if IsExcludedWithExtra(file, extra) {
			patternExcluded = append(patternExcluded, file)
		} else if ClassifyFile(file, routing) != "" {
			reviewable = append(reviewable, file)
		} else {
			unrouted = append(unrouted, file)
		}
	}
	excluded = append(patternExcluded, unrouted...)
	logf("INFO", "Changed files: %d", len(allFiles))
	logf("INFO", "Reviewable:    %d", len(reviewable))
	logf("INFO", "Excluded:      %d (%d unrouted, %d pattern-excluded)",
		len(excluded), len(unrouted), len(patternExcluded))
	return reviewable, excluded
}

func stageReviewableFiles(
	g *GitRunner,
	reviewable, excluded []string,
	routing []RoutingRule,
	refspec, stagingDir, repoRoot, saDir, baseBranch string,
	fileStatus map[string]string,
	agentContext AgentContext,
	personas map[string]string,
) ([]Task, map[string][]string, []string, error) {
	excluded = append([]string{}, excluded...)
	tasks := []Task{}
	reviewAgents := map[string][]string{}
	skipped := []string{}
	logf("INFO", "Included:")
	for _, file := range reviewable {
		full := filepath.Join(repoRoot, filepath.FromSlash(file))
		if _, err := os.Stat(full); err != nil {
			skipped = append(skipped, file)
			continue
		}
		agent := ClassifyFile(file, routing)
		if agent == "" {
			excluded = append(excluded, file)
			continue
		}
		reviewAgents[agent] = append(reviewAgents[agent], file)
		fileContext, err := ContextForFile(file, agentContext, repoRoot)
		if err != nil {
			return nil, nil, nil, err
		}
		status := fileStatus[file]
		if status == "" {
			status = "M"
		}
		staged, err := StageFile(g, file, refspec, stagingDir, repoRoot, agent, saDir, status)
		if err != nil {
			if _, ok := err.(*GitError); ok {
				logf("WARN", "  git error staging %s: %v", file, err)
				continue
			}
			return nil, nil, nil, err
		}
		for _, t := range staged {
			t["agent_context"] = fileContext
			if p, ok := personas[agent]; ok {
				t["persona"] = p
			}
			tasks = append(tasks, t)
		}
	}
	if len(skipped) > 0 {
		logf("INFO", "Skipped (deleted):  %d", len(skipped))
		for _, f := range skipped {
			logf("WARN", "  %s: deleted — skipped", f)
		}
	}
	logf("INFO", "Agent routing:")
	for agent, files := range reviewAgents {
		logf("INFO", "  %s: %d file(s)", agent, len(files))
	}
	manifest := map[string]any{
		"base_branch":   baseBranch,
		"refspec":       refspec,
		"review_agents": reviewAgents,
		"reviewable":    tasks,
		"excluded":      excluded,
	}
	if err := writeJSON(filepath.Join(stagingDir, "manifest.json"), manifest); err != nil {
		return nil, nil, nil, err
	}
	logf("INFO", "Staged %d review task(s) for %d file(s)", len(tasks), len(reviewable))
	logf("INFO", "Manifest: %s", filepath.Join(stagingDir, "manifest.json"))
	return tasks, reviewAgents, excluded, nil
}
