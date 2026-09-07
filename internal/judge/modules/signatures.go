package modules

import (
	"github.com/XiaoConstantine/dspy-go/pkg/core"
	dspymodules "github.com/behaviorengineering/strop/dspy/modules"
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

func digestStoryModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("section_id", "Story section id (mission, architecture, conventions, weaknesses)"),
			in("current_text", "Current section markdown"),
			in("commit_subject", "First-parent commit subject"),
			in("commit_diff", "Capped commit diff"),
			in("changed_files", "Comma-separated changed paths"),
			in("regen_feedback", "Optional gate reject feedback to address"),
		},
		[]core.OutputField{out("updated_text", "Updated section markdown; preserve structure; only add evidenced claims")},
	).WithInstruction(`Update one teaching-story section after a default-branch commit.
Amend only when the commit diff supports a concrete claim. If nothing applies, return current_text unchanged.
Never invent architecture or risks. Chronology is handled separately.`)
	return newGenerator(sig, TaskDigestStory)
}

func bootstrapStoryModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("source_sha", "Default branch HEAD used as the bootstrap source"),
			in("generated_at", "RFC3339 bootstrap time"),
			in("evidence_mode", "Survey mode, such as discover, reuse, or fallback"),
			in("module_scope", "Typology module scope, when available"),
			in("readme_snapshot", "Current README snapshot from the served repo"),
			in("typology_manifest", "Typology evidence manifest YAML"),
			in("typology_architecture", "Post-refine Typology architecture brief or fallback architecture survey"),
			in("typology_refined_catalog", "Refined Typology catalog YAML proposal, when available"),
			in("typology_journey", "Compressed typology journey notes and boundary debt, when available"),
			in("repo_layout", "Top-level repo layout and notable evidence files"),
			in("current_readme", "Current bootstrap README placeholder"),
			in("current_mission", "Current bootstrap mission placeholder"),
			in("current_architecture", "Current bootstrap architecture placeholder"),
			in("current_conventions", "Current bootstrap conventions placeholder"),
			in("current_weaknesses", "Current bootstrap weaknesses placeholder"),
			in("current_chronology", "Current bootstrap chronology placeholder"),
			in("current_grounding", "Current bootstrap agenting grounding placeholder"),
			in("validation_feedback", "Optional evaluator feedback to fix on retry"),
		},
		[]core.OutputField{
			out("readme_md", "Updated context-branch README markdown"),
			out("mission_md", "Updated mission markdown"),
			out("architecture_md", "Updated architecture markdown"),
			out("conventions_md", "Updated conventions markdown"),
			out("weaknesses_md", "Updated weaknesses markdown"),
			out("chronology_md", "Updated chronology markdown"),
			out("grounding_md", "Updated agenting grounding markdown"),
		},
	).WithInstruction(`Seed the context branch from current evidence only.
Write all outputs as present-tense, user-facing markdown.
Prefer the refined Typology catalog and journey notes over raw package inventory when they are present.
The README should describe the context branch and its seed origin.
Mission, architecture, conventions, and weaknesses must be evidence-backed and should not mention historical events that are not in the supplied evidence.
Architecture should describe proposed bounded contexts (slices), surfaces, and known boundary debt from the journey notes.
Root architecture_md is the teaching story for humans and review grounding. Keep the Typology evidence brief (typology_architecture input) as source material; do not pretend it is the confirmed catalog.
Do not copy hollow template slice objectives; paraphrase into concrete teaching language grounded in the catalog and README.
Chronology must stay honest, with at most a single explicit seed marker. Do not reconstruct past decisions.
The grounding output should summarize the accepted mission and architecture for agenting.
If evidence is thin, keep the section minimal rather than inventing details.
When validation_feedback is present, fix those issues before emitting.
Preserve each file's markdown shape and heading conventions.`)
	return newGenerator(sig, TaskBootstrapStory)
}

// Prompt sources (human editors): typology skills/journey/SKILL.md cluster-pass + anti-patterns;
// typology skills/catalog/SKILL.md objectives and surfaces.
func typologyClusterModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("module_scope", "Typology module scope"),
			in("draft_catalog_yaml", "Raw Typology discover draft YAML"),
			in("graph_text", "typology show graph output"),
			in("package_contracts", "Per-package public contracts: exports and hasMain from typology contracts"),
			in("architecture_draft", "Architecture brief for the raw draft"),
			in("repo_layout", "Top-level layout names"),
			in("validation_feedback", "Optional prior structure-validation feedback to fix"),
		},
		[]core.OutputField{
			out("cluster_proposal_md", "Markdown proposal of merges, renames, anti-pattern findings, and boundary debt"),
		},
	).WithInstruction(`You are the unattended Typology cluster-pass for Majordomo context digest.
A discover draft is package-level inventory, not architecture. Propose how to consolidate it into bounded contexts.
Use package_contracts as primary evidence for what each package is: hasMain or cmd delivery packages are CLI surfaces; libraries that only export helpers or exec runners are not CLI surfaces even if the path contains "cli".

Apply these merge heuristics:
1. Sole importer: package imported by only one caller -> merge into caller.
2. Same job family: companion packages around one domain concern -> one family slice.
3. Split companion packages that share a stem (sa + satools -> sa).
4. Manifest/CLI companions used only for staging -> staging/domain owner.
5. Forge side-effects next to publish -> publish or forge.
6. Projection/visualizer packages that only render domain models -> surfaces of the entity domain, not peer slices.

Enforce DDD anti-patterns:
- Do not make temporal pipeline stages peer slices.
- Do not promote capabilities (LLM gateway, eval) to domain pillars.
- Do not create standalone cli or platform slices; put CLI under surfaces of the domain they invoke.
- Do not create projection-as-slice peers.
- Do not treat exec-adapter packages (for example cliexec with Run/Command exports and hasMain false) as CLI surfaces.
- Every draft package path must remain claimed under owns[] or surfaces[]; demoting an exec adapter off kind: cli must keep it under owns[], never drop it.

Keep small platform leaves (config, telemetry, auth) separate.
Record boundary violations and open debt in a debt table.
Output markdown only with sections: Proposed merges, Proposed renames, Anti-pattern findings, Boundary debt, Rationale.
Do not emit catalog YAML in this step.`)
	return newGenerator(sig, TaskTypologyCluster)
}

func typologyRefineModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("module_scope", "Typology module scope"),
			in("draft_catalog_yaml", "Raw Typology discover draft YAML"),
			in("cluster_proposal_md", "Approved cluster-pass proposal markdown"),
			in("package_contracts", "Per-package public contracts: exports and hasMain from typology contracts"),
			in("architecture_draft", "Architecture brief for the raw draft"),
			in("repo_layout", "Top-level layout names"),
			in("validation_feedback", "Optional ValidateStructure or boundary-evaluator feedback to fix"),
		},
		[]core.OutputField{
			out("refined_catalog_yaml", "Full refined Typology catalog YAML proposal"),
			out("journey_md", "Compressed journey notes including decisions and boundary debt table"),
		},
	).WithInstruction(`You are the unattended Typology refine step for Majordomo context digest.
Apply the cluster proposal to the draft catalog and emit a complete refined typology.yaml.
Use package_contracts when placing packages: hasMain or real cmd/ delivery packages belong under kind: cli surfaces; packages that only export library/exec helpers (hasMain false) belong under owns[], never kind: cli solely because the path contains "cli".

Catalog rules:
- Every slice MUST have a non-empty business objective that states why the bounded context exists in one concrete sentence (for example who it serves and what outcome it owns).
- MUST NOT use hollow template objectives such as "Provide X functionality", "Provide X capabilities", or "Provide X services".
- Components are packages under owns or under surfaces.
- Surfaces are ui, cli, or api interaction artefacts for user-facing delivery.
- Packages under cmd/, http/api, ui, dashboard, or server MUST sit under surfaces[], not domain-only owns[].
- Process-exec adapters such as cliexec are infrastructure under owns[], NEVER kind: cli surfaces.
- Every draft package path MUST appear under owns[] or surfaces[] in the refined catalog. Demoting an exec adapter off kind: cli MUST keep that package under owns[]; MUST NOT drop it.
- Blank programs are allowed for pure domain slices; do not invent subprograms or actuators without path evidence.
- Do not emit docs or docs.pages; DocPages are for later human emit, not this proposal.
- Do not invent history; this is a proposal for the context branch, not a confirmed served-repo catalog.
- Preserve real package paths from the draft and graph verbatim in every component path: field (with or without ./ is fine).
- MUST NOT invent or rewrite filesystem package folders (for example localgit -> git/local). Put desired folder renames in the journey debt table only.
- Slice ids MAY rename or merge; package path: values MUST still match draft/graph paths.
- Include sliceBindings/componentBindings only when evidenced by the draft or graph narrative in the proposal.
- Journey markdown MUST include Status, decisions taken, and a Technical debt and boundary violations table.
- Journey debt rows describe remaining open work only. If Status says refinement is complete, do not keep "Merge into" as a pending action.
- If the architecture draft still lists findings, the debt table MUST have at least one concrete row (not "None").
- When validation_feedback is present, fix those issues before emitting.

Output refined_catalog_yaml as YAML only (no markdown fences). Output journey_md as markdown.`)
	return newGenerator(sig, TaskTypologyRefine)
}

func typologyHumanInterventionModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("architecture_md", "Post-refine Typology architecture brief"),
			in("refined_catalog_yaml", "Refined Typology catalog YAML"),
			in("journey_md", "Journey notes from typology refine"),
			in("cluster_proposal_md", "Cluster-pass proposal markdown"),
			in("findings_list", "Deterministic list of open architecture findings; each must be flagged for humans"),
			in("validation_feedback", "Optional prior validation feedback to fix"),
		},
		[]core.OutputField{
			out("journey_md", "Updated journey with open Status and debt covering every finding"),
			out("human_intervention_md", "Operator-facing priority decisions humans must make"),
			out("weaknesses_seed_md", "Weaknesses markdown bullets for bootstrap story seeding"),
			out("pr_priority_md", "Short priority bullets for the context PR body"),
		},
	).WithInstruction(`You are the Majordomo human-intervention flagger after Typology refine.
Humans give direction and leadership. Your job is to surface architecture findings the unattended refine must NOT invent away.

Rules:
- findings_list is authoritative. Every finding MUST appear in journey debt, human_intervention_md, weaknesses_seed_md, and pr_priority_md.
- MUST NOT invent sliceBindings, rewrite package ownership, or invent catalog YAML to clear findings.
- Frame each finding as a human decision: approve a binding, merge slices, or accept temporary debt.
- When findings_list is non-empty, journey Status MUST stay open (not complete/completed).
- Journey MUST include Status, decisions already taken, and a Technical debt and boundary violations table that names every finding.
- human_intervention_md is markdown for operators: priorities, what not to invent, evidence pointers into architecture/journey.
- weaknesses_seed_md is markdown bullets suitable for weaknesses.md (same priorities).
- pr_priority_md is a short scannable bullet list for the GitHub PR body (no long preamble).
- When findings_list is empty, say no open architecture findings and keep Status coherent with an empty/open debt note.
- When validation_feedback is present, fix those issues before emitting.

Output markdown only in the four fields (no YAML catalog).`)
	return newGenerator(sig, TaskTypologyHumanIntervention)
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
