# PR #<!-- FILL: PR number --> — Technical Review

**Base branch:** <!-- FILL: base branch name -->
**Files changed:** <!-- FILL: count of unique files in manifest -->
**Reviewed at:** <!-- FILL: Sydney ISO 8601 timestamp — read from: <skill_dir>/review_timestamp.txt -->

---

## ⚠️ Correctness Risks

<!-- CONSTRAINT: One H3 per risk. H3 is a declarative statement of the failure mode, not a question. Order: most severe first. -->

### <!-- FILL: failure mode — declarative statement naming file and symbol -->

**Does:** <!-- FILL: what the code does — one clause -->

**Trigger:** <!-- FILL: the specific condition that causes the failure -->

**Consequence:** <!-- FILL: what breaks and how -->

**Confirm:** <!-- FILL: yes/no question the reviewer must answer in the diff -->

---

## 🔍 Verification Checklist

<!-- FILL: One bullet per file requiring a targeted read. Each names the full path and the single yes/no question to answer.
     Order: highest-consequence first. Last bullet is always a skip entry. -->

- `<!-- FILL: full file path -->` — <!-- FILL: yes/no question -->

- Skip: `<!-- FILL: full file path -->` — <!-- FILL: one-line reason why it can be safely skipped -->

---

## 🧪 Test Coverage Gaps

<!-- FILL: One bullet per failure mode from ⚠️ that has no unit test. Format:
     "No test for `<method/class>` `<failure mode>` — `<test file>` covers `<what>` but not this path."
     If all risks are covered: write "All identified risks have unit test coverage." -->
