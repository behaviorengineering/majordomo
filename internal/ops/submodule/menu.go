package submodule

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

const opsMenuTemplate = `Submodule Manager
-----------------
Submodule : {{.Submodule}}
Branch    : {{.Branch}}

{{range .Items}}{{.}}
{{end}}`

var parsedOpsMenuTemplate = template.Must(template.New("submodule-menu").Parse(opsMenuTemplate))

func (m *manager) buildOpsMenu(currentBranch string) string {
	data := struct {
		Submodule string
		Branch    string
		Items     []string
	}{
		Submodule: m.submoduleName,
		Branch:    currentBranch,
		Items: []string{
			"1. Update to latest (pull current branch)",
			"2. Switch to a different branch",
			"3. Pin to current commit",
			"q. Quit",
		},
	}
	var out bytes.Buffer
	if err := parsedOpsMenuTemplate.Execute(&out, data); err != nil {
		panic(fmt.Errorf("render submodule menu: %w", err))
	}
	return strings.TrimRight(out.String(), "\n")
}

func (m *manager) opsMenuLoop() error {
	for {
		currentBranch, err := m.currentBranch(m.submoduleRoot)
		if err != nil {
			return fmt.Errorf("read current submodule branch: %w", err)
		}
		m.printf("\n%s\n", m.buildOpsMenu(currentBranch))
		choice, err := m.readKey("Choice: ")
		if err != nil {
			return fmt.Errorf("read menu choice: %w", err)
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
			return fmt.Errorf("run submodule operation: %w", err)
		}
		if changed && m.parentRoot != "" {
			raw, err := m.prompt("\nPush to origin and exit? (y/N): ")
			if err != nil {
				return fmt.Errorf("read push confirmation: %w", err)
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
			return fmt.Errorf("read branch context choice: %w", err)
		}
		switch choice {
		case "q":
			return nil
		case "1":
			_, err := m.cmdUpdateViaWorktree()
			if err != nil {
				return fmt.Errorf("update through isolated worktree: %w", err)
			}
			return nil
		case "2":
			if err := m.opsMenuLoop(); err != nil {
				return fmt.Errorf("run direct submodule operations: %w", err)
			}
			return nil
		default:
			m.printf("Invalid choice — enter 1, 2, or q.\n")
		}
	}
}
