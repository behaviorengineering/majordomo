package contextstore

import (
	"path/filepath"
	"testing"
)

func TestParseAndValidateMeta(t *testing.T) {
	m, err := ParseMeta(filepath.Join("testdata", "valid", "meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateMeta(m); err != nil {
		t.Fatal(err)
	}
	if m.RepoID != "payments-api" || m.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("meta = %+v", m)
	}
}

func TestValidateMetaBootstrapEmptyDigest(t *testing.T) {
	m := Meta{SchemaVersion: 1, RepoID: "demo"}
	if err := ValidateMeta(m); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMetaRejectsUnknownSchema(t *testing.T) {
	m, err := ParseMeta(filepath.Join("testdata", "bad-meta", "meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateMeta(m); err == nil {
		t.Fatal("expected schema_version error")
	}
}

func TestValidateMetaRejectsBadTimestamp(t *testing.T) {
	m := Meta{SchemaVersion: 1, RepoID: "demo", LastDigestAt: "yesterday"}
	if err := ValidateMeta(m); err == nil {
		t.Fatal("expected last_digest_at error")
	}
}

func TestValidateMetaRejectsEmptyRepoID(t *testing.T) {
	m := Meta{SchemaVersion: 1}
	if err := ValidateMeta(m); err == nil {
		t.Fatal("expected repo_id error")
	}
}
