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
Never invent architecture or risks. Chronology is handled separately.
Preserve <!-- majordomo-reading-nav:start --> / <!-- majordomo-reading-nav:end --> banners when present.`)
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
			in("slice_meaning_ledger", "Evidence-first slice meaning ledger YAML when catalog assemble ran"),
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
	).WithInstruction(`Seed the context branch for the served repository (repo_id), not Majordomo the control plane (unless repo_id is majordomo).
Write all outputs as present-tense, user-facing markdown about that product.
Prefer the slice objective ledger and refined Typology catalog over raw package inventory when they are present.
The README should describe the context branch and its seed origin for the served repo.
README MUST keep a ## Reading order section (story path then evidence/typology). Preserve <!-- majordomo-reading-toc:start --> / <!-- majordomo-reading-toc:end --> and <!-- majordomo-reading-nav:start --> / <!-- majordomo-reading-nav:end --> blocks when present; digest re-applies them if dropped.
Mission, architecture, conventions, and weaknesses must be evidence-backed and should not mention historical events that are not in the supplied evidence.
Mission, architecture, and grounding MUST name the served product from evidence; MUST NOT describe Majordomo triage, digest, or context-branch process as the product.
Architecture should describe proposed bounded contexts (slices), surfaces, and known boundary debt from the journey notes and ledger.
Root architecture_md is the teaching story for humans and review grounding. Keep the Typology evidence brief (typology_architecture input) as source material; do not pretend it is the confirmed catalog.
Do not copy hollow template slice objectives; paraphrase into concrete teaching language grounded in the ledger, catalog, and README.
Chronology must stay honest, with at most a single explicit seed marker. Do not reconstruct past decisions.
The grounding output should summarize the accepted mission and architecture for agenting on the served product.
If evidence is thin, keep the section minimal rather than inventing details.
When validation_feedback is present, fix those issues before emitting.
Preserve each file's markdown shape and heading conventions. Preserve majordomo-reading-nav banners when present.`)
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
			in("package_capability_constraints", "Durable is/must_not capability codes per package (and filled_by from fills_dto edges). Factual; MUST NOT contradict."),
			in("mechanical_grouping_yaml", "Deterministic door-walk seed YAML: door-private vs shared vs unreached, libraries, product clumps."),
			in("architecture_draft", "Architecture brief for the raw draft"),
			in("repo_layout", "Top-level layout names"),
			in("readme_snapshot", "Served-repo README: product purpose and delivery commands"),
			in("validation_feedback", "Optional prior structure-validation feedback to fix"),
		},
		[]core.OutputField{
			// Flat strings (not XML arrays): empty [] fails strop mandatory validation.
			// When proposing no folds, emit the literal "none" in each field.
			// Go splits and zips into slice_grouping_proposal.yaml.
			out("merge_ids", "Comma-separated free-form merge nickname ids, or the literal none when proposing no folds"),
			out("merge_packages", "Semicolon-separated package groups (comma-separated paths inside each group), same order as merge_ids; or none"),
			out("merge_intents", "Comma-separated intents: each value MUST be exactly the literal slice or nickname (same order as merge_ids; never repeat the merge id); or none"),
		},
	).WithInstruction(`You are the unattended Typology slice-grouping pass for Majordomo context digest.
A discover draft is package-level inventory. package_roles is the factual observed topology. Grouping is an optional overlay and MUST NOT contradict package_roles.
package_capability_constraints is factual is/is-not prior. MUST NOT contradict it.

Order of evidence (MUST):
1. package_roles: each package already has role + confidence + evidence from code (entrypoint, server, dto, exec_runner, aggregator, adapter, config, observability, unknown).
2. package_capability_constraints: portable is / must_not codes and filled_by from fills_dto edges.
3. mechanical_grouping_yaml: deterministic door-walk seed. It is authoritative for door-private vs shared vs unreached facts; the LLM must not silently override those facts.
4. package_contracts and readme_snapshot: supporting facts.
5. graph_text: coupling and wiring only. Labeled edges in package_roles (fills_dto, uses_runner, serves_server, composes, reads_config) explain imports.
6. Folder and path words (dashboard, board, cli, server) are NEVER evidence and MUST NOT relabel a node.

Hard rules from observed roles and the door-walk seed:
- entrypoint and server are distinct doors. MUST NOT merge them. Cross-door wiring is a note, not ownership.
- Door-private packages may form product slices for that door only.
- Shared-across-doors packages must not be claimed as sole ownership for one door. Shared is not the same as library; only dto/config/exec_runner/observability (and similar technical roles) are library by role.
- Unreached packages MUST NOT be auto-owned; argue or leave debt.
- dto packages are shared data contracts; MUST NOT merge them into an aggregator or call them the product domain.
- aggregator packages build page/domain data; MUST NOT label them kind: ui or "the website".
- exec_runner packages are technical runners; MUST NOT put them under the entrypoint's domain just because the entrypoint also imports them.
- observability packages boot tracing/metrics; they are not config.
- fills_dto edges mean adapters fill JSON types; they are NOT "forge depends on the UI".
- uses_runner edges mean a package shells out through a runner; they are NOT "depends on the CLI domain".

Empty folds are valid: the literal none in all three fields is an allowed default when no lifecycle-same fold is earned. Do not invent companions or product pillars to avoid none.

Apply these merge heuristics only after honoring package_roles:
1. Propose a fold only when packages share the same lifecycle job (same high-confidence role family) with evidenced wiring. Companions are optional; do not invent folds.
2. Split companion packages that share a stem (sa + satools -> sa) when that fold is earned.
3. Sole importer: wiring note only; never merge-into-caller against observed roles.

Mechanical product clumps in mechanical_grouping_yaml are seed hints, not earned catalog slices until a later cluster_merge accept verdict. Folding into the refined catalog requires that accept verdict.

Enforce anti-patterns:
- Do not promote capabilities to domain pillars.
- Do not create standalone cli or platform slices.
- Do not treat exec_runner as kind: cli.
- Every draft package path must remain claimed under owns[], surfaces[], or libraries[].owns[].

Declare domain-free utilities under libraries[] when the draft or graph shows them.

Emit three parallel flat strings of the same length (or all the literal none):
- merge_ids: comma-separated free-form nicknames, or the literal none (example when no folds: none; positive example: git-adapters)
- merge_packages: semicolon-separated groups; inside each group comma-separated repo-relative paths, or the literal none (example when no folds: none; positive example: internal/adaptera,internal/adapterb)
- merge_intents: comma-separated; each token MUST be exactly the literal slice or nickname (not a purpose label or package stem). Never repeat the merge id or nickname here. For example: if merge_ids is "judge-eval", merge_intents must be "slice" (never "judge-eval"). Example when folds exist: nickname,slice. Or the literal none when no folds.
When proposing no folds, set each field to the literal none (not empty tags).
Do not emit markdown counsel or catalog YAML in this step.`)
	return newGenerator(sig, TaskTypologySliceGrouping)
}

func typologyRefineModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("module_scope", "Typology module scope"),
			in("draft_catalog_yaml", "Raw Typology discover draft YAML"),
			in("slice_grouping_proposal_yaml", "Proposed merges YAML composed from cluster CoT (id/packages/intent rows)"),
			in("slice_grouping_verdicts_yaml", "Durable cluster merge audit verdicts: accept|overlay|reject per proposed package set"),
			in("package_contracts", "Per-package public contracts from typology contracts"),
			in("package_roles", "Observed package role topology YAML: role, confidence, evidence, labeled edges"),
			in("package_capability_constraints", "Durable is/must_not capability codes per package; factual; MUST NOT contradict"),
			in("slice_meaning_ledger_yaml", "Authoritative evidence-first ledger: slice id, evidence quotes, claims, objective; copy objectives verbatim"),
			in("architecture_draft", "Architecture brief for the raw draft"),
			in("repo_layout", "Top-level layout names"),
			in("readme_snapshot", "Served-repo README: product purpose and delivery commands"),
			in("validation_feedback", "Optional ValidateStructure or boundary-evaluator feedback to fix"),
		},
		[]core.OutputField{
			// Explicit exception: full catalog stays one YAML string leaf until a follow-up splits slices into XML items.
			out("refined_catalog_yaml", "Full refined Typology catalog as a YAML document string (not a list field)"),
		},
	).WithInstruction(`You are the unattended Typology slice catalog writer for Majordomo context digest.
Apply accepted slice-grouping merges to the draft catalog and emit a complete refined typology.yaml.
package_roles, package_capability_constraints, slice_grouping_verdicts_yaml, and slice_meaning_ledger_yaml are factual. MUST NOT contradict them.
Folder names are never evidence.

slice_meaning_ledger_yaml already settled each owned slice's meaning (evidence, claims, objective).
For every ledger slice id that still owns packages, copy the ledger objective into the catalog verbatim.
When you merge draft neighborhoods into one refined slice, copy ONE contributing ledger objective verbatim (do not invent a prestige blend).
MUST NOT invent prestige objectives beyond the ledger. MUST NOT escalate data_shape slices into synchronize_state or merge_adapters stories.
Claims are produced by the ledger stage in Go; do not invent a competing claims sidecar.

Placement from roles:
- entrypoint -> kind: cli surfaces
- server -> surfaces (api, grpc, or ui when evidence includes embeds_static); NEVER fold into the CLI slice for sole importer; NEVER share a slice with an entrypoint
- dto -> owns[] (or a thin shared data slice); NEVER the product domain from graph position; NEVER owned by an aggregator just because that aggregator imports it
- aggregator -> owns[] of a product slice; MUST NOT kind: ui
- exec_runner -> owns[] or libraries[]; NEVER under the entrypoint domain solely because cmd imports it
- observability -> owns[] or libraries[]; NEVER config
- adapter / config -> owns[] or libraries[] as fits

Fold packages into one slice or libraries[].owns[] ONLY when slice_grouping_verdicts_yaml marks that package set verdict: accept.
overlay and reject rows stay separate package owners; nicknames MUST NOT disguise as libraries[].owns[].
MUST NOT invent "forge depends on UI" or "localgit depends on CLI" smells from false ownership.

Catalog rules:
- Every slice MUST have a non-empty business objective that states why the bounded context exists in one concrete sentence.
- MUST NOT use hollow template objectives such as "Provide X functionality", "Provide X capabilities", or "Provide X services".
- Each slice MUST use at most one owns block, one surfaces block, and one libraries block. List every package for that slice inside the same block instead of repeating the key.
- Components are packages under owns, under surfaces, or under libraries[].owns.
- Libraries are technical package groups with a purpose and owns[] only.
- MUST NOT invent libraries[] membership solely to clear findings.
- When libraries[] claim packages, emit evidenced SliceBinding entries from consumer slices.
- Surfaces are ui, cli, or api interaction artefacts for user-facing delivery based on observed roles, not path words.
- Every draft package path MUST appear under owns[], surfaces[], or libraries[].owns[].
- Preserve real package paths from the draft and graph verbatim.
- MUST NOT invent filesystem package folders.
- When validation_feedback is present, fix those issues before emitting.

Output refined_catalog_yaml as YAML only (no markdown fences). Do not emit journey markdown.`)
	return newGenerator(sig, TaskTypologySliceCatalog)
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
	// Legacy alias: brief-only so old task names keep a registered module.
	return typologyInterventionBriefModule()
}

func typologyInterventionSharedInputs() []core.InputField {
	return []core.InputField{
		in("repo_id", "Served repository id"),
		in("architecture_md", "Post-refine Typology architecture brief"),
		in("refined_catalog_yaml", "Refined Typology catalog YAML"),
		in("journey_md", "Journey notes from prior intervention step (may be empty on first write)"),
		in("slice_grouping_proposal_yaml", "Cluster merge proposal YAML (membership hint)"),
		in("slice_grouping_verdicts_yaml", "Cluster merge audit verdicts YAML"),
		in("findings_list", "Deterministic list of open architecture findings; each must be flagged for humans"),
		in("validation_feedback", "Optional prior validation feedback to fix"),
	}
}

func typologyInterventionJourneyModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		typologyInterventionSharedInputs(),
		[]core.OutputField{
			out("journey_md", "Updated journey with open Status and debt covering every finding"),
		},
	).WithInstruction(`You write typology journey notes after refine and architecture so open findings cannot hide.
` + consultantCounselContract + `

Rules:
- findings_list is authoritative. Every finding MUST appear in the Technical debt and boundary violations table.
- When findings_list is non-empty, journey Status MUST stay open (not complete/completed).
- When debt rows still say Merge into, journey Status MUST stay open (not complete/completed).
- Journey MUST include Status, decisions already taken, and a debt table that names every finding with smell, alternatives with a cost, and a lean.
- MUST NOT flatten debt rows to hollow "Approve binding or refactor".
- MUST NOT invent catalog YAML, sliceBindings, or libraries membership.
- Output markdown only in journey_md.
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyInterventionJourney)
}

func typologyInterventionBriefModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		typologyInterventionSharedInputs(),
		[]core.OutputField{
			out("human_intervention_md", "Tutor-voice operator briefing of priority decisions humans must make"),
		},
	).WithInstruction(`You write the operator human-intervention briefing after Typology refine.
Humans give direction and leadership. Surface architecture findings the unattended refine must NOT invent away.
` + consultantCounselContract + `

Rules:
- findings_list is authoritative. Every finding MUST appear in human_intervention_md.
- Output markdown only. MUST NOT invent sliceBindings, libraries[] rows, rewrite package ownership, or invent catalog YAML.
- Majordomo completes evidenced slice-to-library SliceBindings in the proposal catalog. MUST NOT ask humans to rubber-stamp those mechanical edges.
- Frame remaining findings as normative human decisions: keep library placement, fold into a domain slice, decouple, approve slice-to-slice binding, merge slices, or accept temporary debt.
- Tutor briefing: situation, smell/risk, alternatives with a cost, recommended lean, and evidence pointers. MUST NOT punt to journey_md.
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyInterventionBrief)
}

func typologyInterventionWeaknessesModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		append(typologyInterventionSharedInputs(),
			in("human_intervention_md", "Operator briefing already produced for these findings"),
		),
		[]core.OutputField{
			out("weaknesses_seed_md", "Weaknesses markdown bullets for bootstrap story seeding"),
		},
	).WithInstruction(`You seed weaknesses.md from open architecture findings and the operator briefing.
` + consultantCounselContract + `

Rules:
- findings_list is authoritative. Every finding MUST appear as a weakness bullet.
- Keep the same priorities and leans as human_intervention_md, but shorter.
- Output markdown only starting with # Weaknesses.
- MUST NOT invent catalog YAML or claim findings are resolved.
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyInterventionWeaknesses)
}

func typologyInterventionPRPriorityModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		append(typologyInterventionSharedInputs(),
			in("human_intervention_md", "Operator briefing already produced for these findings"),
		),
		[]core.OutputField{
			out("pr_priority_md", "Context PR summary markdown in tutor voice for a cold reader"),
		},
	).WithInstruction(`You write the GitHub context-PR summary a cold reader sees first, in a tutor voice.
` + consultantCounselContract + `

Rules:
- findings_list is authoritative. Every finding MUST appear in pr_priority_md.
- Assume the reader has never seen this repo. Lead with what Majordomo's Typology digest is proposing on this context branch and why it matters, then smell, alternatives, and the lean.
- Frame slice consolidations and library placements as proposed catalog models for grounding, not as work the product team already shipped.
- Gloss jargon in the same sentence. Name packages by role as well as id so coverage checks still match.
- MUST NOT use only imperative task titles such as "Formalize Config Access".
- MUST NOT dump catalog ids without a gloss. MUST NOT say "see journey_md".
- MUST NOT invent catalog YAML or ask humans to rubber-stamp mechanical slice-to-library bindings.
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyInterventionPRPriority)
}

func typologyFindingCommentModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("architecture_md", "Post-refine Typology architecture brief"),
			in("refined_catalog_yaml", "Refined Typology catalog YAML"),
			in("journey_md", "Updated journey markdown"),
			in("human_intervention_md", "Operator briefing"),
			in("finding", "One open architecture finding to discuss on the context PR"),
			in("validation_feedback", "Optional prior validation feedback to fix"),
		},
		[]core.OutputField{
			out("comment_md", "Tutor-voice PR comment body for this single finding"),
		},
	).WithInstruction(`You write one context-PR comment for a single open architecture finding so humans can discuss it in-thread.
` + consultantCounselContract + `

Rules:
- Cover only the given finding. Mention enough of the finding text (or a backticked id from it) that coverage checks match.
- Tutor voice: situation, smell/risk, alternatives with a cost, recommended lean.
- MUST NOT invent catalog YAML or ask humans to rubber-stamp mechanical slice-to-library bindings.
- MUST NOT say "see journey_md" for the real argument.
- Output markdown only in comment_md (no HTML markers; the host adds those).
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyFindingComment)
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
