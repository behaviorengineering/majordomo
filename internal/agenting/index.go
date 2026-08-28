package agenting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ModeFiles      = "files"
	ModeSummary    = "summary"
	ModeTechnical  = "technical"
	ModeDigest     = "digest"
	IndexRelPath   = "agenting/index.yaml"
	GroundingName  = "GROUNDING.md"
)

// Index is agenting/index.yaml on the context branch.
type Index struct {
	Packs map[string]Pack `yaml:"packs"`
	order []string
}

// Pack maps a pack id to selection rules.
type Pack struct {
	Globs []string `yaml:"globs,omitempty"`
	Modes []string `yaml:"modes"`
}

// LoadIndex reads agenting/index.yaml under contextDir.
func LoadIndex(contextDir string) (Index, error) {
	path := filepath.Join(contextDir, IndexRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return Index{}, fmt.Errorf("read %s: %w", IndexRelPath, err)
	}
	var doc struct {
		Packs map[string]Pack `yaml:"packs"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Index{}, fmt.Errorf("parse %s: %w", IndexRelPath, err)
	}
	if len(doc.Packs) == 0 {
		return Index{}, fmt.Errorf("%s: packs is required", IndexRelPath)
	}
	order := packOrder(data)
	if len(order) == 0 {
		for id := range doc.Packs {
			order = append(order, id)
		}
	}
	idx := Index{Packs: doc.Packs, order: order}
	if err := ValidateIndex(idx); err != nil {
		return Index{}, err
	}
	return idx, nil
}

func packOrder(raw []byte) []string {
	var root yaml.Node
	if yaml.Unmarshal(raw, &root) != nil || len(root.Content) == 0 {
		return nil
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == "packs" && doc.Content[i+1].Kind == yaml.MappingNode {
			var ids []string
			for j := 0; j+1 < len(doc.Content[i+1].Content); j += 2 {
				ids = append(ids, doc.Content[i+1].Content[j].Value)
			}
			return ids
		}
	}
	return nil
}

// ValidateIndex returns nil when every pack has modes and known mode names.
func ValidateIndex(idx Index) error {
	if len(idx.Packs) == 0 {
		return fmt.Errorf("agenting index: no packs defined")
	}
	for id, pack := range idx.Packs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("agenting index: empty pack id")
		}
		if len(pack.Modes) == 0 {
			return fmt.Errorf("agenting index: pack %q requires modes", id)
		}
		for _, m := range pack.Modes {
			if !validMode(m) {
				return fmt.Errorf("agenting index: pack %q has unknown mode %q", id, m)
			}
		}
	}
	return nil
}

func validMode(m string) bool {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case ModeFiles, ModeSummary, ModeTechnical, ModeDigest:
		return true
	default:
		return false
	}
}

// PackIDs returns pack ids in index file order.
func (idx Index) PackIDs() []string {
	out := make([]string, len(idx.order))
	copy(out, idx.order)
	return out
}
