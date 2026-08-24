package cluster

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestClusterDocs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.md"), "See [b](./b.md)\n")
	writeFile(t, filepath.Join(root, "b.md"), "Back to [a](a.md)\n")
	writeFile(t, filepath.Join(root, "c.md"), "Standalone\n")

	clusters := sortedClusters(ClusterDocs([]string{"a.md", "b.md", "c.md"}, root))
	want := [][]string{{"a.md", "b.md"}, {"c.md"}}
	if !reflect.DeepEqual(clusters, want) {
		t.Fatalf("clusters = %#v, want %#v", clusters, want)
	}
}

func TestReverseLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "target.md"), "# Target\n")
	writeFile(t, filepath.Join(root, "linker.md"), "See [target](target.md)\n")
	writeFile(t, filepath.Join(root, "node_modules", "skip.md"), "[target](target.md)\n")

	got := ReverseLinks([]string{"target.md"}, root)
	want := map[string][]string{"target.md": {"linker.md"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reverse links = %#v, want %#v", got, want)
	}
}

func TestBuildCorpusIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "guide.md"), "# Guide Title\n\n## Section\n\nUse **`config`** and **setup**.\n\n[other](other.md)\n")
	writeFile(t, filepath.Join(root, "other.md"), "# Other\n")

	entries := BuildCorpusIndex(root)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	first := entries[0]
	if first["file"] != "guide.md" {
		t.Fatalf("first file = %v, want guide.md", first["file"])
	}
	if first["title"] != "Guide Title" {
		t.Fatalf("title = %v", first["title"])
	}
	headings, ok := first["headings"].([]string)
	if !ok || len(headings) != 1 || headings[0] != "Section" {
		t.Fatalf("headings = %#v", first["headings"])
	}
	terms, ok := first["key_terms"].([]string)
	if !ok || !reflect.DeepEqual(terms, []string{"`config`", "config", "setup"}) {
		t.Fatalf("key_terms = %#v", first["key_terms"])
	}
	links, ok := first["links_out"].([]string)
	if !ok || len(links) != 1 || links[0] != "other.md" {
		t.Fatalf("links_out = %#v", first["links_out"])
	}
}

func TestDocClusterAwareBatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.md"), "[b](b.md)\n")
	writeFile(t, filepath.Join(root, "b.md"), "[a](a.md)\n")
	writeFile(t, filepath.Join(root, "c.md"), "solo\n")

	tasks := []map[string]any{
		{"file": "a.md"},
		{"file": "b.md"},
		{"file": "c.md"},
	}

	batches := DocClusterAwareBatches(tasks, 2, root)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	if len(batches[0]) != 2 || len(batches[1]) != 1 {
		t.Fatalf("unexpected batch sizes: %#v", batches)
	}
}

func TestParseMDLinksSkipsImages(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	writeFile(t, path, "![img](img.png)\n[link](target.md)\n")
	writeFile(t, filepath.Join(root, "target.md"), "# Target\n")

	targets := map[string]struct{}{"target.md": {}}
	found := parseMDLinks(path, root, targets)
	if len(found) != 1 {
		t.Fatalf("expected one link, got %#v", found)
	}
	if _, ok := found["target.md"]; !ok {
		t.Fatalf("expected target.md link, got %#v", found)
	}
}
