package contextdigest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

const (
	findingCommentsRel      = "evidence/typology/finding_comments.json"
	findingCommentBodiesRel = "evidence/typology/finding_comment_bodies.json"
	findingMarkerPrefix     = "<!-- majordomo-finding:"
	findingMarkerSuffix     = " -->"
)

var findingMarkerRE = regexp.MustCompile(`<!--\s*majordomo-finding:([a-f0-9]+)\s*-->`)

// FindingCommentsSidecar maps fingerprints to forge comment ids for upsert.
type FindingCommentsSidecar struct {
	ByFingerprint map[string]string `json:"by_fingerprint"`
}

// FindingCommentBodiesFile stores generated counsel before PR sync.
type FindingCommentBodiesFile struct {
	Comments []FindingCommentBody `json:"comments"`
}

func findingFingerprint(finding string) string {
	norm := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(finding))), " ")
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:8])
}

func findingCommentMarker(fingerprint string) string {
	return findingMarkerPrefix + fingerprint + findingMarkerSuffix
}

var findingCommentBodyTmpl = template.Must(template.New("findingCommentBody").Funcs(template.FuncMap{
	"code": func(s string) string { return "`" + s + "`" },
}).Parse(`{{.Marker}}

{{if .Cleared}}{{.Counsel}}
{{else}}## Open architecture finding

{{.Counsel}}

_Reply on this comment to discuss. Gate the context PR with {{code "@majordomo done"}} or {{code "@majordomo reject <reason>"}} when ready._
{{end}}
`))

func formatFindingCommentBody(fingerprint, counsel string) string {
	counsel = strings.TrimSpace(counsel)
	var b strings.Builder
	if err := findingCommentBodyTmpl.Execute(&b, struct {
		Marker  string
		Counsel string
		Cleared bool
	}{
		Marker:  findingCommentMarker(fingerprint),
		Counsel: counsel,
		Cleared: strings.HasPrefix(strings.ToLower(counsel), "cleared"),
	}); err != nil {
		panic(fmt.Sprintf("finding comment template: %v", err))
	}
	return b.String()
}

func parseFindingFingerprint(body string) string {
	m := findingMarkerRE.FindStringSubmatch(body)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

func saveFindingCommentBodies(evidenceDir string, comments []FindingCommentBody) error {
	ctxDir := filepath.Dir(filepath.Dir(evidenceDir))
	path := filepath.Join(ctxDir, findingCommentBodiesRel)
	if len(comments) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove finding comment bodies: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("finding comment bodies mkdir: %w", err)
	}
	data, err := json.MarshalIndent(FindingCommentBodiesFile{Comments: comments}, "", "  ")
	if err != nil {
		return fmt.Errorf("finding comment bodies encode: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func loadFindingCommentBodies(ctxDir string) ([]FindingCommentBody, error) {
	path := filepath.Join(ctxDir, findingCommentBodiesRel)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var file FindingCommentBodiesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("finding comment bodies decode: %w", err)
	}
	return file.Comments, nil
}

func loadFindingCommentsSidecar(ctxDir string) (FindingCommentsSidecar, error) {
	path := filepath.Join(ctxDir, findingCommentsRel)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FindingCommentsSidecar{ByFingerprint: map[string]string{}}, nil
		}
		return FindingCommentsSidecar{}, err
	}
	var s FindingCommentsSidecar
	if err := json.Unmarshal(data, &s); err != nil {
		return FindingCommentsSidecar{}, fmt.Errorf("finding comments sidecar decode: %w", err)
	}
	if s.ByFingerprint == nil {
		s.ByFingerprint = map[string]string{}
	}
	return s, nil
}

func saveFindingCommentsSidecar(ctxDir string, s FindingCommentsSidecar) error {
	if s.ByFingerprint == nil {
		s.ByFingerprint = map[string]string{}
	}
	path := filepath.Join(ctxDir, findingCommentsRel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("finding comments sidecar mkdir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("finding comments sidecar encode: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// PRCommentAPI posts and updates ordinary PR/MR comments.
type PRCommentAPI interface {
	ListPRCommentsWithIDs(prNumber string) ([]PRCommentWithID, error)
	PostPRComment(prNumber, body string) (string, error)
	UpdatePRComment(prNumber, commentID, body string) error
}

// SyncFindingPRComments upserts one marked PR comment per open finding counsel body.
func SyncFindingPRComments(api PRCommentAPI, prNumber, ctxDir string) error {
	if api == nil || strings.TrimSpace(prNumber) == "" {
		return nil
	}
	bodies, err := loadFindingCommentBodies(ctxDir)
	if err != nil {
		return err
	}
	sidecar, err := loadFindingCommentsSidecar(ctxDir)
	if err != nil {
		return err
	}
	listed, err := api.ListPRCommentsWithIDs(prNumber)
	if err != nil {
		return fmt.Errorf("list finding comments: %w", err)
	}
	for _, c := range listed {
		fp := parseFindingFingerprint(c.Body)
		if fp == "" {
			continue
		}
		if _, ok := sidecar.ByFingerprint[fp]; !ok {
			sidecar.ByFingerprint[fp] = c.ID
		}
	}

	active := map[string]struct{}{}
	for _, body := range bodies {
		fp := strings.TrimSpace(body.Fingerprint)
		if fp == "" {
			fp = findingFingerprint(body.Finding)
		}
		active[fp] = struct{}{}
		text := formatFindingCommentBody(fp, body.Body)
		id := strings.TrimSpace(sidecar.ByFingerprint[fp])
		if id == "" {
			newID, err := api.PostPRComment(prNumber, text)
			if err != nil {
				return fmt.Errorf("post finding comment %s: %w", fp, err)
			}
			sidecar.ByFingerprint[fp] = newID
			continue
		}
		if err := api.UpdatePRComment(prNumber, id, text); err != nil {
			newID, postErr := api.PostPRComment(prNumber, text)
			if postErr != nil {
				return fmt.Errorf("update finding comment %s: %v; recreate: %w", fp, err, postErr)
			}
			sidecar.ByFingerprint[fp] = newID
		}
	}

	for fp, id := range sidecar.ByFingerprint {
		if _, ok := active[fp]; ok {
			continue
		}
		cleared := formatFindingCommentBody(fp, "Cleared by digest: this architecture finding is no longer open.")
		if strings.TrimSpace(id) == "" {
			continue
		}
		if err := api.UpdatePRComment(prNumber, id, cleared); err != nil {
			logf("WARN", "clear finding comment %s: %v", fp, err)
		}
	}
	return saveFindingCommentsSidecar(ctxDir, sidecar)
}
