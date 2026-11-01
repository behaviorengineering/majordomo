package submodule

import (
	"fmt"
	"strings"
	"text/template"
)

var (
	opsMenuTemplate = template.Must(template.New("submodule-ops-menu").Parse(
		"Submodule Manager\n-----------------\nSubmodule : {{.SubmoduleName}}\nBranch    : {{.Branch}}",
	))
	offBranchTemplate = template.Must(template.New("submodule-off-branch").Parse(
		"OFF-BRANCH WARNING\nParent repo is on '{{.ParentBranch}}', not '{{.PipelinesBranch}}'.\nAny direct commits will land on '{{.ParentBranch}}'.",
	))
)

func (m *manager) buildOpsMenu(currentBranch string) (string, error) {
	var header strings.Builder
	if err := opsMenuTemplate.Execute(&header, struct {
		SubmoduleName string
		Branch        string
	}{m.submoduleName, currentBranch}); err != nil {
		return "", fmt.Errorf("render submodule menu: %w", err)
	}
	items := []string{
		"1. Update to latest (pull current branch)",
		"2. Switch to a different branch",
		"3. Pin to current commit",
		"q. Quit",
	}
	return header.String() + "\n\n" + strings.Join(items, "\n"), nil
}

func (m *manager) opsMenuLoop() error {
	for {
		currentBranch, err := m.currentBranch(m.submoduleRoot)
		if err != nil {
			return err
		}
		menu, err := m.buildOpsMenu(currentBranch)
		if err != nil {
			return err
		}
		m.printf("\n%s\n", menu)
		choice, err := m.readKey("Choice: ")
		if err != nil {
			return err
		}
		var changed bool
		switch choice {
		case "q":
			return nil
		case "1":
			changed, err = m.cmdUpdate()
		case "2":
			changed, err = m.cmdSwitchBranch()
		case "3":
			changed, err = m.cmdPinCommit()
		default:
			m.printf("Invalid choice.\n")
			continue
		}
		if err != nil {
			return err
		}
		if changed && m.parentRoot != "" {
			raw, err := m.prompt("\nPush to origin and exit? (y/N): ")
			if err != nil {
				return err
			}
			if strings.ToLower(strings.TrimSpace(raw)) == "y" {
				return m.pushToOrigin()
			}
		}
	}
}

func (m *manager) promptOffBranchContext(currentParentBranch string) error {
	var warning strings.Builder
	if err := offBranchTemplate.Execute(&warning, struct {
		ParentBranch    string
		PipelinesBranch string
	}{currentParentBranch, pipelinesBranch}); err != nil {
		return fmt.Errorf("render branch warning: %w", err)
	}
	m.printf("\n%s\n\n", warning.String())
	m.printf("1. Safe, update '%s' via isolated worktree.\n", pipelinesBranch)
	m.printf("2. Direct, operate on '%s'.\n", currentParentBranch)
	m.printf("q. Quit.\n")
	for {
		choice, err := m.readKey("\nContext: ")
		if err != nil {
			return err
		}
		switch choice {
		case "q":
			return nil
		case "1":
			_, err := m.cmdUpdateViaWorktree()
			return err
		case "2":
			return m.opsMenuLoop()
		default:
			m.printf("Invalid choice, enter 1, 2, or q.\n")
		}
	}
}
