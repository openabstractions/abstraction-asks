# History: abstraction.asks/application@1, abstraction.asks/operator@1

Linked from [CONTRACT.md](CONTRACT.md)'s "Reading this page."

## Superseded ids

No id is retired this release. `ASK-R1` keeps its number, restated in the
`[PREFIX-LN]` form this shape requires. `ASK-A1`..`ASK-A5`, `ASK-W1`..`ASK-W4`,
`ASK-P1`..`ASK-P3`, `ASK-R2`, `ASK-O1`..`ASK-O5` and `ASK-E1` are declared
for the first time: this contract's admission, waiting, profile and operator
sections previously carried the same obligations as prose with no id a test
or refusal could cite.

## Design records

- `research/vocabulary/DECISION.md` §2 — D14 (the Go host → the service in
  prose, its Go type named only in code font), D16 (scope → subject), D30
  (a still-shipped earlier interface → deprecated, its removal release named
  or "kept"), D45 (removing a persistent record → remove), D50 (dismissing a
  question without answering → dismiss), D57 (a caller-owned retry key →
  idempotency key), D58 (a question template's id → question id). Applied
  this release: all seven, in prose only; no wire field or method name
  changes (`research/vocabulary/RENAME-PLAN.md`'s asks steps 2, 3, 4 [doc
  half], 5, 6 are class `doc`).
- `research/vocabulary/DECISION.md` §3 (S1–S13) — the contract shape this
  page and its "Reading this page" section follow. The page code is `ASK`
  (S4); at three letters it is well under the eight-letter cap
  `documents.sh`'s `SHAPE_ID` regex allows, so no prefix truncation was
  needed (S4's own caution against page codes over eight letters).
- `research/vocabulary/RENAME-PLAN.md` "asks (7 steps)" — the seven rows
  this release applies, cited by number in "Applied and skipped" below.
- `research/vocabulary/RENAME-PLAN.md` §4 "third" release — schedules
  `asks/operator@2` (`DismissQuestion`), the wire half of step 4; recorded
  as the S13 prose-to-wire table entry and as a "Not built" line in
  CONTRACT.md until that release.

## Applied and skipped

- Step 1 — title, ids, letter table: applied. Title is
  `# abstraction.asks contract`; ids `ASK-A1`..`ASK-A5` (admission),
  `ASK-W1`..`ASK-W4` (waiting), `ASK-P1`..`ASK-P3` (profile), `ASK-R1`,
  `ASK-R2` (rule), `ASK-O1`..`ASK-O5` (operator), `ASK-E1` (outcome); letter
  table declares A, W, P, R, O, E and X (reserved, unused).
- Step 2 — scope → subject (D16): applied, in the subject rule (ASK-A1),
  the idempotent-admission rule (ASK-A3) and the dismissal rule (ASK-A4).
- Step 3 — legacy `Via`/`For` and legacy `asks.json` → deprecated with a
  removal release named or "kept" (D30): applied. Neither
  `research/vocabulary/RENAME-PLAN.md` nor `research/vocabulary/DECISION.md`
  names a removal release for either; both are recorded "kept" (ASK-A5,
  ASK-P1).
- Step 4 — retire → dismiss, forget → remove (D50, D45); `RetireQuestion`
  waits for `operator@2`: applied in prose (ASK-O4, ASK-A4, ASK-O5), with
  the wire method and outcome word left `RetireQuestion`/`retired` and
  recorded in the S13 table and CONTRACT.md's "Not built". The step's code
  half, `monitor/rights_page.go:7`, belongs to the monitor module and is not
  touched here.
- Step 5 — question key → question id, request key → idempotency key (D57,
  D58): applied (ASK-A2). The wire fields (`ApplicationQuestion.key`,
  `.request_key`) are unchanged this release; no release for either is
  scheduled in `research/vocabulary/RENAME-PLAN.md` §4, so no S13 table
  entry is added — the rules name the wire field in code font beside the
  new prose word instead.
- Step 6 — "the Go host" → the service (D14): applied. `CONTRACT.md`'s
  opening paragraph and ASK-O1 say "the service" and "the host" (the
  operator of the service, not a Go type); the Go type `Host` is not named
  in this contract's prose.
- Step 7 — `host:<name>` in the `rights.first_use` example → `server:<name>`:
  **skipped.** The module's files carry no `host:<name>` example anywhere
  near the `rights.first_use` text (`CONTRACT.md`'s "Asking is decided by a
  rule" section, now ASK-R2): the resource is the placeholder `<resource>`,
  never a literal `host:<name>`, in `CONTRACT.md`, `README.md` and
  `go/questions.go`. Confirmed by search across the module; nothing in
  `ASK-R2` cites a resource string to rename.

## Reviewed

No review finding for the asks module is listed in
`research/vocabulary/REVIEW.md` or `research/vocabulary/RENAME-PLAN.md`'s
asks rows.
