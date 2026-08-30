package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type groundingPack struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

// GroundingPaths returns absolute paths to staged grounding files for a batch manifest.
// skillDir is where the agent reads manifest/SKILL.md (batch dir or batch/<skill>).
func GroundingPaths(batchDir, skillDir string) ([]string, error) {
	batchDir = filepath.Clean(batchDir)
	packs, err := loadGroundingPacks(batchDir)
	if err != nil {
		return nil, err
	}
	if len(packs) == 0 {
		return nil, nil
	}
	skillDir = filepath.Clean(skillDir)
	var paths []string
	for _, p := range packs {
		if strings.TrimSpace(p.File) == "" {
			continue
		}
		rel := filepath.FromSlash(strings.TrimPrefix(p.File, "./"))
		candidates := []string{
			filepath.Join(skillDir, rel),
			filepath.Join(batchDir, rel),
		}
		var found string
		for _, c := range candidates {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				found = c
				break
			}
		}
		if found == "" {
			return nil, fmt.Errorf("grounding pack %q: file not found (%s)", p.ID, p.File)
		}
		paths = append(paths, found)
	}
	return paths, nil
}

// GroundingSkillDir is the directory where grounding files live beside the agent manifest.
func GroundingSkillDir(stagingDir string, mode Mode) (string, error) {
	stagingDir = filepath.Clean(stagingDir)
	switch mode {
	case ModeProse, ModeFinalize, ModeScore, ModeTechScore:
		return "", nil
	case ModeTechnicalDeep:
		return stagingDir, nil
	}
	skill, err := manifestSkill(stagingDir)
	if err != nil {
		return "", err
	}
	if skill == "" {
		return "", nil
	}
	return filepath.Join(stagingDir, skill), nil
}

func loadGroundingPacks(batchDir string) ([]groundingPack, error) {
	data, err := os.ReadFile(filepath.Join(batchDir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var raw struct {
		GroundingPacks []groundingPack `json:"grounding_packs"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode manifest grounding_packs: %w", err)
	}
	return raw.GroundingPacks, nil
}

func manifestSkill(batchDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(batchDir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read manifest: %w", err)
	}
	var raw struct {
		ReviewAgents map[string][]string `json:"review_agents"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("decode manifest review_agents: %w", err)
	}
	for skill := range raw.ReviewAgents {
		if strings.TrimSpace(skill) != "" {
			return skill, nil
		}
	}
	return "", nil
}
