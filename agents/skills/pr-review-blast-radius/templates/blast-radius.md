# PR #<!-- FILL: PR number --> — Blast Radius

**Base branch:** <!-- FILL: base branch name -->
**Reviewed at:** <!-- FILL: Sydney ISO 8601 timestamp — read from: <skill_dir>/review_timestamp.txt -->

---

## Live Seam Points

<!-- FILL: One H3 per entry in `reverse_deps` that is CONFIRMED LIVE by grep.

     For each `reverse_deps` entry {changed_file: [importer, ...]}, grep the live repo
     to confirm the import still exists in each importer. Only include entries where at
     least one importer is confirmed live. Skip entries where all importers have already
     been updated.

     H3 text: short statement of what the seam is.
     Example: "`legacy_navigator.py` is still imported by `main.py`."

     Body (3-5 sentences):
     - What the changed file is and what replaced it (if anything)
     - Which unchanged file(s) still import it (confirmed by grep)
     - What breaks or stays live if this seam is not resolved
     - What the reviewer should confirm or action

     If `reverse_deps` is empty or all imports are confirmed resolved: write a single
     paragraph stating that no live seam points were found and all changed files appear
     to be cleanly adopted.
-->

---

## Dependency Clusters

<!-- FILL: One subsection per entry in `dep_clusters` with 3 or more files.
     Skip singleton and two-file clusters — they add no blast radius signal.

     For each cluster, describe:
     - Which file is the root (most imported by others in the cluster)
     - Which files depend on it and what they do
     - What the consequence is if the root file has a bug

     Format each cluster as:

     **Cluster: `<root-file-basename>`**
     `<root>` → `<dep1>`, `<dep2>`, ...

     <1-2 sentences: what the root provides, what breaks if it's wrong.>

     If `dep_clusters` contains no clusters with 3+ files, omit this section entirely.
-->

---

## Scope Verification

<!-- FILL: For any claim in the PR (or inferred from the diff) that "all X were updated"
     or "Y is no longer used", use grep to verify against the live repo.

     For each verified or refuted claim, one entry:

     **Claim:** <what the diff implies — e.g. "LegacyNavigator is replaced and no longer needed">
     **Result:** CONFIRMED or REFUTED
     **Evidence:** <grep result — file and line if refuted, or "no remaining references found" if confirmed>
     **Action (if REFUTED):** <what needs to happen — one sentence>

     Limit: at most 5 verification checks. Prioritise claims backed by `reverse_deps` data first,
     then claims visible in deprecation annotations or comments in the diff.

     If no scope claims are present in the diff, omit this section entirely.
-->
