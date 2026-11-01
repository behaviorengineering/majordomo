package submodule

import (
	"bytes"
	"strings"
	"text/template"
)

func (m *manager) buildOpsMenu(currentBranch string) (string, error) {
	const report = `Submodule Manager
-----------------
Submodule : {{.Submodule}}
Branch    : {{.Branch}}

1. Update to latest (pull current branch)
2. Switch to a different branch
3. Pin to current commit
q. Quit`
	tpl, err := template.New("submodule-menu").Parse(report)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, map[string]string{
		"Submodule": m.submoduleName,
		"Branch":    currentBranch,
	}); err != nil {
		return "", err
	}
	return out.String(), nil
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
	const warningTemplate = `⚠️  OFF-BRANCH WARNING  ⚠️
Parent repo is on '{{.Current}}', not '{{.Pipelines}}'.
Any direct commits will land on '{{.Current}}'.`
	tpl, err := template.New("off-branch-warning").Parse(warningTemplate)
	if err != nil {
		return err
	}
	var warningOut bytes.Buffer
	if err := tpl.Execute(&warningOut, map[string]string{
		"Current":   currentParentBranch,
		"Pipelines": pipelinesBranch,
	}); err != nil {
		return err
	}
	warning := warningOut.String()
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
