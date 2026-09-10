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

// consultantCounselContract is the shared voice for unattended Typology digest prose
// (cluster proposal, refine journey, human-intervention / PR priority). Digest cannot wait
// for a human pick, so the lean is the recommendation a human can override at merge.
const consultantCounselContract = `
Consultant counsel (MUST apply to every recommendation and open debt item):
- The speaker is Majordomo (and its Typology digest processes), not a product team. Attribute proposals to Majordomo or Typology digest.
- Catalog merges, libraries, and bindings on the context branch are architecture-grounding proposals awaiting human merge or reject. They are not consented reorganizations already landed in the product repository.
- Evidenced slice-to-library SliceBindings complete a library classification in this proposal (natural when a consumer imports a library-owned package). They remain proposals until the context PR merges.
- Lead with a recommendation in fluent prose.
- When there is a real fork (different catalog or ownership outcome), name two shippable approaches in product terms, one cost each, then the lean.
- Keep package and slice ids so coverage checks still match; gloss jargon in the same sentence.
- MUST NOT invent decoy options or offer "do nothing" as the second approach.
- MUST NOT defer the argument to another file (for example "see journey_md" or "please review the full list").
- MUST NOT stop at hollow mitigations such as "Approve binding or refactor" with no smell, alternatives, or lean.
- MUST NOT mansplain: no "Hello Operator," no inventory restatement without a lean.
- MUST NOT use corporate "we" for shipped-sounding claims (for example "we successfully reorganized the repository", "we consolidated", "we are proceeding"). Prefer "Majordomo proposes", "Typology refine grouped", or "this context catalog proposes".
- A recommended lean is guidance only. MUST NOT invent libraries membership solely to clear findings. MUST NOT invent slice-to-slice bindings without draft or graph evidence.`

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
			in("package_contracts", "Per-package public contracts from typology contracts"),
			in("package_roles", "Observed package role topology YAML: role, confidence, evidence, labeled edges. Folder names are not evidence."),
			in("mechanical_grouping_md", "Deterministic grouping seed derived from roles and graph."),
			in("architecture_draft", "Architecture brief for the raw draft"),
			in("repo_layout", "Top-level layout names"),
			in("readme_snapshot", "Served-repo README: product purpose and delivery commands"),
			in("validation_feedback", "Optional prior structure-validation feedback to fix"),
		},
		[]core.OutputField{
			out("cluster_proposal_md", "Markdown proposal of merges, renames, anti-pattern findings, and boundary debt"),
		},
	).WithInstruction(`You are the unattended Typology cluster-pass for Majordomo context digest.
A discover draft is package-level inventory. package_roles is the factual observed topology. Clustering is an optional overlay and MUST NOT contradict package_roles.

Order of evidence (MUST):
1. package_roles: each package already has role + confidence + evidence from code (entrypoint, server, dto, exec_runner, aggregator, adapter, config, observability, unknown).
2. mechanical_grouping_md: deterministic grouping seed. It is authoritative for the seed groups, and the LLM must not silently override it.
3. package_contracts and readme_snapshot: supporting facts.
4. graph_text: coupling and wiring only. Labeled edges in package_roles (fills_dto, uses_runner, serves_server, composes, reads_config) explain imports.
5. Folder and path words (dashboard, board, cli, server) are NEVER evidence and MUST NOT relabel a node.

Hard rules from observed roles:
- entrypoint packages are CLI surfaces.
- server packages are HTTP, gRPC, or UI surfaces. MUST NOT share a slice with an entrypoint.
- dto packages are shared data contracts; MUST NOT merge them into an aggregator or call them the product domain.
- aggregator packages build page/domain data; MUST NOT label them kind: ui or "the website".
- exec_runner packages are technical runners; MUST NOT put them under the entrypoint's domain just because the entrypoint also imports them.
- observability packages boot tracing/metrics; they are not config.
- fills_dto edges mean adapters fill JSON types; they are NOT "forge depends on the UI".
- uses_runner edges mean a package shells out through a runner; they are NOT "depends on the CLI domain".

Grouping is optional and only when both sides are high-confidence and an evidenced import exists. Prefer recording wiring notes over inventing ownership.
` + consultantCounselContract + `

Apply these merge heuristics only after honoring package_roles:
1. Same job family companions (for example two forge adapters) may share a slice when both are adapters.
2. Split companion packages that share a stem (sa + satools -> sa).
3. Sole importer: wiring note only; never merge-into-caller against observed roles.

Enforce anti-patterns:
- Do not promote capabilities to domain pillars.
- Do not create standalone cli or platform slices.
- Do not treat exec_runner as kind: cli.
- Every draft package path must remain claimed under owns[], surfaces[], or libraries[].owns[].

Declare domain-free utilities under libraries[] when the draft or graph shows them.
Record boundary debt with smell, alternatives, and lean. MUST NOT use hollow mitigations such as "Approve binding or refactor".
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
			in("package_contracts", "Per-package public contracts from typology contracts"),
			in("package_roles", "Observed package role topology YAML: role, confidence, evidence, labeled edges"),
			in("architecture_draft", "Architecture brief for the raw draft"),
			in("repo_layout", "Top-level layout names"),
			in("readme_snapshot", "Served-repo README: product purpose and delivery commands"),
			in("validation_feedback", "Optional ValidateStructure or boundary-evaluator feedback to fix"),
		},
		[]core.OutputField{
			out("refined_catalog_yaml", "Full refined Typology catalog YAML proposal"),
			out("journey_md", "Compressed journey notes including decisions and boundary debt table"),
		},
	).WithInstruction(`You are the unattended Typology refine step for Majordomo context digest.
Apply the cluster proposal to the draft catalog and emit a complete refined typology.yaml.
package_roles is factual. MUST NOT contradict it. Folder names are never evidence.

Placement from roles:
- entrypoint -> kind: cli surfaces
- server -> surfaces (api, grpc, or ui when evidence includes embeds_static); NEVER fold into the CLI slice for sole importer; NEVER share a slice with an entrypoint
- dto -> owns[] (or a thin shared data slice); NEVER the product domain from graph position; NEVER owned by an aggregator just because that aggregator imports it
- aggregator -> owns[] of a product slice; MUST NOT kind: ui
- exec_runner -> owns[] or libraries[]; NEVER under the entrypoint domain solely because cmd imports it
- observability -> owns[] or libraries[]; NEVER config
- adapter / config -> owns[] or libraries[] as fits

Catalog MAY group companion adapters when both are high-confidence. MUST NOT invent "forge depends on UI" or "localgit depends on CLI" smells from false ownership.
` + consultantCounselContract + `

Catalog rules:
- Every slice MUST have a non-empty business objective that states why the bounded context exists in one concrete sentence.
- MUST NOT use hollow template objectives such as "Provide X functionality", "Provide X capabilities", or "Provide X services".
- Components are packages under owns, under surfaces, or under libraries[].owns.
- Libraries are technical package groups with a purpose and owns[] only.
- MUST NOT invent libraries[] membership solely to clear findings.
- When libraries[] claim packages, emit evidenced SliceBinding entries from consumer slices.
- Surfaces are ui, cli, or api interaction artefacts for user-facing delivery based on observed roles, not path words.
- Every draft package path MUST appear under owns[], surfaces[], or libraries[].owns[].
- Preserve real package paths from the draft and graph verbatim.
- MUST NOT invent filesystem package folders. Put desired renames in journey debt only.
- Journey markdown MUST include Status, decisions taken, and a Technical debt and boundary violations table.
- Each decision MUST say what was rejected and why.
- Each open debt row MUST carry smell, alternatives, and lean.
- When validation_feedback is present, fix those issues before emitting.

Output refined_catalog_yaml as YAML only (no markdown fences). Output journey_md as markdown.`)
	return newGenerator(sig, TaskTypologyRefine)
}

func typologyInspectModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("package_path", "Repository-relative package path being inspected"),
			in("package_contracts", "Contract row for this package if available"),
			in("package_source", "Go source files for this package only"),
			in("candidate_role", "Optional mechanical candidate role"),
			in("current_evidence", "Mechanical evidence ids already known"),
		},
		[]core.OutputField{
			out("role", "One of: entrypoint, server, dto, exec_runner, aggregator, adapter, config, observability, unknown"),
			out("evidence", "Short symbol-based evidence quotes; never the directory name"),
		},
	).WithInstruction(`Classify one Go package into an observed role from its source and contracts only.
Allowed roles: entrypoint, server, dto, exec_runner, aggregator, adapter, config, observability, unknown.
MUST NOT use the directory or folder name as evidence (ignore words like dashboard, board, cli, server in the path).
	Cite exported symbols and import paths only (os/exec, net/http, google.golang.org/grpc, go.opentelemetry.io, gopkg.in/yaml.v3, go:embed, JSON/YAML tags, ServeHTTP, Register*Server).
MUST NOT invent a role from English function names. If unsure, return role unknown.`)
	return newGenerator(sig, TaskTypologyInspect)
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
			out("human_intervention_md", "Tutor-voice operator briefing of priority decisions humans must make"),
			out("weaknesses_seed_md", "Weaknesses markdown bullets for bootstrap story seeding"),
			out("pr_priority_md", "Context PR summary markdown in tutor voice for a cold reader"),
		},
	).WithInstruction(`You are the Majordomo human-intervention flagger after Typology refine.
Humans give direction and leadership. Your job is to surface architecture findings the unattended refine must NOT invent away.
` + consultantCounselContract + `

Rules:
- findings_list is authoritative. Every finding MUST appear in journey debt, human_intervention_md, weaknesses_seed_md, and pr_priority_md.
- Output markdown only. MUST NOT invent sliceBindings, libraries[] rows, rewrite package ownership, or invent catalog YAML in these fields.
- Majordomo completes evidenced slice-to-library SliceBindings in the proposal catalog when it classifies packages as libraries. MUST NOT ask humans to rubber-stamp those mechanical edges.
- Frame remaining findings as normative human decisions: keep the library placement, fold packages into a domain slice, decouple the import, approve a slice-to-slice binding, merge slices, or accept temporary debt.
- Missing slice-to-slice bindings still need counsel. Missing slice-to-library bindings after refine should be rare; if one remains, argue library placement vs decouple, not "approve this binding".
- When findings_list is non-empty, journey Status MUST stay open (not complete/completed).
- Journey MUST include Status, decisions already taken, and a Technical debt and boundary violations table that names every finding.
- Journey rewrite MUST keep leans from refine counsel. MUST NOT flatten debt rows back to a table of verbs such as "Approve binding or refactor".
- human_intervention_md is a tutor briefing for operators: situation, smell/risk, alternatives with a cost, recommended lean, and evidence pointers into architecture/journey. MUST NOT punt the argument to journey_md.
- weaknesses_seed_md is markdown bullets suitable for weaknesses.md (same priorities and leans, shorter).
- pr_priority_md is the GitHub context-PR summary a cold reader sees first. Write it in a tutor voice with consultant counsel. Assume the reader has never seen this repo. Lead with what Majordomo's Typology digest is proposing on this context branch and why it matters, then smell, alternatives, and the lean. Frame slice consolidations and library placements as proposed catalog models for grounding, not as work the product team already shipped. Gloss jargon in the same sentence (a SliceBinding is an approved allowed coupling from a bounded context to another slice or to a library; a library is a technical package group with no product objective). Name packages by role (UI, git adapters, CLI runner, shared config) as well as id so coverage checks still match. MUST NOT use only imperative task titles such as "Formalize Config Access". MUST NOT dump catalog ids without a gloss. MUST NOT say "see journey_md" or ask the reader to open another file for the real argument. MUST NOT imply humans already consented to a repo reorganization.
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
