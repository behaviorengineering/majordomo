package submodule

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

var (
	opsMenuTemplate = template.Must(template.New("ops-menu").Parse(`Submodule Manager
-----------------
Submodule : {{.Submodule}}
Branch    : {{.Branch}}

{{range .Items}}{{.}}
{{end}}`))
	offBranchTemplate = template.Must(template.New("off-branch").Parse(`OFF-BRANCH WARNING
Parent repo is on '{{.ParentBranch}}', not '{{.PipelinesBranch}}'.
Any direct commits will land on '{{.ParentBranch}}'.

1. Safe: update '{{.PipelinesBranch}}' via isolated worktree
2. Direct: I know what I am doing (operate on '{{.ParentBranch}}')
q. Quit
`))
)

func (m *manager) buildOpsMenu(currentBranch string) (string, error) {
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
	var output bytes.Buffer
	if err := opsMenuTemplate.Execute(&output, data); err != nil {
		return "", err
	}
	return output.String(), nil
}

func (m *manager) opsMenuLoop() error {
	for {
		currentBranch, err := m.currentBranch(m.submoduleRoot)
		if err != nil {
			return err
		}
		menu, err := m.buildOpsMenu(currentBranch)
		if err != nil {
			return fmt.Errorf("render submodule menu: %w", err)
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
	var output bytes.Buffer
	if err := offBranchTemplate.Execute(&output, struct {
		ParentBranch    string
		PipelinesBranch string
	}{
		ParentBranch:    currentParentBranch,
		PipelinesBranch: pipelinesBranch,
	}); err != nil {
		return fmt.Errorf("render off-branch warning: %w", err)
	}
	m.printf("\n%s\n", output.String())
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
			m.printf("Invalid choice: enter 1, 2, or q.\n")
		}
	}
}
