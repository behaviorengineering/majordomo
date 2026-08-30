package workspace

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"sync"
)

// Stub is an in-memory Port for tests. No filesystem or process.
type Stub struct {
	mu    sync.Mutex
	Files map[string][]byte
	// ShellResult is returned from Shell when set; otherwise Shell errors.
	ShellStdout []byte
	ShellStderr []byte
	ShellErr    error
}

// NewStub returns an empty Stub.
func NewStub() *Stub {
	return &Stub{Files: map[string][]byte{}}
}

func (s *Stub) key(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("workspace: empty path")
	}
	if path.IsAbs(p) || strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%w: absolute path %q", ErrEscape, p)
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: %q", ErrEscape, p)
	}
	return clean, nil
}

func (s *Stub) Read(_ context.Context, p string) ([]byte, error) {
	k, err := s.key(p)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.Files[k]
	if !ok {
		return nil, fmt.Errorf("workspace read %q: not found", p)
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func (s *Stub) Grep(_ context.Context, pattern, p string) ([]Match, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("workspace grep: %w", err)
	}
	prefix, err := s.key(p)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Match
	for name, data := range s.Files {
		if name != prefix && !strings.HasPrefix(name, prefix+"/") {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				out = append(out, Match{Path: name, Line: i + 1, Content: line})
			}
		}
	}
	return out, nil
}

func (s *Stub) Edit(_ context.Context, p string, content []byte) error {
	k, err := s.key(p)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(content))
	copy(cp, content)
	s.Files[k] = cp
	return nil
}

func (s *Stub) Shell(_ context.Context, argv []string) ([]byte, []byte, error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("workspace shell: empty argv")
	}
	if s.ShellErr != nil || s.ShellStdout != nil || s.ShellStderr != nil {
		return s.ShellStdout, s.ShellStderr, s.ShellErr
	}
	return nil, nil, fmt.Errorf("workspace stub: Shell not configured")
}
