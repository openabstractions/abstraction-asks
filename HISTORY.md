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

- 2026-10-09, remove three redundant unreleased operator methods before 0.4.0.
  `ReadAttributedQuestion` owns metadata and attribution;
  `ReadQuestionAsker` owns trusted details and the bound asker. The draft
  `ReadQuestion` and `ReadQuestionDetails` methods are removed. A legacy target
  that cannot be recovered still returns unavailable from the trusted read;
  its metadata remains inspectable. `ListAnswers` owns the complete answer
  audit; consumers filter all its pages by choice kind and responder route.
  The draft `ListDelegatedAnswers` method and its two response records are
  removed. Book versions 7/8 retain their private decoder and migration into
  the existing audit. Published 0.3.0 contained none of those removed methods.

- 2026-10-08, typed requests in the Panel and settled requests withdrawn
  (ASK-A13, ASK-O10; research/release-0.4.0/journey-2026-10-08.md findings 1
  and 2). A typed permission request was answerable only in the native
  dialog: the Panel's `/questions` reads `operator@1`, which excludes typed
  occurrences, so a person who closed the dialog had no Panel path. The
  Panel's Questions waiting now lists them through `permission-operator@1`,
  worded by the dialog's projection and answered through
  `grants.AnswerPermissionQuestion` after a digest re-read; the navigation
  badge counts them. A request also stayed pending after a saved rule
  granted its access; the runtime now decides pending typed requests again
  after every rights change (`DecisionPolicy.OnChange`, RIGHTS-R23) and
  withdraws those a rule, default or Block settled. Tested by
  `serve/runtime_permission_settle_test.go`,
  `go/typed_permission_settle_test.go`,
  `monitor/permissions_panel_test.go` and `monitor/rights_live_page_test.js`.
- 2026-10-08, every recorded answer is audited (ASK-O15, ASK-O13, ASK-P1;
  TODO "Activity shows only answers the assistant gave"). The Panel's
  Activity timeline read only assistants' answers: the person's own answers
  left the Book when a question was released or dismissed or a choice was
  acknowledged, and only assistants' choice answers had an audit. The
  delegated-answer audit becomes the answer audit of every answer recorded
  through `operator@1`, `permission-operator@1` or `responder@1`, with its
  route, under the same bounds; `ListDelegatedAnswers` is its responder
  choice view, and `operator@1` gains `ListAnswers`. Book version 9; a read of
  version 7 or 8 moves the delegated entries into the audit.

- 2026-10-07, the enrollment is asked like any other right (ASK-R10,
  ASK-R2, ASK-R5; owner: "assistant enrolment is ask for right and for that
  we have popup panel already"). An assistant could be enrolled only from the
  Panel. `responder@1` gains `RequestAnswering`; the runtime admits its
  ordinary first-use question for `question.answer` on `apps`, shown in the
  native permission prompt and administrative, so only the person answers.
  The person's Allow writes the Panel's enrollment with its 30-day lapse
  through `ApplyFirstUseAllow` (RIGHTS-O10). The Panel keeps the list, the
  warning and Stop. Tested by `serve/runtime_enrol_test.go`,
  `go/application/answering_request_test.go`,
  `abstraction-rights/go/operator_allow_enrollment_test.go` and the grants
  enrollment test.
- 2026-10-07, shared ordinary choices and their audit (ASK-A12, ASK-R9,
  ASK-O9, ASK-O12, ASK-O13, ASK-P1;
  research/release-0.4.0/planned/ordinary-agent-delegation.md). An
  enrolled assistant could neither see nor answer an ordinary choice:
  `ListAnswerable` read only the legacy partition and ASK-O9 excluded
  enrollment. The owner decided one yes covers both (VISION 2026-10-07).
  `application@2` gains `ShareChoice`, `responder@1` gains `ListChoices` and
  `AnswerChoice`, and `operator@1` gains `ReadChoiceDetails` and
  `ListDelegatedAnswers`, each an added method under the 2026-10-01
  service-version decision. Book v7 keeps the requester program, shared
  options, decision route and a bounded audit that acknowledgement leaves in
  place; it also closes the TODO row on choices keeping only the asker's file
  name. Tested by `go/application/shared_choice_test.go` (the gap regression
  first failed to build: none of the methods existed) and
  `go/shared_choice_book_test.go`.
- 2026-10-07, script hosts by file name on every platform (ASK-R7;
  research/python-identity/FINDINGS.md, side finding). The file-name
  fallback held only Windows names, so macOS and Linux shells, `osascript`
  and interpreters such as `python3.12` could enroll as answerers.
  `ScriptHostFileName` now owns one cross-platform stem list, read after
  removing `.exe` and a trailing version; monitor's icon-only duplicate
  is removed. Tested by `go/scripthost_test.go` per platform group.
- 2026-10-06, the bound asker program (ASK-A2, ASK-P1, ASK-O11). An
  application question recorded only its asker's file name, so an operator
  could not tell two programs of one name apart (review of `139b58323`).
  Book v6 keeps the bound program the receiver derived its scope from, and
  the asker is its file name derived from it. Versions 1..5 refuse the field
  and migrate on edit; an older runtime refuses a v6 file. The program
  reaches operators only through the new `ReadQuestionAsker`, since every
  existing operator result refuses unknown fields. The Panel Questions page,
  the native prompt and `questions pending` show the derived name with the
  program beside it (`e31ee2e27`, `92363c8c2`). Ordinary choices keep the
  display name: their stored record is a closed wire type.

- 2026-10-05, typed permission consent (ASK-A10/A11, ASK-R8, ASK-O10,
  ASK-E3). Additive `permission@1` and `permission-operator@1` preserve the
  existing application and exact first-use APIs. Native capability preparation
  binds the caller and canonical owner target; consent admission remains
  available before an application holds any ordinary asking grant. Book v5
  freezes the complete One/All or versioned finite-bundle proposal and trusted
  display. Rights `operator@2` commits and undoes that pinned effect separately.
  Existing exact responder scopes gain no typed answering authority.

- 2026-10-04, completed operator Allows ask again (ASK-R2; owner-required
  0.4 correction). Rights ApplyFirstUseAllow derives complete permits from
  the pinned actual operator answer and commits occurrence-linked success
  proof with those permits. Revocation before any successful application
  retry, including after a lost reply/restart, retains proof and permits a
  fresh question. Failed writes keep explicit Retry. Release now checks the
  observed random occurrence ID under Book CAS; Q1 cannot remove Q2 under
  the same request key. Existing asks answer methods still record choice
  only, and the asks wire shape stays unchanged.

- `VISION.md` 2026-09-30, "Inference activation follows authorized
  admission; permission requests support native presentation and delegated
  answers" — adds `ASK-O6` (TODO HP8). A native prompt is an operator UI:
  it shows a question only in an unlocked session and re-reads it before
  answering. No wire change; the rule relies on ASK-O3's existing conflict
  and replay behavior. Tested by
  `go/application/revalidate_test.go` and the Panel's
  `monitor/tray_presenter_test.go`; evidence in
  `feedback/hp8-native-authorization.md`.

- HP8 follow-up (1), 2026-10-01 — adds `ASK-A6` and `ASK-O7`. The service
  records the requester's publisher at admission, and a native prompt shows
  that recorded publisher in place of a file check at display time. Wire
  change: `QuestionOperator.ReadQuestion` with `OperatorQuestion` and
  `OperatorQuestionOutcome`, added to `operator@1` as a new method. The
  publisher fields stay out of `RecordMetadata`, whose earlier readers
  refuse unknown fields. Tested by `go/publisher_test.go`,
  `go/application/read_question_test.go`, `serve/runtime_asks_test.go` and
  `monitor/tray_presenter_test.go`.

  Downgrade (review F3, decided 2026-10-01): the state file keeps the
  version `asks-application@1`, and its records gain the optional
  `publisher` and `publisher_trusted` members. A runtime older than this
  change refuses them as unknown fields (ASK-P1) and does not read an
  `asks/application.json` this release wrote. To downgrade, stop the
  runtime and delete that file; pending questions and the answered question
  history are lost, and the rights rules answers wrote are unaffected.
  `serve/SUPERVISION.md` carries the same note.

- Review residual, 2026-10-01 — `OperatorQuestion` gains `answered_by`
  (closed enum `Answerer`: `operator`, `responder`), set by the service when
  it records an answer, and `ASK-O7` forbids retrying a responder's answer
  into a permanent rule. `OperatorQuestion` is new in this release and not
  yet released, so it may still gain a member (`VISION.md` 2026-10-01, "A
  service version may gain a method; a closed struct or enum may not gain a
  member"). The state file's records gain the optional `answered_by`
  member; the downgrade note above covers it. Tested by
  `go/application/answer_if_pending_test.go`,
  `go/application/responder_test.go`,
  `abstraction-facade/go/grants/first_use_test.go`
  (`TestRetryFirstUseRuleRefusesAResponderAnswer`) and, on the real Book,
  `monitor/questions_panel_test.go`
  (`TestQuestionsPageRetryRuleRefusesADelegatedAnswer`).

- HP8 follow-up (2), 2026-10-01 — adds `ASK-O8`. The native prompt answers
  only while the question is still pending, so an HP9 responder's answer
  that lands after the re-read reads `conflict` and no permanent rule is
  written over its bounded permit. Wire change:
  `QuestionOperator.AnswerQuestionIfPending`, added to `operator@1` as a new
  method because method arguments cannot be optional. Tested by
  `go/application/answer_if_pending_test.go` and
  `monitor/tray_presenter_test.go`
  (`TestPresenterAnswerCannotReplayADelegatedAnswer`).

- HP8 follow-up (3), 2026-10-01 — adds `ASK-A7`: a question still pending
  15 minutes after it was asked expires and reads as absent everywhere.
  Evidence, fetched 2026-10-01 and cited in the rule: RFC 8628 §3.2 (a
  required `expires_in`, example 1800 s), GitHub's device flow (default 900
  s), Microsoft Entra's device code flow (15 minutes), OpenID CIBA §7.3 (a
  required `expires_in`, example 120 s), Chromium's
  `permission_request_manager.cc` (a prompt ends with its page), Apple's
  `AVCaptureDevice.requestAccess` and Android's runtime permission guide
  (no documented expiry). The Windows UAC consent timeout is sourced only
  from community reports, and no Microsoft documentation of its value was
  found. Rejected: `gone` with a tombstone per expiry. It would claim a
  person dismissed the question, silence the runtime's first-use question
  after 16 expiries of one program, action and resource, and spend the
  admission bound. No wire change. Tested by `go/expiry_test.go`,
  `go/application/expiry_test.go`, `serve/runtime_asks_test.go` and
  `monitor/tray_presenter_test.go`
  (`TestPresenterStopsPresentingAnExpiredQuestion`).

- `VISION.md` 2026-10-01, "A prompt names a signed program by its signed
  version resource, and a script host's prompt says its publisher covers
  the host" — extends `ASK-A6` and `ASK-O7`, adds `ASK-A8`. A trusted
  publisher is recorded with the program's signed description and original
  file name (identity's `Code.Description`, `Code.OriginalName`), and the
  package owns the reviewed script-host list and its sentence. The list
  lives in `go/scripthost.go`; ASK-A8 and VISION cite it rather than copy
  it. Review of 2026-10-01 added `git.exe`: Git for Windows runs hooks and
  `!` aliases, and its `bin\bash.exe` launcher signs as "Git for Windows"
  with `OriginalFilename` `git.exe`. Wire change:
  `OperatorQuestion` gains `program_description` and
  `program_original_name`. `OperatorQuestion` is new in this release and
  not yet released (the last published asks tree, `published.tsv`
  `a720bfcc`, has none), so it may still gain members (`VISION.md`
  2026-10-01, "A service version may gain a method; a closed struct or enum
  may not gain a member"). The state file's records gain the optional
  `program_description` and `program_original_name` members; the downgrade
  note under HP8 follow-up (1) covers them. Tested by
  `go/publisher_test.go`, `go/scripthost_test.go`,
  `go/scripthost_windows_test.go` (the list's spellings against this
  machine's signed files), `go/application/read_question_test.go` and
  `monitor/tray_question_test.go`.

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
- 2026-10-02, no self-approval (ASK-R3). Research found that the runtime's
  responder authorization (`serve/runtime_asks.go` `authorizeResponder`)
  checked the responder's exact, expiring permit and never compared the
  responder with the question's target program or its asker. An operator
  could delegate an application to answer its own first-use question. The
  runtime now refuses such a responder before reading its permit:
  `Answer` reads `forbidden`, and `ListAnswerable`, which asks the same
  authorization per option, omits the question. The Panel's delegation
  page refuses to enroll a question's own program as its responder. The
  OpenCode authorization tool reaches the same check through
  `responder@1`.
- 2026-10-02, self-approval hardening (security review of the change
  above). The asker is also compared by the record's verified peer
  (`via.path`, when bound), beside the runtime, so questions an application
  admits are covered once they become delegable. The coordinator decided
  two further parties: a program whose trusted signer is the target's
  recorded trusted publisher, and a program that is the target's
  executable under another path (volume and file ID). They cover a vendor's
  own signed helper and a hard-linked copy of the app; the cost is that two
  apps of one trusted publisher cannot answer each other's questions. The
  Panel also refuses the runtime program as a responder. A unit test
  checks case-variant and 8.3 spellings against the runtime and asker,
  where no file check applies.
- 2026-10-02, a lapsed allow asks again (ASK-R2; live walkthrough D4,
  `research/live-runs/2026-10-02-walkthrough.md`). After a delegated
  permit was revoked, the application was refused with no new question
  until someone pressed Dismiss: an answered first-use question replayed
  for every later refusal. A refusal of a question answered `allow` now
  releases that question the way expiry removes a pending one, with no
  tombstone, and asks again under the same key (`Book.ReleaseAnsweredApplication`).
  A dismissal would also re-ask, through the next generation key, but each
  costs one of the sixteen generations a key may follow, and a delegated
  permit lapses within five minutes (ASK-R4), so dismissals would silence
  a busy application within hours. `refuse` and `never` still replay.
- 2026-10-02, the re-ask rule narrowed and `refuse` decided (coordinator).
  An `allow` is asked again only when its delegated effect was `applied`;
  an allow whose effect conflicted or failed, or an operator's allow, keeps
  its record so the Panel's Retry rule still finds it. The runtime records
  applied effects beside the Book (`first-use-effects.json` in its asks
  state) rather than in the Book's records, so an earlier runtime can still
  read a Book this one wrote. `refuse` means "not this time": it is asked
  again once 10 minutes have passed since the answer, at most that often
  per program and question. `never` stays a silent deny.
- 2026-10-02, the re-ask race (security review of the change above). A
  refusal decided before a delegated `allow` landed could reach the
  admission after it and release the question the person had just
  answered. The runtime now decides again before a release and releases
  only on a current `not_granted`. Its applied-effect record is read and
  changed through cas within 512 KiB; an unreadable record is reported once
  and set aside, and the next write starts a fresh one.
- 2026-10-02, paired questions (review of the change above). The second
  decision read only `content.write` for a paired input or output
  question, so an allow whose `content.read` alone was revoked kept
  replaying. Both rules are now decided.
- `VISION.md` 2026-10-03, "The 0.4.0 rights slice" — adds `ASK-R5`
  (enrollment and class-scoped answering), `ASK-R6` (tiers from one table)
  and `ASK-R7` (the answerer is a program and never a script host), and
  amends `ASK-R1`, `ASK-R3`, `ASK-R4` and `ASK-O7`
  (research/rights-model/PROPOSAL.md §15). The tier check applies to every
  answerer, an exact delegation included, so a sensitive first-use question
  (lend, a download job) that ASK-R3 let a delegate answer is now the
  person's only. Compatibility correction: the closed `OperatorQuestion`
  stays unchanged. Additive `ReadAttributedQuestion` returns it inside a
  new `AttributedQuestion` with optional bound `answerer`; old readers of
  `ReadQuestion` continue to decode it. The Book's version 2 application
  profile stores the actor; canonical version 1 files read and migrate on
  edit, while an older runtime refuses version 2. Tested by
  `go/application_profile_test.go`, `go/answerer_test.go`,
  `go/application/responder_test.go`, `serve/runtime_answerer_test.go` and
  `monitor/rights_assistant_test.go`.
  The owner then made Stop's grant cleanup optional and Undo a guarded
  removal of the answer's still-current permit effects. Enrolled and exact
  delegated paired effects now write in one rights edit; an existing read
  deny leaves no write permit. The runtime judges on-machine inference
  servers by literal loopback or authenticated local IPC, not merely by a
  router host's local-placement flag.

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
