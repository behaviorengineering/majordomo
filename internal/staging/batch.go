package staging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/cluster"
)

// BatchEntry is one row in batch-plan.json.
type BatchEntry struct {
	Skill      string `json:"skill"`
	BatchNum   string `json:"batch_num"`
	TaskCount  int    `json:"task_count"`
	StagingDir string `json:"staging_dir"`
}

// StageSkillBatches writes per-skill batch dirs and returns plan entries + skill names.
func StageSkillBatches(
	tasks []Task,
	reviewAgents map[string][]string,
	excluded []string,
	stagingDir, repoRoot, baseBranch, refspec string,
	batchSize int,
) ([]BatchEntry, []string, error) {
	bySkill := map[string][]Task{}
	skillOrder := []string{}
	for _, task := range tasks {
		agent, _ := task["agent"].(string)
		if _, ok := bySkill[agent]; !ok {
			skillOrder = append(skillOrder, agent)
		}
		bySkill[agent] = append(bySkill[agent], task)
	}

	docChanged := []string{}
	for _, t := range tasks {
		f, _ := t["file"].(string)
		if strings.HasSuffix(f, ".md") {
			docChanged = append(docChanged, f)
		}
	}
	var corpusIndex []map[string]any
	if len(docChanged) > 0 {
		corpusIndex = cluster.BuildCorpusIndex(repoRoot)
		logf("INFO", "Corpus index: %d .md file(s) indexed", len(corpusIndex))
	}

	batchEntries := []BatchEntry{}
	for _, skill := range skillOrder {
		skillTasks := bySkill[skill]
		skillStaging := filepath.Join(stagingDir, skill)
		if err := os.MkdirAll(skillStaging, 0o755); err != nil {
			return nil, nil, err
		}
		skillManifest := map[string]any{
			"base_branch":    baseBranch,
			"refspec":        refspec,
			"skill_dir":      skill,
			"review_agents":  map[string][]string{skill: reviewAgents[skill]},
			"reviewable":     skillTasks,
			"excluded":       excluded,
		}
		if err := writeJSON(filepath.Join(skillStaging, "manifest.json"), skillManifest); err != nil {
			return nil, nil, err
		}

		skillMD := []string{}
		for _, t := range skillTasks {
			f, _ := t["file"].(string)
			if strings.HasSuffix(f, ".md") {
				skillMD = append(skillMD, f)
			}
		}
		skillHasMD := len(skillMD) > 0
		var skillDocClusters [][]string
		var skillReverseLinks map[string][]string
		var batches [][]map[string]any
		asMaps := tasksToMaps(skillTasks)
		if skillHasMD {
			batches = cluster.DocClusterAwareBatches(asMaps, batchSize, repoRoot)
			for _, c := range cluster.ClusterDocs(skillMD, repoRoot) {
				if len(c) > 1 {
					skillDocClusters = append(skillDocClusters, c)
				}
			}
			skillReverseLinks = cluster.ReverseLinks(skillMD, repoRoot)
		} else {
			batches = cluster.DepClusterAwareBatches(asMaps, batchSize, repoRoot)
		}

		for batchIdx, batchSlice := range batches {
			batchNum := batchIdx + 1
			batchDir := filepath.Join(skillStaging, fmtBatchDir(batchNum))
			if err := os.MkdirAll(batchDir, 0o755); err != nil {
				return nil, nil, err
			}
			batchManifest := map[string]any{
				"base_branch":   baseBranch,
				"refspec":       refspec,
				"skill_dir":     skill,
				"review_agents": map[string][]string{skill: reviewAgents[skill]},
				"reviewable":    batchSlice,
				"excluded":      excluded,
			}
			if skillHasMD {
				batchManifest["doc_clusters"] = skillDocClusters
				batchManifest["reverse_links"] = skillReverseLinks
			}
			if err := writeJSON(filepath.Join(batchDir, "manifest.json"), batchManifest); err != nil {
				return nil, nil, err
			}
			for _, task := range batchSlice {
				inputFile, _ := task["input_file"].(string)
				src := filepath.Join(stagingDir, inputFile)
				dst := filepath.Join(batchDir, inputFile)
				if err := copyFile(src, dst); err != nil {
					// missing source is skipped like Python (only copy if exists)
					continue
				}
			}
			if skillHasMD && len(corpusIndex) > 0 {
				if err := writeJSON(filepath.Join(batchDir, "corpus-index.json"), corpusIndex); err != nil {
					return nil, nil, err
				}
			}
			dirs := map[string]struct{}{}
			for _, t := range batchSlice {
				f, _ := t["file"].(string)
				dirs[filepath.ToSlash(filepath.Dir(f))] = struct{}{}
			}
			dirList := make([]string, 0, len(dirs))
			for d := range dirs {
				dirList = append(dirList, d)
			}
			sortStrings(dirList)
			logf("INFO", "  Batch %03d: %d task(s) from %d dir(s): %s",
				batchNum, len(batchSlice), len(dirList), strings.Join(dirList, ", "))
			batchEntries = append(batchEntries, BatchEntry{
				Skill:      skill,
				BatchNum:   fmt.Sprintf("%03d", batchNum),
				TaskCount:  len(batchSlice),
				StagingDir: batchDir,
			})
		}
		logf("INFO", "Skill %s: %d task(s) → %d batch(es) (directory-aware)",
			skill, len(skillTasks), len(batches))
	}
	return batchEntries, skillOrder, nil
}

func tasksToMaps(tasks []Task) []map[string]any {
	out := make([]map[string]any, len(tasks))
	for i, t := range tasks {
		out[i] = map[string]any(t)
	}
	return out
}

func fmtBatchDir(n int) string {
	return fmt.Sprintf("batch_%03d", n)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func sortStrings(s []string) {
	sort.Strings(s)
}
