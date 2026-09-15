# Caller-bound application questions

`abstraction.asks/application@1` admits and observes questions through generated
`QuestionApplication` clients. The service owns question text and options. The
Go host requires an explicit `LoadApplicationBook` provider and shared Program
peer proof. Its application client creates no provider files.

The receiving account and observed absolute executable path define scope.
Moving an executable changes scope; restarting it preserves scope. This is
executable-path identity with the platform's reported proof. The service requires
its own account and records the shared native `Via` observation. Application
requests contain no asker, upstream `For`, administrative credential or path.
Legacy native `Via`/`For` behavior remains available through explicitly selected
native integration. It is not promoted into this application's authority.

`request_key` is 1..128 UTF-8 bytes without control characters. A question key
must be known; slots must exactly match its declared placeholders excluding
`asker`. Each value is valid UTF-8, contains no control character and has at most
200 Unicode characters. The display name is derived from the bound executable.

Admission stores scope, request key and a canonical full-question fingerprint
atomically with the question. Repeating that key/content returns the original
question, including an answered `once`, across a service restart. Different
content conflicts. New keys may share pending/kept questions only in the same
scope with identical full content. Existing unscoped kept answers are excluded.
Forgetting a question preserves its admission keys: replay and observation give
`gone`, and the key never becomes a new admission. Another scope sees `unknown`.

Only `pending` and `answered` carry an Answer. Pending contains no decision.
Answered reports the selected operator choice; resource services separately
enforce authorization. `invalid`, `conflict`, `forbidden`, `unknown`, `gone` and
`unavailable` carry no answer. Storage failure and capacity refusal are
`unavailable`, never an inferred refusal or approval by a person.

`Observe` accepts `wait_ms` from 0 to 30000. Expiry returns the current state;
it may still be pending. Caller cancellation or timeout ends its transport wait.
It does not answer, forget or cancel a question. After uncertain Ask completion,
retry exactly the same key/content. Generated and handwritten clients never
retry automatically. The Go client defaults to five seconds for the whole call;
`WithTimeout` explicitly chooses a positive budget up to 35 seconds on a copy.
Caller context and configured timeout compose by taking the earlier deadline.
The host admits at most 32 simultaneous calls, each bounded to 35 seconds.
Disconnected clients release observation waiting through shared WaitContext; accepted admissions remain intact.

The application-profile file is separate from legacy asks.json and owned by one
configured host. Its version is `asks-application@1`; no automatic migration from
legacy state occurs. Canonical provider JSON is required. Unknown versions,
fields, noncanonical/invalid Unicode, nonregular nodes and files beyond 4 MiB
are refused. At most 4096 admissions and 4096 retained records are accepted;
capacity exhaustion refuses new admission while existing keys remain readable.
Native Answer/Forget on this provider preserve its metadata. Current legacy
LoadBook and Ask refuse application-profile use. Older binaries must never be
configured with this new file. Its parent directory must be service-owned and
private; replacing that namespace concurrently is outside this profile.
Deleting/restoring the state externally does not preserve retry guarantees.

## Authorized operator service

`abstraction.asks/operator@1` uses the same application-profile Book and endpoint.
The host explicitly configures `EnableOperator(AuthorizeOperator)` before Serve.
Each call requires receiving same-account Program proof plus the callback's
permission. ErrOperatorForbidden reports policy denial; other callback errors
report unavailable. The callback receives the rechecked typed peer and call context;
it is trusted service composition and may query rights. A nil callback refuses.
Identity attributes and caller-supplied text alone grant no operator permission.
No administrative secret or provider path is accepted. Native Via/For evidence
stays server-side. The existing operator UI is responsible for obtaining the
person's selection; this protocol records that selection without fabricating
consent or granting resource authority.

`ListQuestions(cursor, limit)` returns pending and answered question metadata.
Limit is 1..64 records; conservative compact-record plus indentation/envelope accounting bounds the
encoded reply to 256 KiB, within the shared 1 MiB frame limit. The underlying Book remains limited
to 4 MiB and 4096 records/admissions. Empty cursor starts at the current Book;
nonempty cursors are at most 256 UTF-8 bytes and bind receiving caller scope,
host epoch and exact Book revision. Changes, restart or scope mismatch return
`gap`; the operator explicitly restarts from an empty cursor. Stable content
supports replay of a continuation. This is latest-book enumeration, with no
promise to replay every historical change. Slow/disconnected readers retain no
server enumeration sessions. Authorization is rechecked before returning pages.
Storage failure is `unavailable`, never an empty successful history.

`AnswerQuestion(id, option)` accepts bounded IDs/options (1..128 UTF-8 bytes,
without controls) and only existing catalog options. It conditionally updates
the Book under the existing atomic edit lock. Context and authorization are
rechecked inside that edit after lock waiting, before recording a new choice. Repeating the same option returns
the original timestamp/choice/kept flag; another valid option returns `conflict`.
Missing IDs return `unknown`; invalid options return `invalid`. `answered` alone
carries a record. A lost reply or storage error may leave an uncertain outcome;
clients never retry automatically. Explicit same-ID/option retry or fresh history
reconciles it. Native Forget remains explicitly selected provider integration;
its admission tombstones prevent re-admission.

`RetireQuestion(id)` removes a retained pending or answered question for an
authorized operator. Authorization and context are rechecked inside the atomic
edit after lock waiting and before removal. The first retirement returns
`retired` with the question's last metadata; a pending question carries no
decision. Admission tombstones remain: application Ask replay and Observe of its
key report `gone`, and the key never admits again. Retiring an ID that only an
admission still names replays `retired` without a record, including after a
restart. `unknown` means no retained or retired question and `invalid` a
malformed ID. Storage failure is `unavailable` and establishes no retirement.
Clients never retry automatically; an explicit same-ID retry or fresh history
reconciles an uncertain call. Retirement grants and revokes no resource authority.

The operator UI obtains the person's selection or retirement decision and
submits it through this service. Retirement removes questions from operator
history; the application learns the outcome as `gone` and must not treat it as a
refusal or approval. Tombstones stay within the Book's 4096-admission bound, so
retirement frees record capacity and leaves admission capacity unchanged.
