package contextgate

import (
	"encoding/json"
	"fmt"
	"os"
)

// FileStore is a minimal JSON-backed humanreview.Store for tests (optional extension point).
type FileStore struct {
	Path string
}

type evalRow struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Load reads evaluations from path.
func (f *FileStore) Load() ([]evalRow, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []evalRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// Save writes evaluations to path.
func (f *FileStore) Save(rows []evalRow) error {
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(f.Path, data, 0o644)
}

// AppendStatus appends one evaluation status row (debug/audit).
func (f *FileStore) AppendStatus(id, status string) error {
	rows, err := f.Load()
	if err != nil {
		return err
	}
	rows = append(rows, evalRow{ID: id, Status: status})
	return f.Save(rows)
}

// ErrNotImplemented marks optional strop Store methods not used in v1 gate sidecar flow.
var ErrNotImplemented = fmt.Errorf("contextgate: file store is audit-only in v1")
