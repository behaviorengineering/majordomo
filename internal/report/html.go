package report

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>__TITLE__</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; }
    body {
      font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
      font-size: 14px;
      line-height: 1.6;
      color: #24292f;
      background: #ffffff;
      max-width: 900px;
      margin: 40px auto;
      padding: 0 24px 60px;
    }
    h1, h2, h3, h4, h5, h6 {
      margin-top: 1.5em;
      margin-bottom: 0.5em;
      font-weight: 600;
      line-height: 1.25;
    }
    h1 { font-size: 2em; border-bottom: 1px solid #d0d7de; padding-bottom: 0.3em; }
    h2 { font-size: 1.5em; border-bottom: 1px solid #d0d7de; padding-bottom: 0.3em; }
    h3 { font-size: 1.25em; }
    p { margin: 0 0 1em; }
    a { color: #0969da; text-decoration: none; }
    a:hover { text-decoration: underline; }
    code {
      font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
      font-size: 85%;
      background: #f6f8fa;
      padding: 0.2em 0.4em;
      border-radius: 6px;
    }
    pre {
      background: #f6f8fa;
      padding: 16px;
      overflow: auto;
      border-radius: 6px;
      line-height: 1.45;
    }
    pre code {
      background: transparent;
      padding: 0;
      font-size: 100%;
    }
    blockquote {
      margin: 0 0 1em;
      padding: 0 1em;
      color: #57606a;
      border-left: 4px solid #d0d7de;
    }
    table {
      border-collapse: collapse;
      width: 100%;
      margin-bottom: 1em;
    }
    th, td {
      border: 1px solid #d0d7de;
      padding: 6px 13px;
      text-align: left;
    }
    tr:nth-child(even) { background: #f6f8fa; }
    hr { border: none; border-top: 1px solid #d0d7de; margin: 1.5em 0; }
    ul, ol { padding-left: 2em; margin: 0 0 1em; }
    li { margin: 0.25em 0; }
  </style>
</head>
<body>
__BODY__
</body>
</html>
`

var (
	h1Re      = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	tagStrip  = regexp.MustCompile(`<[^>]+>`)
	mdRenderer = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
)

// DeriveTitle extracts the first H1 from HTML body, else title-cases the file stem.
func DeriveTitle(mdPath, htmlBody string) string {
	if m := h1Re.FindStringSubmatch(htmlBody); m != nil {
		raw := tagStrip.ReplaceAllString(m[1], "")
		return strings.TrimSpace(raw)
	}
	stem := strings.TrimSuffix(filepath.Base(mdPath), filepath.Ext(mdPath))
	stem = strings.ReplaceAll(stem, "-", " ")
	stem = strings.ReplaceAll(stem, "_", " ")
	return titleCase(stem)
}

func titleCase(s string) string {
	parts := strings.Fields(s)
	for i, p := range parts {
		if p == "" {
			continue
		}
		lower := strings.ToLower(p)
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}

// ConvertMarkdownToHTML converts mdPath to a self-contained HTML file at htmlPath.
func ConvertMarkdownToHTML(mdPath, htmlPath string) error {
	source, err := os.ReadFile(mdPath)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := mdRenderer.Convert(source, &buf); err != nil {
		return err
	}
	body := buf.String()
	title := DeriveTitle(mdPath, body)
	page := strings.Replace(htmlTemplate, "__TITLE__", title, 1)
	page = strings.Replace(page, "__BODY__", body, 1)
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(htmlPath, []byte(page), 0o644)
}

// ConvertMarkdownToHTMLCLI is the CLI entry matching md-to-html.py main().
func ConvertMarkdownToHTMLCLI(mdPath, htmlPath string) error {
	if _, err := os.Stat(mdPath); err != nil {
		return fmt.Errorf("ERROR: input file not found: %s", mdPath)
	}
	if err := ConvertMarkdownToHTML(mdPath, htmlPath); err != nil {
		return err
	}
	fmt.Printf("Converted: %s → %s\n", mdPath, htmlPath)
	return nil
}
