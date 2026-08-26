package submodule

import (
	"fmt"
	"strings"
)

func (m *manager) buildOpsMenu(currentBranch string) string {
	header := fmt.Sprintf(
		"Submodule Manager\n-----------------\nSubmodule : %s\nBranch    : %s",
		m.submoduleName, currentBranch,
	)
	items := []string{
		"1. Update to latest (pull current branch)",
		"2. Switch to a different branch",
		"3. Pin to current commit",
		"q. Quit",
	}
	return header + "\n\n" + strings.Join(items, "\n")
}

func (m *manager) opsMenuLoop() error {
	for {
		currentBranch, err := m.currentBranch(m.submoduleRoot)
		if err != nil {
			return err
		}
		m.printf("\n%s\n", m.buildOpsMenu(currentBranch))
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
	warning := fmt.Sprintf(
		"⚠️  OFF-BRANCH WARNING  ⚠️\nParent repo is on '%s', not '%s'.\nAny direct commits will land on '%s'.",
		currentParentBranch, pipelinesBranch, currentParentBranch,
	)
	m.printf("\n%s\n\n", warning)
	m.printf("1. 🔒 Safe  — update '%s' via isolated worktree\n", pipelinesBranch)
	m.printf("2. ⚡ Direct — I know what I'm doing (operate on '%s')\n", currentParentBranch)
	m.printf("q. Quit\n")
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
			m.printf("Invalid choice — enter 1, 2, or q.\n")
		}
	}
}
