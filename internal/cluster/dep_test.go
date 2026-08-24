package cluster

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sortedClusters(clusters [][]string) [][]string {
	out := append([][]string(nil), clusters...)
	for _, cluster := range out {
		sort.Strings(cluster)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		if len(out[i]) == 0 {
			return false
		}
		return out[i][0] < out[j][0]
	})
	return out
}

func TestUnionFind(t *testing.T) {
	uf := NewUnionFind([]string{"a", "b", "c", "d"})
	uf.Union("a", "b")
	uf.Union("c", "d")
	if uf.Find("a") != uf.Find("b") {
		t.Fatal("expected a and b in same component")
	}
	if uf.Find("c") != uf.Find("d") {
		t.Fatal("expected c and d in same component")
	}
	if uf.Find("a") == uf.Find("c") {
		t.Fatal("expected separate components")
	}

	components := sortedClusters(uf.Components())
	want := [][]string{{"a", "b"}, {"c", "d"}}
	if !reflect.DeepEqual(components, want) {
		t.Fatalf("components = %#v, want %#v", components, want)
	}
}

func TestClusterFilesPython(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pkg", "__init__.py"), "")
	writeFile(t, filepath.Join(root, "pkg", "a.py"), "from pkg.b import x\n")
	writeFile(t, filepath.Join(root, "pkg", "b.py"), "from pkg.a import y\n")
	writeFile(t, filepath.Join(root, "other.py"), "import pkg.a\n")

	clusters := sortedClusters(ClusterFiles([]string{
		"pkg/a.py",
		"pkg/b.py",
		"other.py",
	}, root))

	want := [][]string{{"other.py", "pkg/a.py", "pkg/b.py"}}
	if !reflect.DeepEqual(clusters, want) {
		t.Fatalf("clusters = %#v, want %#v", clusters, want)
	}
}

func TestClusterFilesJS(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "lib", "a.js"), "import x from './b'\n")
	writeFile(t, filepath.Join(root, "lib", "b.js"), "const y = require('./a')\n")
	writeFile(t, filepath.Join(root, "main.js"), "import './lib/a'\n")

	clusters := sortedClusters(ClusterFiles([]string{
		"lib/a.js",
		"lib/b.js",
		"main.js",
	}, root))

	want := [][]string{{"lib/a.js", "lib/b.js"}, {"main.js"}}
	if !reflect.DeepEqual(clusters, want) {
		t.Fatalf("clusters = %#v, want %#v", clusters, want)
	}
}

func TestReverseDeps(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "changed.py"), "x = 1\n")
	writeFile(t, filepath.Join(root, "caller.py"), "import changed\n")
	writeFile(t, filepath.Join(root, "node_modules", "ignored.js"), "import changed\n")

	got := ReverseDeps([]string{"changed.py"}, root)
	want := map[string][]string{"changed.py": {"caller.py"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reverse deps = %#v, want %#v", got, want)
	}
}

func TestDepClusterAwareBatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.py"), "import b\n")
	writeFile(t, filepath.Join(root, "b.py"), "x = 1\n")
	writeFile(t, filepath.Join(root, "c.py"), "y = 2\n")

	tasks := []map[string]any{
		{"file": "a.py", "chunk": 0},
		{"file": "b.py", "chunk": 0},
		{"file": "c.py", "chunk": 0},
	}

	batches := DepClusterAwareBatches(tasks, 2, root)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	if len(batches[0]) != 2 || len(batches[1]) != 1 {
		t.Fatalf("unexpected batch sizes: %#v", batches)
	}
}
