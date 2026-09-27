# abstraction.asks contract

Binds: `asks.thrift`

`asks.thrift` defines `abstraction.asks/application@1`, which admits and
observes fixed-catalogue questions on a caller's own subject, and
`abstraction.asks/operator@1`, which lets an authorized operator list,
answer and dismiss them. The service owns question text and options; it
requires an explicit `LoadApplicationBook` provider and shared Program peer
proof, and its application client creates no provider files. Both services
are served through the same versioned application-profile Book, reached
from generated `QuestionApplication` and `QuestionOperator` clients.
[README.md](README.md) is the walkthrough: obtaining the module, the fixed
question catalogue, and a worked `Ask` and `Observe` call.

## Reading this page

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY" and "OPTIONAL" in this page are to be
interpreted as described in RFC 2119 and RFC 8174, when, and only when, they
appear in all capitals, as shown here.

A rule id such as `ASK-A1` is declared once, in bold brackets before its
title, at the head of the rule it names; tests and refusals cite it the same
way. A retired id is never reused; [HISTORY.md](HISTORY.md) keeps it with
the release it left. `HISTORY.md` also carries this contract's design
records and what was measured or tested, linked from here and linking back.

| letter | topic |
| --- | --- |
| A | admission — binding the subject, and admitting a question |
| W | waiting — `Observe`'s long poll, retry and call budgets |
| P | the profile — the application-profile file's format and bounds |
| R | the rule — the rights decision that gates asking |
| O | operator — the authorized operator service |
| E | error and outcome — the outcome words a call can return |
| X | extension — reserved, unused |

| Prose | Wire (until its own release) |
| --- | --- |
| dismiss, dismissed | `RetireQuestion`, outcome `retired`, until `operator@2` (`research/vocabulary/RENAME-PLAN.md` §4, third release) |

## Admission

**[ASK-A1] The subject.** The service MUST derive every admission's subject
from the receiving account and the calling program's observed absolute
executable path, proved by the platform, never claimed by the caller.
Moving the executable changes the subject; restarting it preserves the
subject.

**[ASK-A2] The question and its slots.** `Ask` MUST refuse an unknown
question id. Slots MUST exactly match the question's declared placeholders
excluding `asker`. Each slot value MUST be valid UTF-8, MUST contain no
control character, and MUST NOT exceed 200 Unicode characters. The
idempotency key (the wire's `request_key`) MUST be 1..128 UTF-8 bytes
without control characters. The display name shown to a person is derived
from the bound executable, never supplied by the caller.

**[ASK-A3] Idempotent admission.** The service MUST store the subject, the
idempotency key and a canonical full-question fingerprint atomically with
the question. Repeating that key and content MUST return the original
question, including an answer kept as `once`, across a service restart.
Different content under the same key is `conflict`. A new key MAY share a
pending or kept question only within the same subject and with identical
full content; an existing kept answer with no subject attached is excluded
from that match.

**[ASK-A4] A dismissed question stays dismissed.** Dismissing a question
(ASK-O4) preserves its admission's key: replay through `Ask` and
observation through `Observe` MUST both report `gone`, and that key MUST
never admit a new question. Another subject sees `unknown`.

**[ASK-A5] Legacy native `Via`/`For` is deprecated.** The service MUST
require its own account and MAY record the shared native `Via` observation;
an application request carries no asker, upstream `For`, administrative
credential or path. Legacy native `Via`/`For` behavior is deprecated, kept
for explicitly selected native integration, and MUST NOT be promoted into
this application service's authority.

## Waiting

**[ASK-W1] `Observe` waits, and only waits.** `Observe` MUST accept
`wait_ms` from 0 to 30000. Waiting expiry MUST return the current state,
which may still be `pending`. Caller cancellation or a client timeout MUST
end only the transport wait: it MUST NOT answer, dismiss or cancel the
question.

**[ASK-W2] Retry after an uncertain call repeats the same key.** After an
`Ask` whose completion is uncertain, a caller MUST retry with exactly the
same key and content to reach the same record. Generated and handwritten
clients MUST NOT retry automatically.

**[ASK-W3] Call budgets.** The Go client MUST default to a five-second
budget for the whole call; `WithTimeout` MAY choose a positive budget up to
35 seconds on a copy of the client. Caller context and a configured timeout
compose by taking the earlier deadline. The host MUST admit at most 32
simultaneous calls, each bounded to 35 seconds.

**[ASK-W4] Disconnection releases waiting, not admission.** A disconnected
client's observation wait MUST be released through the shared
`WaitContext`; the admission it was waiting on MUST remain intact.

## The profile

**[ASK-P1] The profile file's own format.** The application-profile file
MUST be kept separate from the deprecated legacy `asks.json` (kept, no
removal scheduled) and MUST be owned by one configured host. Its version
MUST be `asks-application@1`; no automatic migration from legacy state
occurs. Canonical provider JSON is REQUIRED. An unknown version, an unknown
field, noncanonical or invalid Unicode, a nonregular node, or a file over 4
MiB MUST be refused.

**[ASK-P2] Capacity.** The profile file MUST accept at most 4096 admissions
and 4096 retained records. Once either is exhausted, the service MUST
refuse new admission while every existing key remains readable.

**[ASK-P3] The legacy provider boundary.** Native `Answer` and the legacy
native `Forget` operation on this provider MUST preserve the profile's
metadata unchanged otherwise. The current legacy `LoadBook` and `Ask` MUST
refuse application-profile use; an older binary MUST NOT be configured with
this new file. The profile's parent directory MUST be service-owned and
private. Replacing that namespace concurrently is outside this profile's
guarantee, and deleting or restoring its state externally does not preserve
the retry guarantees above.

## The rule

**[ASK-R1] Asking is decided by a rule.** `resource_actions` is closed:
`abstraction.asks/question.ask` on the resource `account`. A host that
decides application `Ask` MUST configure `EnableAskPolicy`; the policy
receives the rechecked typed peer and the question id before admission. An
evaluated refusal is `forbidden`, and a decision that cannot be obtained is
`unavailable`; neither MUST admit, replay or record anything. `Observe` of a
caller's own admissions stays scoped by subject. With no rule configured, an
application MUST NOT be able to ask (DREAM §14).

**[ASK-R2] The runtime's own first-use question.** `rights.first_use` is
the question a runtime admits as itself when a spending action it gates
decides `not_granted`. Its About slot is the program's exact path, and its
text reads `<program> wants to <action> on <resource>`; `FirstUse` reads the
three back. A runtime's ask policy MUST refuse this question id to
applications. The answer grants nothing by itself: an operator program
writes the permit rule on `allow` and an exact deny rule on `never`;
`refuse` writes nothing.

## Operator

**[ASK-O1] The operator service is explicitly authorized.**
`abstraction.asks/operator@1` MUST use the same application-profile Book
and endpoint. The host MUST explicitly configure
`EnableOperator(AuthorizeOperator)` before `Serve`. Each call MUST require
receiving same-account Program proof plus the callback's permission;
`ErrOperatorForbidden` reports policy denial, and any other callback error
reports `unavailable`. The callback receives the rechecked typed peer and
call context; it is trusted service composition and MAY query rights. A nil
callback MUST refuse. Identity attributes and caller-supplied text alone
grant no operator permission, and no administrative secret or provider path
is accepted. Native `Via`/`For` evidence stays server-side. The operator UI
is responsible for obtaining the person's selection; this protocol records
that selection without fabricating consent or granting resource authority.

**[ASK-O2] `ListQuestions`.** `ListQuestions(cursor, limit)` MUST return
pending and answered question metadata. `limit` MUST be 1..64 records,
bounded so the encoded reply MUST NOT exceed 256 KiB, within the shared 1
MiB frame limit; the underlying Book stays limited to 4 MiB and 4096
records/admissions. An empty cursor starts at the current Book; a nonempty
cursor MUST be at most 256 UTF-8 bytes and binds the receiving caller's
subject, the host epoch and the exact Book revision. A change, a restart, or
a subject mismatch returns `gap`, and the operator MUST explicitly restart
from an empty cursor; stable content supports replay of a continuation, but
this is latest-book enumeration, with no promise to replay every historical
change. A slow or disconnected reader retains no server enumeration
session. Authorization MUST be rechecked before returning each page.
Storage failure is `unavailable`, never an empty successful history.

**[ASK-O3] `AnswerQuestion`.** `AnswerQuestion(id, option)` MUST accept
bounded IDs and options, 1..128 UTF-8 bytes without control characters, and
only an option the catalog already lists. It conditionally updates the Book
under the existing atomic edit lock; context and authorization MUST be
rechecked inside that edit, after lock waiting and before recording a new
choice. Repeating the same option MUST return the original timestamp,
choice and kept flag; another valid option is `conflict`. A missing id is
`unknown`; an invalid option is `invalid`. Only `answered` carries a record.
A lost reply or storage error may leave an uncertain outcome; clients MUST
NOT retry automatically, and an explicit same-id/option retry or fresh
history reconciles it. The legacy native `Forget` operation remains
explicitly selected provider integration; its admission tombstones prevent
re-admission.

**[ASK-O4] `DismissQuestion`.** `DismissQuestion` (the wire's
`RetireQuestion` until `operator@2`, `research/vocabulary/RENAME-PLAN.md`
§4 third release) removes a retained pending or answered question for an
authorized operator. Authorization and context MUST be rechecked inside the
atomic edit, after lock waiting and before removal. The first dismissal
returns `dismissed` (the wire's `retired`) with the question's last
metadata; a pending question carries no decision. Admission tombstones
remain: an application's `Ask` replay and `Observe` of its key MUST report
`gone`, and that key MUST never admit again. Dismissing an id that only an
admission still names replays `dismissed` without a record, including after
a restart. `unknown` means no retained or dismissed question, and `invalid`
means a malformed id. Storage failure is `unavailable` and establishes no
dismissal. Clients MUST NOT retry automatically; an explicit same-id retry
or fresh history reconciles an uncertain call. Dismissal grants and revokes
no resource authority.

**[ASK-O5] The operator UI.** The operator UI obtains the person's
selection or dismissal decision and submits it through this service.
Dismissal removes a question from operator history; the application learns
the outcome as `gone` and MUST NOT treat it as a refusal or an approval.
Tombstones stay within the Book's 4096-admission bound, so dismissal frees
record capacity and leaves admission capacity unchanged.

## Outcomes

**[ASK-E1] Only a decided question carries an answer.** `pending` and
`answered` are the only outcomes that carry an `Answer`; pending carries no
decision, and answered reports the selected operator choice — a resource
service separately enforces its own authorization on that choice.
`invalid`, `conflict`, `forbidden`, `unknown`, `gone` and `unavailable`
carry no answer. A storage failure or a capacity refusal is `unavailable`,
never an inferred refusal or approval by a person.

| Call | Outcome | Meaning |
| --- | --- | --- |
| `Ask`, `Observe` | `pending` | Admitted; no person has answered yet (ASK-E1). |
| `Ask`, `Observe` | `answered` | The recorded choice, present only here and above (ASK-E1). |
| `Ask`, `Observe` | `gone` | The question was dismissed; the key never admits again (ASK-A4, ASK-O4). |
| `Ask`, `Observe` | `unknown` | No admission for this key in the caller's subject (ASK-A3). |
| `Ask`, `Observe` | `conflict` | The key was already used with different content (ASK-A3). |
| `Ask`, `Observe` | `invalid` | A question id, slot or the idempotency key is unknown or out of bounds (ASK-A2). |
| `Ask`, `Observe` | `forbidden` | No rule lets the calling program ask (ASK-R1). |
| `Ask`, `Observe` | `unavailable` | The rights decision or the profile store could not be reached (ASK-R1, ASK-P1). |
| `ListQuestions` | `page` | One page of pending and answered metadata (ASK-O2). |
| `ListQuestions` | `gap` | The cursor's book changed, restarted or does not match the caller's subject (ASK-O2). |
| `ListQuestions` | `invalid` | `cursor` or `limit` is out of bounds (ASK-O2). |
| `ListQuestions` | `forbidden` | No operator authorization for the calling program (ASK-O1). |
| `ListQuestions` | `unavailable` | Storage failure; never an empty successful history (ASK-O2). |
| `AnswerQuestion` | `answered` | The recorded choice, replayed on a repeat (ASK-O3). |
| `AnswerQuestion` | `conflict` | Another valid option was already recorded (ASK-O3). |
| `AnswerQuestion` | `unknown` | No retained question for this id (ASK-O3). |
| `AnswerQuestion` | `invalid` | The id or option is malformed, or the option is not in the catalog (ASK-O3). |
| `AnswerQuestion` | `forbidden` | No operator authorization for the calling program (ASK-O1). |
| `AnswerQuestion` | `unavailable` | Storage failure; the call establishes no decision (ASK-O3). |
| `DismissQuestion` | `dismissed` | Removed; the question's last metadata is returned (ASK-O4). |
| `DismissQuestion` | `unknown` | No retained or dismissed question for this id (ASK-O4). |
| `DismissQuestion` | `invalid` | The id is malformed (ASK-O4). |
| `DismissQuestion` | `forbidden` | No operator authorization for the calling program (ASK-O1). |
| `DismissQuestion` | `unavailable` | Storage failure; the call establishes no dismissal (ASK-O4). |

## Bounds

| what | bound | rule |
| --- | --- | --- |
| slot value | at most 200 Unicode characters, no control character | ASK-A2 |
| idempotency key (`request_key`) | 1..128 UTF-8 bytes, no control character | ASK-A2 |
| `Observe` wait | 0..30000 ms | ASK-W1 |
| Go client default call budget | 5 seconds | ASK-W3 |
| Go client `WithTimeout` budget | up to 35 seconds | ASK-W3 |
| simultaneous host calls | at most 32, each bounded to 35 seconds | ASK-W3 |
| application-profile file | at most 4 MiB | ASK-P1 |
| admissions and retained records | at most 4096 each | ASK-P2 |
| `ListQuestions` page | 1..64 records, at most 256 KiB encoded, within the shared 1 MiB frame | ASK-O2 |
| `ListQuestions` cursor | at most 256 UTF-8 bytes | ASK-O2 |
| `AnswerQuestion`, `DismissQuestion` id and option | 1..128 UTF-8 bytes, no control character | ASK-O3, ASK-O4 |

## Divergences

None declared for this release.

## Not built

- `DismissQuestion` is the prose name this release; the wire method stays
  `RetireQuestion` and the outcome stays `retired` until `operator@2`
  (`research/vocabulary/RENAME-PLAN.md` §4, third release).
- `unavailable` here carries no `INF-A2`-style reason family (`lease:<outcome>`,
  `engine:<name>`); a caller distinguishes a storage failure from a capacity
  refusal from diagnostic text only, not a structured reason.
- Only the Go client is implemented and tested this release; the C++,
  JavaScript, Python and Rust packages under this repository carry build
  manifests only, no implementation.
- Verified only on Windows 11 over a named pipe; Linux and macOS are not
  examined (the unix socket listener is shared with `rights` and has never
  run there either).
- No conformance scenario corpus cites this layer. The tests in `go/` send
  every operation with a valid credential and no identity and expect five
  refusals, covering the peer-boundary case only.
