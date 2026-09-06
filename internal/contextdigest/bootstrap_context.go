package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func bootstrapContextBranch(ctxDir string, opts Options, repoID, sourceSHA string, at time.Time) error {
	policy := strings.ToLower(strings.TrimSpace(opts.BootstrapSurveyPolicy))
	if policy == "never" {
		return nil
	}
	runner := opts.BootstrapSurveyRunner
	if runner == nil {
		runner = LocalBootstrapSurveyRunner{}
	}

	analysisDir, err := cloneAnalysisRepo(context.Background(), opts.WorkDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(analysisDir)

	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	input := BootstrapSurveyInput{
		AnalysisDir:    analysisDir,
		EvidenceDir:    evidenceDir,
		SourceSHA:      sourceSHA,
		RepoID:         repoID,
		TypologyBinary: opts.TypologyBinary,
		ModuleScope:    opts.ModuleScope,
		GeneratedAt:    at,
	}
	if err := runner.Survey(context.Background(), input); err != nil {
		return err
	}
	if err := refineTypologyEvidence(context.Background(), opts, analysisDir, evidenceDir, opts.TypologyRefineGenerator, opts.Judge); err != nil {
		return err
	}
	if err := writeBootstrapStory(ctxDir, analysisDir, at, sourceSHA, opts.BootstrapStoryGenerator, opts.Judge); err != nil {
		return err
	}
	ctxGit := &Git{Dir: ctxDir}
	if err := configureCommitIdentity(ctxGit); err != nil {
		return err
	}
	msg := fmt.Sprintf("context bootstrap: seed snapshot at %s", shortSHA(sourceSHA))
	if committed, err := CommitAll(ctxGit, msg); err != nil {
		return err
	} else if !committed {
		return fmt.Errorf("bootstrap survey produced no changes to commit")
	}
	return contextstore.ValidateTree(ctxDir)
}
