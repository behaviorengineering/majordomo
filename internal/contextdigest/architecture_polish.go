package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// polishTypologyArchitectureBrief rewrites known incomplete shapes from older
// Typology architecture templates so context evidence stays readable.
func polishTypologyArchitectureBrief(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("polish typology architecture: read %s: %w", path, err)
	}
	body := polishTypologyArchitectureBriefText(string(raw), filepath.Dir(path))
	body = ensureTypologyArchitectureBanner(body)
	if body == string(raw) {
		return nil
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("polish typology architecture: write %s: %w", path, err)
	}
	return nil
}

func polishTypologyArchitectureBriefText(body, evidenceDir string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		switch {
		case strings.HasPrefix(trim, "- Surfaces:") && strings.TrimSpace(strings.TrimPrefix(trim, "- Surfaces:")) == "":
			lines[i] = indent + "- Surfaces: _(none)_"
		case strings.HasPrefix(trim, "- Programs:") && strings.TrimSpace(strings.TrimPrefix(trim, "- Programs:")) == "":
			lines[i] = indent + "- Programs: _(none)_"
		}
	}
	body = strings.Join(lines, "\n")
	return rewriteModuleScopePackageInventory(body, evidenceDir)
}

func rewriteModuleScopePackageInventory(body, evidenceDir string) string {
	const marker = "Go packages** in the inspected modules:"
	idx := strings.Index(body, marker)
	if idx < 0 {
		return body
	}
	start := idx + len(marker)
	// Skip blank lines after the sentence.
	rest := body[start:]
	bulletStart := 0
	for bulletStart < len(rest) && (rest[bulletStart] == '\n' || rest[bulletStart] == '\r') {
		bulletStart++
	}
	lines := strings.Split(rest[bulletStart:], "\n")
	var bullets []string
	endLine := 0
	for ; endLine < len(lines); endLine++ {
		trim := strings.TrimSpace(lines[endLine])
		if trim == "" {
			if len(bullets) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trim, "#") {
			break
		}
		if !strings.HasPrefix(trim, "- ") {
			break
		}
		bullets = append(bullets, trim)
	}
	if len(bullets) == 0 || !packageInventoryLooksLikeModuleScope(strings.Join(bullets, "\n")+"\n") {
		return body
	}
	replacement := packageInventoryFromGraph(filepath.Join(evidenceDir, "graph.txt"))
	if replacement == "" {
		replacement = "- _(see high-coupling and leaf package tables below)_\n"
	}
	if !strings.HasSuffix(replacement, "\n") {
		replacement += "\n"
	}

	// Rebuild: prefix through marker + newline + replacement + remaining after bullets.
	prefix := body[:start]
	if !strings.HasSuffix(prefix, "\n") {
		prefix += "\n"
	}
	// Consume the same number of lines we scanned (including leading blanks before first bullet).
	consumed := rest[:bulletStart]
	for i := 0; i < endLine; i++ {
		consumed += lines[i] + "\n"
	}
	// If we stopped on a blank line after bullets, keep one blank for spacing.
	suffix := rest[len(consumed):]
	return prefix + replacement + suffix
}

func packageInventoryLooksLikeModuleScope(bullets string) bool {
	lines := strings.Split(bullets, "\n")
	saw := 0
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if !strings.HasPrefix(trim, "- ") {
			return false
		}
		saw++
		item := strings.TrimSpace(strings.TrimPrefix(trim, "- "))
		item = strings.Trim(item, "`")
		if item == "" {
			return false
		}
		if item == "." || item == "./" {
			continue
		}
		// Real packages usually contain a path separator.
		if strings.Contains(item, "/") {
			return false
		}
	}
	return saw > 0
}

func packageInventoryFromGraph(graphPath string) string {
	raw, err := os.ReadFile(graphPath)
	if err != nil {
		return ""
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, line := range strings.Split(string(raw), "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "- ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trim, "- "))
		pkg := rest
		if i := strings.Index(rest, " "); i > 0 {
			pkg = rest[:i]
		}
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || !strings.Contains(pkg, "/") {
			continue
		}
		if _, ok := seen[pkg]; ok {
			continue
		}
		seen[pkg] = struct{}{}
		paths = append(paths, pkg)
	}
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "- `%s`\n", p)
	}
	return b.String()
}
