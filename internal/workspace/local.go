package workspace

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

// Local is a Port backed by the filesystem under Root.
type Local struct {
	Root string
}

// NewLocal returns a Local Port for root.
func NewLocal(root string) (*Local, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("workspace: root %q is not a directory", abs)
	}
	return &Local{Root: abs}, nil
}

func (l *Local) Read(_ context.Context, path string) ([]byte, error) {
	full, err := resolvePath(l.Root, path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("workspace read %q: %w", path, err)
	}
	return data, nil
}

func (l *Local) Grep(_ context.Context, pattern, path string) ([]Match, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("workspace grep: %w", err)
	}
	full, err := resolvePath(l.Root, path)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("workspace grep %q: %w", path, err)
	}
	var out []Match
	if fi.IsDir() {
		err = filepath.WalkDir(full, func(p string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(l.Root, p)
			if err != nil {
				return err
			}
			ms, err := grepFile(re, rel, p)
			if err != nil {
				return err
			}
			out = append(out, ms...)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("workspace grep %q: %w", path, err)
		}
		return out, nil
	}
	rel, err := filepath.Rel(l.Root, full)
	if err != nil {
		return nil, err
	}
	return grepFile(re, rel, full)
}

func grepFile(re *regexp.Regexp, rel, full string) ([]Match, error) {
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Match
	sc := bufio.NewScanner(f)
	// Allow long lines in source files.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if re.MatchString(text) {
			out = append(out, Match{Path: filepath.ToSlash(rel), Line: line, Content: text})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (l *Local) Edit(_ context.Context, path string, content []byte) error {
	full, err := resolvePath(l.Root, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("workspace edit %q: %w", path, err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return fmt.Errorf("workspace edit %q: %w", path, err)
	}
	return nil
}

func (l *Local) Shell(ctx context.Context, argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("workspace shell: empty argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = l.Root
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	return outBuf.Bytes(), errBuf.Bytes(), runErr
}
