package staging

import (
	"encoding/json"
	"os"
	"path/filepath"

	contextprovider "github.com/behaviorengineering/majordomo/pkg/context/provider"
)

// WriteContextPin records the pinned context checkout on batch-plan.json for the run.
func WriteContextPin(stagingDir string, snap *contextprovider.Snapshot) error {
	if snap == nil {
		return nil
	}
	path := filepath.Join(stagingDir, "batch-plan.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var plan map[string]any
	if err := json.Unmarshal(data, &plan); err != nil {
		return err
	}
	pin := map[string]any{
		"checkout_sha":  snap.Provenance.ResolvedCommit,
		"source_commit": snap.Provenance.SourceCommit,
	}
	if !snap.Provenance.GeneratedAt.IsZero() {
		pin["generated_at"] = snap.Provenance.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	plan["context_pin"] = pin
	out, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o644)
}
