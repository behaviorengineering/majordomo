package modules

import (
	"github.com/XiaoConstantine/dspy-go/pkg/core"
	dspymodules "github.com/behaviorengineering/strop/pkg/dspy/modules"
)

func in(name, desc string) core.InputField {
	return core.InputField{Field: core.NewField(name, core.WithDescription(desc))}
}

func out(name, desc string) core.OutputField {
	return core.OutputField{Field: core.NewField(name, core.WithDescription(desc))}
}

func newGenerator(sig core.Signature, name string) *dspymodules.DirectivesCoT {
	return dspymodules.New(sig, dspymodules.Config{Name: name})
}

func fileReviewModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("file_path", "Repository-relative path under review"),
			in("slug", "Stable slug for the per-file report filename"),
			in("diff_content", "Staged diff or file content for review"),
			in("grounding", "Optional grounding markdown from agenting packs"),
		},
		[]core.OutputField{out("markdown", "Per-file review markdown with # header and - [SEVERITY] findings or 'No issues found.'")},
	).WithInstruction(`Review one changed file. Output markdown only.
Use exactly one H1 with the file path, then bullet findings as - [CRITICAL|WARN|INFO] text.
If there are no issues, write "No issues found." after the header.
Do not invent issues; only cite evidence from the diff.`)
	return newGenerator(sig, TaskFileReview)
}

func summaryModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("staging_context", "Summary staging context and diffs"),
		},
		[]core.OutputField{out("summary_md", "PR summary markdown matching Majordomo summary rubric")},
	).WithInstruction("Write a PR summary following Majordomo summary structure and rubric.")
	return newGenerator(sig, TaskSummary)
}

func technicalModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("staging_context", "Technical review staging context"),
		},
		[]core.OutputField{out("technical_md", "Technical review markdown")},
	).WithInstruction("Write a technical PR review following Majordomo tech rubric.")
	return newGenerator(sig, TaskTechnical)
}
