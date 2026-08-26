package submodule

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (m *manager) pullWithRecovery(branch string) (string, bool, error) {
	out, err := m.git([]string{"pull", "origin", branch}, m.submoduleRoot, true)
	if err == nil {
		return out, true, nil
	}
	gitDir, gerr := m.gitDir(m.submoduleRoot)
	if gerr == nil {
		mergeHead := filepath.Join(gitDir, "MERGE_HEAD")
		if st, e := os.Stat(mergeHead); e == nil && !st.IsDir() {
			m.printf("Warning: pull left repo in a conflicted merge state — aborting.\n")
			_, _ = m.git([]string{"merge", "--abort"}, m.submoduleRoot, false)
		}
	}
	m.printf("Error: git pull failed — %v\n", err)
	raw, perr := m.prompt(fmt.Sprintf("Reset hard to 'origin/%s' (discards all local changes)? (y/N): ", branch))
	if perr != nil {
		return "", false, perr
	}
	if strings.ToLower(strings.TrimSpace(raw)) != "y" {
		m.printf("Cancelled — no changes made.\n")
		return "", false, nil
	}
	if _, err := m.git([]string{"fetch", "origin"}, m.submoduleRoot, true); err != nil {
		return "", false, err
	}
	if _, err := m.git([]string{"reset", "--hard", "origin/" + branch}, m.submoduleRoot, true); err != nil {
		return "", false, err
	}
	_, _ = m.git([]string{"clean", "-fd"}, m.submoduleRoot, true)
	return fmt.Sprintf("Reset to origin/%s.", branch), true, nil
}

func (m *manager) selectBranch(branches []string, current string) (string, error) {
	m.printf("\nCurrent branch: %s\n\nAvailable branches:\n", current)
	for i, branch := range branches {
		marker := ""
		if branch == current {
			marker = " *"
		}
		m.printf("  %2d. %s%s\n", i+1, branch, marker)
	}
	m.printf("\n")
	raw, err := m.prompt("Enter branch number (or 'q' to cancel): ")
	if err != nil {
		return "", err
	}
	raw = strings.TrimSpace(raw)
	if strings.ToLower(raw) == "q" {
		return "", nil
	}
	choice, err := strconv.Atoi(raw)
	if err != nil {
		m.printf("Invalid input — expected a number.\n")
		return "", nil
	}
	if choice < 1 || choice > len(branches) {
		m.printf("Invalid choice — enter a number between 1 and %d.\n", len(branches))
		return "", nil
	}
	selected := branches[choice-1]
	if selected == current {
		m.printf("Already on '%s' — nothing to do.\n", selected)
		return "", nil
	}
	return selected, nil
}

func (m *manager) cmdUpdate() (bool, error) {
	shaBefore, err := m.git([]string{"rev-parse", "HEAD"}, m.submoduleRoot, true)
	if err != nil {
		return false, err
	}
	current, err := m.currentBranch(m.submoduleRoot)
	if err != nil {
		return false, err
	}
	ok, err := m.confirmAndReset()
	if err != nil || !ok {
		return false, err
	}
	m.printf("Pulling latest on '%s' in '%s'...\n", current, m.submoduleName)
	out, cont, err := m.pullWithRecovery(current)
	if err != nil || !cont {
		return false, err
	}
	m.printf("%s\n", out)
	localSHA, err := m.git([]string{"rev-parse", "HEAD"}, m.submoduleRoot, true)
	if err != nil {
		return false, err
	}
	remoteSHA := m.remoteTrackingSHA(current)
	if remoteSHA != "" && localSHA != remoteSHA {
		msg := fmt.Sprintf(
			"Warning: local HEAD (%s) does not match origin/%s (%s).\n"+
				"The branch may have diverged (merge commit instead of fast-forward).\n"+
				"Reset hard to origin/%s to fix this?\n",
			localSHA[:min(7, len(localSHA))], current, remoteSHA[:min(7, len(remoteSHA))], current,
		)
		m.printf("%s", msg)
		raw, err := m.prompt("Fix now? (y/N): ")
		if err != nil {
			return false, err
		}
		if strings.ToLower(strings.TrimSpace(raw)) == "y" {
			_, _ = m.git([]string{"reset", "--hard", "origin/" + current}, m.submoduleRoot, true)
			_, _ = m.git([]string{"clean", "-fd"}, m.submoduleRoot, true)
			m.printf("Reset to origin/%s.\n", current)
			localSHA, _ = m.git([]string{"rev-parse", "HEAD"}, m.submoduleRoot, true)
		} else {
			m.printf("Skipped — submodule left at diverged state.\n")
		}
	}
	changed := localSHA != shaBefore
	if m.parentRoot != "" {
		if !m.isGitlinkInIndex(m.submoduleName) {
			m.printf("  ⚠️  Submodule not tracked on this branch — skipping parent pointer update.\n")
		} else {
			_, _ = m.git([]string{"add", m.submoduleName}, m.parentRoot, true)
			commitMsg := fmt.Sprintf("Update %s submodule to latest '%s'", m.submoduleName, current)
			commitOut, _ := m.git([]string{"commit", "-m", commitMsg}, m.parentRoot, false)
			if commitOut == "" {
				m.printf("Nothing to commit — submodule pointer already up to date.\n")
			} else {
				m.printf("%s\n", commitOut)
			}
		}
	}
	return changed, nil
}

func (m *manager) cmdSwitchBranch() (bool, error) {
	branches, err := m.remoteBranches()
	if err != nil {
		return false, err
	}
	if len(branches) == 0 {
		m.printf("No remote branches found.\n")
		return false, nil
	}
	current, err := m.currentBranch(m.submoduleRoot)
	if err != nil {
		return false, err
	}
	selected, err := m.selectBranch(branches, current)
	if err != nil || selected == "" {
		return false, err
	}
	m.printf("\nSwitching to '%s'...\n", selected)
	ok, err := m.confirmAndReset()
	if err != nil || !ok {
		return false, err
	}
	if gitDir, gerr := m.gitDir(m.submoduleRoot); gerr == nil {
		if st, e := os.Stat(filepath.Join(gitDir, "MERGE_HEAD")); e == nil && !st.IsDir() {
			m.printf("Warning: aborting in-progress merge before switching branch.\n")
			_, _ = m.git([]string{"merge", "--abort"}, m.submoduleRoot, false)
		}
	}
	if _, err := m.git([]string{"checkout", "-B", selected, "origin/" + selected}, m.submoduleRoot, true); err != nil {
		return false, fmt.Errorf("git checkout failed: %w", err)
	}
	m.printf("Resetting to 'origin/%s'...\n", selected)
	_, _ = m.git([]string{"reset", "--hard", "origin/" + selected}, m.submoduleRoot, true)
	_, _ = m.git([]string{"clean", "-fd"}, m.submoduleRoot, true)
	if m.parentRoot != "" {
		if !m.isGitlinkInIndex(m.submoduleName) {
			m.printf("  ⚠️  Submodule not tracked on this branch — skipping parent pointer update.\n")
		} else {
			_, _ = m.git([]string{"submodule", "set-branch", "--branch", selected, m.submoduleName}, m.parentRoot, true)
			_, _ = m.git([]string{"add", ".gitmodules", m.submoduleName}, m.parentRoot, true)
			commitMsg := fmt.Sprintf("Pin %s submodule to branch '%s'", m.submoduleName, selected)
			commitOut, _ := m.git([]string{"commit", "-m", commitMsg}, m.parentRoot, false)
			if commitOut == "" {
				m.printf("Nothing to commit.\n")
			} else {
				m.printf("%s\n", commitOut)
			}
		}
	}
	m.printf("\nSubmodule '%s' is now on '%s'.\n", m.submoduleName, selected)
	return true, nil
}

func (m *manager) cmdPinCommit() (bool, error) {
	sha, err := m.currentSHA(m.submoduleRoot)
	if err != nil {
		return false, err
	}
	branch, err := m.currentBranch(m.submoduleRoot)
	if err != nil {
		return false, err
	}
	m.printf("Pinning '%s' to %s (branch: %s)...\n", m.submoduleName, sha, branch)
	if m.parentRoot == "" {
		return false, nil
	}
	if !m.isGitlinkInIndex(m.submoduleName) {
		m.printf("  ⚠️  Submodule not tracked on this branch — skipping parent pointer update.\n")
		return false, nil
	}
	_, _ = m.git([]string{"add", m.submoduleName}, m.parentRoot, true)
	commitMsg := fmt.Sprintf("Pin %s submodule to commit %s", m.submoduleName, sha)
	commitOut, _ := m.git([]string{"commit", "-m", commitMsg}, m.parentRoot, false)
	if commitOut == "" {
		m.printf("Nothing to commit — submodule pointer already up to date.\n")
	} else {
		m.printf("%s\n", commitOut)
	}
	return true, nil
}

func (m *manager) cmdUpdateViaWorktree() (bool, error) {
	sha, err := m.git([]string{"rev-parse", "HEAD"}, m.submoduleRoot, true)
	if err != nil {
		return false, err
	}
	branch, err := m.currentBranch(m.submoduleRoot)
	if err != nil {
		return false, err
	}
	shortSHA := sha
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	m.printf("Fetching remote to verify '%s' branch exists...\n", pipelinesBranch)
	if _, err := m.git([]string{"fetch", "origin"}, m.parentRoot, true); err != nil {
		return false, fmt.Errorf("git fetch failed: %w", err)
	}
	remoteRef, _ := m.git([]string{"rev-parse", "--verify", "origin/" + pipelinesBranch}, m.parentRoot, false)
	if remoteRef == "" {
		m.printf("Error: '%s' branch does not exist on remote.\nCreate it first, then re-run this option.\n", pipelinesBranch)
		return false, nil
	}
	worktreePath := filepath.Join(m.parentRoot, worktreeDir)
	if st, e := os.Stat(worktreePath); e == nil && st.IsDir() {
		m.printf("Removing stale worktree at '%s'...\n", worktreeDir)
		_, _ = m.git([]string{"worktree", "remove", "--force", worktreePath}, m.parentRoot, false)
	}
	m.printf("Creating isolated worktree for '%s'...\n", pipelinesBranch)
	if _, err := m.git([]string{"worktree", "add", "--detach", worktreePath, "origin/" + pipelinesBranch}, m.parentRoot, true); err != nil {
		return false, fmt.Errorf("git worktree add failed: %w", err)
	}
	committed := false
	defer func() {
		m.printf("Cleaning up worktree...\n")
		_, _ = m.git([]string{"worktree", "remove", "--force", worktreePath}, m.parentRoot, false)
	}()
	cacheInfo := fmt.Sprintf("160000,%s,%s", sha, m.submoduleName)
	if _, err := m.git([]string{"update-index", "--cacheinfo", cacheInfo}, worktreePath, true); err != nil {
		return false, err
	}
	commitMsg := fmt.Sprintf("Update %s to %s (branch: %s)", m.submoduleName, shortSHA, branch)
	commitOut, _ := m.git([]string{"commit", "-m", commitMsg}, worktreePath, false)
	if commitOut == "" {
		m.printf("Nothing to commit — submodule pointer already up to date.\n")
	} else {
		m.printf("%s\n", commitOut)
		committed = true
		m.printf("Pushing '%s' to origin...\n", pipelinesBranch)
		pushOut, err := m.git([]string{"push", "origin", "HEAD:"+pipelinesBranch}, worktreePath, true)
		if err != nil {
			return false, err
		}
		if pushOut == "" {
			m.printf("Pushed '%s' to origin.\n", pipelinesBranch)
		} else {
			m.printf("%s\n", pushOut)
		}
	}
	return committed, nil
}

func (m *manager) pushToOrigin() error {
	branch, err := m.currentBranch(m.parentRoot)
	if err != nil {
		return err
	}
	m.printf("Pushing '%s' to origin...\n", branch)
	out, err := m.git([]string{"push", "origin", branch}, m.parentRoot, true)
	if err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	if out == "" {
		m.printf("Pushed '%s' to origin.\n", branch)
	} else {
		m.printf("%s\n", out)
	}
	return nil
}
