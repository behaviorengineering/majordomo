package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeriveTitleFromH1(t *testing.T) {
	got := DeriveTitle("x.md", "<h1>Hello <em>World</em></h1><p>body</p>")
	if got != "Hello World" {
		t.Fatalf("got %q", got)
	}
}

func TestDeriveTitleFallbackStem(t *testing.T) {
	got := DeriveTitle("/tmp/tech-review.md", "<p>no heading</p>")
	if got != "Tech Review" {
		t.Fatalf("got %q", got)
	}
}

func TestConvertMarkdownToHTMLSmoke(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "summary.md")
	htmlOut := filepath.Join(dir, "summary.html")
	src := "# Summary\n\nHello **world**.\n\n```go\nfmt.Println(1)\n```\n\n| a | b |\n| - | - |\n| 1 | 2 |\n"
	if err := os.WriteFile(md, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ConvertMarkdownToHTML(md, htmlOut); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(htmlOut)
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	if !strings.Contains(page, "<!DOCTYPE html>") {
		t.Fatal("missing doctype")
	}
	if !strings.Contains(page, "<title>Summary</title>") {
		t.Fatalf("title missing: %s", page[:200])
	}
	if !strings.Contains(page, "<h1") {
		t.Fatal("missing h1")
	}
	if !strings.Contains(page, "<table") {
		t.Fatal("missing table")
	}
	if !strings.Contains(page, "<pre") {
		t.Fatal("missing pre/code fence")
	}
}
