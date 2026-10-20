# abstraction.asks contract

Remote first-use questions belong to the deciding receiving runtime and its
exact remote subject. The receiving person answers exact receiver consent;
remote origin claims and assistant answerers grant no permission. Held by:
`serve/messages_remote_windows_test.go` (`TestRemoteSendAsksReceivingPerson`).

Machine-pairing native occurrence admission, exact agreement binding and answer
eligibility are owned by Facade's [native consent and activation](https://github.com/openabstractions/abstraction-facade/blob/main/CONTRACT.md#native-consent-and-activation).
The runtime-bound `runtime.machine.pair` application occurrence uses the existing
Book and operator-answer rules; its payload and ID establish no authority.

Binds: `asks.thrift`

`asks.thrift` defines `abstraction.asks/application@1`, which admits and
observes fixed-catalogue questions on a caller's own subject;
`abstraction.asks/application@2`, which admits application-authored ordinary
choices with explicit pending and replay deadlines; and
[`abstraction.asks/operator@1`](https://openabstractions.github.io/reference.html#concept-asks.question-operator), which lets an authorized operator list,
answer and dismiss each kind through separate methods and closed records.
`abstraction.asks/responder@1` exposes only bounded
listing and answering of routine first-use questions, under the person's
enrollment of the answerer (ASK-R5) or an exact, expiring rights delegation
(ASK-R3), and of the options applications share in ordinary choices, under
that same enrollment (ASK-A12, ASK-R9).
`abstraction.asks/permission@1` admits caller-bound typed permission requests;
`abstraction.asks/permission-operator@1` lists, reads and conditionally answers
those occurrences. Rights `operator@2` separately commits their complete effect.
`abstraction.asks/notification@1` admits a caller's one-way notification whose
actions the service holds after the caller exits, and
`abstraction.asks/notification-operator@1` lets an authorized presenter list,
act on, dismiss and audit them (ASK-N1..N7).
The service owns the fixed catalogue's text, options and short display summaries.
Go `Question.Summary(about)` uses explicit catalogue templates and only the
display slots `{folder.leaf}` and `{host}`; unsafe or absent values use generic
wording. It returns plain text, carries no authority and stores no new field.
Full prompts and trusted richer permission/registration projections retain
their existing owners. Ordinary choices
carry the application's title and stable offered IDs. The service
requires an explicit `LoadApplicationBook` provider and shared [Program](https://openabstractions.github.io/reference.html#concept-identity.listener-program-need) peer
proof, and its application client creates no provider files. Both services
are served through the same versioned application-profile [Book](https://openabstractions.github.io/reference.html#concept-asks.book), reached
from generated `QuestionApplication`, `ChoiceApplication` and
`QuestionOperator` clients.
[README.md](README.md) is the walkthrough: obtaining the module, the fixed
question catalogue, and a worked `Ask` and `Observe` call.

## Client checks and presentation ownership

| Surface | Owner and client check |
|---|---|
| Permission Request/Observe | Generated method postconditions enforce answer presence exactly for pending/answered. Clients correlate outcome, answer and policy revision; a pending or answered observation grants nothing. |
| Keys and IDs | The published component projection refuses adding bounds in this window. Compatibility readers retain the existing 1..128 UTF-8/no-control guard, with test-only comparison to the owning implementation. The next owning window carries these declared bounds. |
| Authority display | The pure `go/typedpermission` owner verifies effect digests, immutable occurrence/account binding, native attribution and catalogue presentation. Server and presenters supply Identity's actual native/remote validators; missing composition refuses. |
| Catalogue | `go/catalogue` owns the existing question table and rendering. Native evidence and Book state remain receiving responsibilities. |
| Retained choice/notification replies | The existing immutable record, publisher, answerer and state/payload consistency checks share the pure presentation owner. They read no live eligibility, grant no authority and record no answer. Request admission remains with the receiving service. |
| Deprecated reader helpers | Deprecated `client.ReadAskerContext`, `ReadDetailsContext`, `ReadContentContext` and `ReadPermissionDetailsContext` preserve their published behavior as thin forwards. Their replacements are the explicit `typedpermission` readers with native validator composition. New native callers use those replacements. The removal set is W2-02b under CONV-V1. |

## Reading this page

The conventions every OA contract carries are stated once in [CONVENTIONS.md](https://github.com/openabstractions/abstractions/blob/main/idl/CONVENTIONS.md); a rule here that applies one cites its `CONV-` id.

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
| N | notifications — posting, actions, expiry, dismissal and suppression |
| X | extension — reserved, unused |

| Prose | Wire (until its own release) |
| --- | --- |
| dismiss, dismissed | `RetireQuestion`, outcome `retired`, until `operator@2` (`research/vocabulary/RENAME-PLAN.md` §4, third release) |

## Admission

**[ASK-A1] The subject.** For new native admissions the service MUST derive
the receiving account and canonical Program through identity.SubjectOf,
never from wire claims. Its independently proved absolute executable path
remains an observation. Canonical ownership MUST retain explicit native
account/Program provenance. Before a new admission, the same Book transaction
MUST check canonical replay and prior path-scope replay for that exact current native
executable. Matching replay returns its original occurrence and outcome;
changed intent or conflicting distinct occurrences refuses. Existing path-scope
equations remain byte-compatible. Old records without canonical provenance
MUST NOT acquire a package family from present-day observations or transfer
history across unproven aliases or package versions. Trusted Go hosts compose
the additive native Book view from independently obtained native facts; that
composition API supplies no authentication or authority.

**[ASK-A2] The question and its slots.** `Ask` MUST refuse an unknown
question id. Slots MUST exactly match the question's declared placeholders
excluding `asker`. Each slot value MUST be valid UTF-8, MUST contain no
control character, and MUST NOT exceed 200 Unicode characters. The
idempotency key (the wire's `request_key`) MUST be 1..128 UTF-8 bytes
without control characters. The display name shown to a person is derived
from the bound executable, never supplied by the caller. A question the
application service admits MUST keep that bound program, the clean absolute
path observed alongside the ASK-A1 subject, and its `asker` MUST be the program's
file name derived from it. Two programs with one file name stay two askers:
their records keep different programs. Only an authorized operator reads
the program (ASK-O11); the application and responder services return the
display name alone.

**[ASK-A3] Idempotent admission.** The service MUST store the subject, the
idempotency key and a canonical full-question fingerprint atomically with
the question. Repeating that key and content MUST return the original
question, including an answer kept as `once`, across a service restart,
until a pending question expires (ASK-A7).
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

**[ASK-A6] The requester's publisher is recorded at admission.** A service
that admits a question MUST record the requester's publisher with it: the
code signer's subject and whether the platform trust policy accepted the
signature. The application service takes both from the calling peer's code
binding. The runtime takes them for its own first-use question (ASK-R2)
from the code binding of the peer the refused caller's route bound, and that
peer must bind the decided subject. A job submit, a model lookup, an
inference call and a lend over the native IPC route each carry that peer to
the decision. An inference job carries its native submitter's peer to the
decisions of its admission and its execution. A job resumed after a
runtime restart carries none. The inference gateway window carries the peer it bound for
its loopback caller, code included. A decision that carries no such peer
records no publisher, and a prompt reads it as not verified. A remote
caller is a certificate mapped to a subject, and its questions record no
publisher.
The file at the program path is never the evidence: the caller can place a
signed file there for the moment of its refusal. Only the service sets
these fields, and a caller supplies none. A replay or a shared question
keeps the evidence of its first admission. A subject over 256 UTF-8 bytes,
in invalid UTF-8, or containing a control or format character is not
recorded. With a trusted signature the service also records the program's
signed version resource from the same code binding: its description
(`FileDescription`) and its original file name (`OriginalFilename`), which
identity reads from the verified image on Windows. Each follows the
subject's bounds, and neither is recorded beside a signature the platform
did not accept. The state file carries these as optional members; an earlier
binary refuses a file that carries them as an unknown field (ASK-P1).

**[ASK-A7] A pending question expires.** A question still pending 15
minutes after it was asked MUST expire; a host MAY configure another
positive lifetime. The lifetime is wall-clock time from the question's
recorded `asked` time, and a sleeping machine or a stopped service counts
toward it. An expired question is no longer retained. It is removed
with every admission naming it, and every call reads it as absent: `Observe`
and an `Ask` replay find no admission, `ListQuestions` and `ListAnswerable`
omit it, and `ReadAttributedQuestion`, `AnswerQuestion`, `AnswerQuestionIfPending`,
`DismissQuestion` and the responder's `Answer` report `unknown`. Its
idempotency key admits a new question on the next `Ask`, so a requester
still waiting is asked again. An answered question never expires. Expiry
records no answer and writes no rule. An operator UI MUST stop presenting a
question once it expires, and it learns that from its re-read (ASK-O6).
Expiry uses `unknown` and leaves no tombstone because `gone` means a person
dismissed the question and its key never admits again (ASK-A4). A tombstone
per expiry would also silence the runtime's first-use question after 16
expiries and spend the 4096-admission bound (ASK-P2).

The default follows the consent requests shaped like this one, where the
requester is refused at once and asks again later. RFC 8628 device
authorization requires an `expires_in` for every request, and its example
is 1800 seconds (<https://www.rfc-editor.org/rfc/rfc8628>, §3.2). GitHub
defaults it to 900 seconds
(<https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps>).
Microsoft Entra gives the user 15 minutes by default
(<https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-device-code>).
OpenID CIBA requires `expires_in` too, with an example of 120 seconds
(<https://openid.net/specs/openid-client-initiated-backchannel-authentication-core-1_0.html>,
§7.3). After expiry each answers `expired_token`, and the client starts a
new request, which is the behavior this rule gives a requester. In-context
prompts end with the request that raised them and carry no clock of their
own. Chromium cancels a pending permission request when its page navigates
or its tab closes and records it as ignored
(`components/permissions/permission_request_manager.cc`). Apple's
`AVCaptureDevice.requestAccess` documents no timeout; the completion
handler runs when the person answers. Android's `requestPermissions`
documents no expiry, and the platform may deny automatically
(<https://developer.android.com/training/permissions/requesting>). A
Windows UAC consent prompt closes without elevation after a timeout; only
community reports state it, at two to five minutes, and no Microsoft
documentation of the value was found. An OA question outlives its request,
since the requester was refused at once, so it takes the out-of-band
default: 15 minutes, the value two deployed device-flow services chose.

**[ASK-A8] A script host's question says its publisher covers the host.**
A signed script host verifies as its publisher's program, and the code that
asks is the script it runs. Identity keeps the one reviewed list of shared
interpreters keyed on the recorded original file name, compared without case
(identity ID-I1, `SharedInterpreterName`); this package reads it through
`ScriptHostSubject` and keeps the sentence. The exported `ScriptHost` is
deprecated and is removed in the asks application window (W2-02a). An
operator UI that shows a question whose
recorded program `ScriptHostSubject` reads as a shared interpreter, with a
trusted publisher, MUST add the one sentence `ScriptHostSentence` gives,
"<host> runs scripts; its publisher covers <host> and the script it runs is
unverified.", where <host> is the recorded description, else the original
file name, else the program's file name. The sentence, the prompt's level and
its icon read that one rule: a program that runs foreign code without being
a shared interpreter, such as stock OpenCode, gets its publisher's level, its
own icon and no sentence (TD83). An application's own interpreter copy in
its own environment is that program's interpreter and follows the same rule:
its publisher's level, its own icon and no sentence. A UI that shortens a
long description for display names the host in the sentence the same way.
It MUST take the list from this package. Windows attributes trust in a script to the
script's own signature (PowerShell execution policy, Windows Script Host,
AppLocker and WDAC script rules), which no code binding of the host reads
(VISION 2026-10-01).

An operator UI that colors its native prompt by level MUST choose the level
from the service's record of the request alone. The first matching row
decides: orange for an administrative first-use tier (ASK-R6), a selector
over every item of a collection or a script host on this list, whoever asks;
blue for OpenAbstractions' own program asking through the Book; gray for a
trusted publisher (ASK-A6); and yellow otherwise. Blue never marks a program other
processes can drive, such as the runtime's command-line programs, which are
judged like any other requester. Question text, option
labels and application titles select no level. The level is presentation
and leaves tier, eligibility and authority with their owners.
The native question style renders only Book occurrences.

**Published codecs retain service-owned bounds.** Published `application@1`,
`application@2`, `permission@1` and `notification@1` keep their original codec
language. Their receivers and Book retain the documented semantic bounds on
request keys, `RequestRef`, observation waits, option and action ids,
notification actions, tag, operation id and permission purpose. For an
otherwise admitted caller, codec-valid values outside those bounds keep the
method's `invalid` outcome and admit no question or notification. Structural
codec failures retain their declared decode errors.

A first-added or narrower generated bound on a published member or argument
requires the next service version (CONV-V1, DEF-V1). Declared constraints on
new shapes follow DEF-A11's generated reader/writer refusal before a handler
or transport; they do not retrofit an early `bad_value` onto these published
profiles.

## Waiting

A waiting or slow call never delays another caller, app, service, channel or
connection. This includes calls sharing one helper, plugin, connector or IPC
connection. A connection carries concurrent calls without head-of-line
blocking. The IPC transport multiplexes requests by id, or the client opens
one connection per in-flight call. Helpers, plugins and connectors serve tool
calls concurrently. A call may wait for its
own reply within the caller's wait_ms. No lock is held across a wait, an
upstream call, disk or network I/O, or a person's answer. A waiter costs no
CPU while it waits (WATCH-W3). Admission runs once per wake and per call.
Per-caller limits bound waits and queue use.

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
removal scheduled) and MUST be owned by one configured host. Its current
version MUST be `asks-application@12`. Canonical application versions 1 to 11
MUST remain readable under the retained field/decoding rules in
[HISTORY.md](HISTORY.md#application-profile-field-history); they migrate on
their next edit. No migration from deprecated native state occurs, and an
older reader MUST refuse a later profile before mutation.

The current format retains bounded native ownership provenance on new
application, choice and notification admissions. Rows from previous versions
retain their scope, IDs, outcomes and operator access; replay through the exact
current executable MUST NOT relabel them. Native provenance counts toward each
existing encoded partition/file bound; no capacity or retention bound increases.
Program fingerprints retained with admissions and acknowledged choices remain
governed by ASK-P5. Typed permission data MUST use `rights.permission@2`, retain
its complete effect/digest/account/trusted display and its closed choice
projection (ASK-O10), and carry no deprecated `PermissionRequest`. A typed responder answer
retains its exact nonempty answering program under the finite routine-effect
guard below; migration never changes that actor into an operator.

Canonical provider JSON is REQUIRED. An unknown version, an unknown field,
noncanonical or invalid Unicode, a nonregular node, or a file over
4 MiB + 1 MiB + 256 KiB + 1 MiB + 328 KiB + 4096 bytes MUST be refused. The
legacy, ordinary, answer-audit and notification encoded partitions have
independent bounds (ASK-P2, ASK-P4, ASK-O15, ASK-N6); the program fingerprints
of `application@1` and `permission@1` admissions, 82 bytes each, have the 328 KiB,
and that partition is measured without them and without the version label.

**[ASK-P2] Capacity.** The legacy partition MUST accept at most 4096 admissions
and 4096 retained records. Once either is exhausted, the service MUST
refuse new admission while every existing key remains readable.

**[ASK-P5] One program's share.** A program MUST NOT hold more than an eighth
of a partition (`ProgramShareDivisor`), so one program that asks without
end cannot leave every other application reading `exhausted` (DREAM §14).
A program's share of the `application@1` and `permission@1` partition is 512 admissions and
512 retained records, counted against the program a record concerns: the
target of a runtime first-use question, the subject of a typed request, or
the bound asker of an `application@1` question, and the admitting scope when
none is recorded. Its share of the ordinary partition is 128 admissions and
128 KiB encoded, counted against the bound requester. A question's
admission that outlives its record and an acknowledged choice keep the
fingerprint of that program (`program_hash`) and stay in its share until
they are collected, so a program whose occurrences are decided,
acknowledged or removed still reads `exhausted` at its share. An admission past the
share reads `exhausted` exactly as a full partition does, and writes
nothing; every admitted key still replays, and another program is admitted.
The partition bounds (ASK-P2, ASK-P4) still hold for all programs together.
Notifications keep their own per-program rate (ASK-N6). An eighth follows
ASK-N6's choice of a per-program bound beside a shared one; a rolling-hour
rate for questions is not set, since an ordinary choice is retained for at
most seven days and a pending `application@1` question expires in 15 minutes (ASK-A7), which
already returns a share. Tests: `TestOneProgramCannotExhaustOrdinaryChoices`,
`TestOneProgramCannotExhaustQuestionsOrTypedRequests`,
`TestAcknowledgedChoicesStayInTheAskersShare`,
`TestRetiredQuestionsStayInTheProgramsShare`,
`TestProgramHashesMigrateFromVersion9`.

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
`abstraction.asks/question.ask` on the resource `account`,
`abstraction.asks/question.answer` on the exact delegated answer scope
(ASK-R3) or on the class scope `apps` (ASK-R5), and
`abstraction.asks/notification.post` on `account` (ASK-N1, ASK-N6). A host that
decides application `Ask` MUST configure `EnableAskPolicy`; the policy
receives the rechecked typed peer and the question id before admission. An
evaluated refusal is `forbidden`, and a decision that cannot be obtained is
`unavailable`; neither MUST admit, replay or record anything. `Observe` of a
caller's own admissions stays scoped by subject. With no rule configured, an
application MUST NOT be able to ask (DREAM §14). The installed runtime
configures the account-wide `ask` default (rights RIGHTS-R22, VISION
2026-10-07): a program of the runtime's own account with no `question.ask`
rule of its own may ask while the switch is on. A Block on that program, a
script host, a remote peer, or the switch turned off refuses it; the
person's *May ask: never* is that Block.

**[ASK-R2] The runtime's own first-use question.** `rights.first_use` is
the question a runtime admits as itself when a first-use action it gates
decides `not_granted`. The runtime's first-use actions are its spending
actions and lending into an engine's directory (VISION 2026-09-22), and the
enrollment, `abstraction.asks/question.answer` on `apps` (ASK-R10). The trusted
runtime MUST pin a `PermissionRequest` in the occurrence at admission. Each
target carries the receiving account, normalized subject program, action and
resource, bounded by rights' 128/4096/128/1024 UTF-8 byte limits. It contains
one primary target, or only the existing deterministic `content.write` plus
`content.read` pair on the same account, program and input/output profile.
An independent secondary target MUST be refused. Its About and text retain a
complete legacy presentation projection; authority MUST come from the pinned
target, never from that projection or shortened display text. Only canonical
version 1/2 records use `FirstUse` to recover their historical tuple, preserving
its program spelling. Public application admission MUST refuse this question id
even without a configured ask policy. The answer grants nothing by itself: an operator program
writes the permit rule on `allow` and an exact deny rule on `never`;
`refuse` writes nothing. An operator Allow is applied through rights'
`ApplyFirstUseAllow`, which derives the complete rule from the actual Book
occurrence and commits durable completion with its permits (RIGHTS-O10).

An answered question replays for a later refusal of the same program,
action and resource, with two exceptions, where the runtime MUST release
the answered question, removing it and its admission without a tombstone
as expiry does (ASK-A7), and admit the question again under the same key.
An `allow` whose delegated effect was `applied` (ASK-R4), or whose operator
effect has rights-owned durable completion (RIGHTS-O10), is asked again,
since a refusal means its permit no longer holds, revoked or expired. An
`allow` whose effect conflicted or failed keeps its record, as does an
operator's `allow` whose rule has never completed. Its failed write remains
available for explicit Retry. Historical unmarked operator Allows retain
Retry/Dismiss behavior; completion is never inferred from matching rules or
descriptive reasons. A `refuse` means "not this time"
and is asked again once 10 minutes have passed since it was answered, so a
person is asked at most that often per program and question. A `never`
stands as its deny rule and is never asked again. The refusal that leads to
a release may have been decided before an `allow` landed, so the runtime
MUST decide again just before releasing and release only on a current
`not_granted`. Release MUST condition on the observed exact question ID,
so a replacement answer under the same key cannot be removed. A paired input or output question's `allow` wrote both
`content.write` and `content.read`, so both are decided: it is released
when either reads `not_granted` and neither is denied or unavailable.

**[ASK-R3] Delegated first-use answering is exact and expiring.** A runtime
MAY configure `QuestionResponder` with trusted service callbacks. Only a
same-account Program peer with a current exact permit for
`abstraction.asks/question.answer`, or an enrollment (ASK-R5), may see or
choose an option, and only on a routine question (ASK-R6) while it runs no
foreign code (ASK-R7). The resource
is `FirstUseAnswerResource(program, action, resource, option)`. For every
previously representable tuple it preserves the exact `first-use@1:` bytes:
compact `encoding/json` encoding of those four strings in order, each program
and resource at most 200 runes/800 bytes and the whole key at most 1024 bytes.
Otherwise, after validating the complete rights target and the allow/refuse
option, it selects `first-use@2:` plus the lowercase SHA-256 hex digest of the
same JSON tuple with the shared normalized program. There is exactly one
canonical resource per tuple; the service MUST NOT read an alternate key or
fall back after denial, expiry or revocation. A digest for a short, representable
tuple names an unused opaque rights resource. The digest covers only the primary
tuple because the secondary effect, when present, is uniquely fixed by it;
every target and responder MUST bind the receiving runtime account.
The runtime derives every component
from a retained `rights.first_use` record whose admission scope is its own
within the service-owned Book's integrity boundary (ASK-P3),
and MUST reject any other question, target, action or option. The only target
actions are the runtime's configured first-use actions. The caller
cannot claim a target or grant itself a delegation. A party to the question
MUST read `forbidden` on `Answer` and MUST NOT see the question in
`ListAnswerable`, whatever its permit names. The parties are the target
program; the asking program, which is the runtime and, when the admission
bound one, the verified peer the record keeps in `via`; a program whose
trusted code signer is the target's trusted publisher as recorded with the
question (ASK-A6); and a program whose executable is the target's file under
another path (the same volume and file ID on Windows, the same device and
inode elsewhere). Programs compare by the shared subject-program rules, so
another letter case or an 8.3 short name is the same program. The publisher
and file checks cover what one vendor controls beside its app: a signed
helper it ships and a hard link to the app's own executable. A host adds the
parties its current trusted relationships name through
`AnswerCheck.PartyWithRelated`, compared by the same rules. The OA runtime
names each enrolled agent application that holds a current
`abstraction.facade/application.message` permit to the target's agent
application, or that the target holds one to: paired assistants never answer
each other's questions, under an exact delegation or an enrollment, and the
question stays with the person. Ending the permit in both directions ends the
relationship for the next answer. Relationships come only from the host's
current permits and enrollments, and metadata the host cannot read refuses.
An operator sets the
responder's exact rule with `SetRuleFor` and a nonzero expiry; revocation and
expiry take effect at the next decision. This check rejects an application
that tries to forge `rights.first_use` through OA IPC. Like the rest of this
same-account profile, it does not prevent a hostile peer with direct write
access to the runtime's state directory from tampering with the Book or
rights file; OS isolation is a separate boundary (RIGHTS-R15).

**[ASK-R4] Answer and resource permission are separate commits.** A first
delegated `allow` records the choice, then attempts one exact target permit
through the current rights policy revision. The policy edit MUST recheck
caller liveness and delegation expiry inside its atomic edit. A concurrent
rights change MUST conflict, and no stale grant may land. Under an exact
delegation the target permit MUST expire within five minutes and before the
delegation expires, computed again at the rights edit time after lock
waiting. Under an enrollment the edit is the rights service's
`ChangeRulesUnderAnswer` (RIGHTS-R16): it reads the enrollment inside the
same atomic edit, renews it, and writes the permit `set_by` the answerer,
expiring at the enrollment's stamped expiry. A paired first-use answer
writes both action rules in one rights edit; a conflict on either leaves
neither new rule, under an exact delegation or an enrollment.
A rule on the target that someone else set stays, and a deny there reads
`conflict`. A first `refuse` under an enrollment writes no target rule and
renews the enrollment the same way. `refuse`
records the choice without a target rule. The response MUST distinguish the
recorded answer from an applied, conflicted, forbidden or unavailable target
effect and report the current target decision when obtainable. A same-option
replay preserves the Book answer and MUST NOT write target policy; a partial
effect requires explicit operator reconciliation. A different option with
its own current delegation conflicts; without that delegation it is
forbidden. The application MUST retry its original refused operation: only
that operation's fresh rights decision permits it.

**[ASK-R5] An enrolled answerer answers every app's routine questions.**
The person enrolls an answerer with one yes (VISION 2026-10-03, "The
0.4.0 rights slice"): an operator writes a permit for
`abstraction.asks/question.answer` on the resource `apps`, subject the
answerer program, expiring 30 days after the edit (RIGHTS-R16). The yes is
given in the operator UI, worded "Let <program> answer OpenAbstractions
questions for you" with a warning naming what it answers and what stays with
the person (research/rights-model/PROPOSAL.md §8.1), or in the native
permission prompt when the answerer asks for the right like any other
(ASK-R10). The operator UI MUST present it only to the person. No answerer
can answer it: the operator UI's yes is no question in the Book, the
prompt's question is administrative (ASK-R6), and only an operator writes
the rule. Each applied allow or accepted
refusal renews the enrollment; a conflicting effect renews nothing. It
lapses after 30 days without an accepted answer. Stop
removes the rule and ends future answers; checking the optional cleanup also
removes current permits whose `set_by` is the answerer in the same rights
edit. The default leaves existing permits at their stamped expiry (RIGHTS-R16).
The answerer's next call reads
`forbidden`, and the questions come to the person. The service MUST check,
on every `ListAnswerable` record and option and inside the edit of every
`Answer`, before writing any rule: the enrollment is a current permit; the
question is the runtime's own first-use question; its tier is routine
(ASK-R6); the answerer is no party to it (ASK-R3's parties); and the
answerer runs no foreign code (ASK-R7). A check that cannot be made refuses.
Question text and program names come from applications and decide nothing
here: the service computes the tier from the question's action and
resource. An answerer answers only its own runtime's questions. The same
enrollment also answers the options applications share in ordinary choices
(ASK-R9), and the warning names them, says that an answer to an app choice
cannot be undone, and states ASK-R9's known limit.

**[ASK-R6] A first-use question's tier comes from one table.** A
first-use question takes the tier of the permission it decides, from
`grants.FirstUseTier` (abstraction-facade `go/grants/tiers.go`), the one
table the runtime and the operator UI both read. Routine: `complete` on a
model server OpenAbstractions runs on this machine: a native provider
reached over authenticated local IPC whose declaration has OpenAbstractions
product standing (placed by the installation in the per-machine folder, or
naming a program that passes `product.Shipped`). A product's observed
server, a registration naming a program without product standing, and
every HTTP endpoint, loopback included, are not such a server, since a local listener
can forward a call to a paid hosted service. Also routine: `lookup` on a registry the runtime lists,
`acceptance.submit` for `job:inference`, and `content.write` or
`content.read` on an output profile; and application `command.read`,
`command.reveal` and `command.propose` on one exact `app:<name>`: read what
is open in another app and suggest changes there. This finite application
effect discloses all three operations and requires enrollment for responder
answering; exact delegation supplies no fallback. Sensitive: `lend`, and
`acceptance.submit` for `job:download`. Administrative: `complete` on
any other server: a hosted one, one the runtime does not list, and one on
this machine that OpenAbstractions does not run. Any permission of
`abstraction.asks` or `abstraction.rights` is administrative too. Every other first-use permission
is sensitive. Only a routine question is answerable by an answerer under
any grant, an exact delegation included; every other is refused to every
answerer and stays with the person.

**[ASK-R7] An answerer is a program that runs no foreign code.** The
answerer is the subject program the service binds from the answering peer:
for OpenCode, OpenCode's own executable, which loads the OA client into its
process. This package owns the answerer rule (`AnswererRule`, asks P0-2): a
host composes it from its account, its own program and its operator
programs, and keeps only the rights reads (a current enrollment, an exact
delegation) and whether the question is routine (ASK-R6). `AnswererRule.Answerer` refuses another account,
an operator program and a program identity's `ClassifyProgram` reads
`RunsForeignCode` for (identity ID-I1, TD83), since every script such a
program runs would answer: a shared interpreter, an application's own
interpreter copy, an Electron program whose `RunAsNode` fuse is on, a Bun
standalone executable whose runtime honours `BUN_BE_BUN`, and a macOS program
whose signature carries a foreign-code entitlement. Stock OpenCode is such a
Bun standalone executable, so it never answers for other applications,
whatever its enrollment. A host's enrollment refusal (rights RIGHTS-R16)
and an operator UI's assistant chooser read the same rule, so no enrollment
is written or offered for a program it always refuses.
The receiving runtime also composes complete, validated application declarations
through `AnswererWithDeclaredHosts`: a declared script host is a conservative
veto, including its retained native file aliases. A false declaration grants
nothing and cannot override native classification. Unavailable declaration
metadata refuses as unavailable. The published `AnswererRule` fields and
`Answerer` call remain compatible; the directory owns declaration availability.
`Host.EnableResponderPreparation` composes the outer metadata refresh after
native caller admission and before responder Book reads/edits. Every responder
entry prepares freshly, including lists, answers/replay, shared choices and
RequestAnswering. Nested authorizers and effects carry that operation context
and use atomic facts; failed preparation is unavailable. Existing nil-configured
hosts retain their behavior. Facade composes this through its existing RightsPolicy.
`AnswerCheck.Party`, or `PartyWithRelated` with the host's current related
parties, then applies ASK-R3's party rule.
Programs compare by the shared subject-program rules
(`identity.SameSubjectProgram`), and files by file identity
(`identity.SameFile`), never by their spelling; one check reads each
program's file identity once.

`ScriptHostSubject` is identity's `SharedInterpreter` for a rights subject,
with the recorded publisher as evidence. It decides rights' account-wide
default exclusion (RIGHTS-R22) and the operator UI's script-host level, icon
and sentence (ASK-A8), and every caller takes it from this package. A program that runs foreign code
without being a shared interpreter keeps its defaults. Identity reads, in
order: an `msix:<family>` subject by the reviewed package families
(`ScriptHostPackage`: PowerShell, Python, WSL and its distributions); the
`pyvenv.cfg` home; the signed original file name the answering peer's code
binding carries; the program's own file name on any platform after its short
8.3 alias is resolved (`ScriptHostFileName`: macOS and Linux shells,
`osascript` and interpreters, versioned names such as `python3.12` and
Python.app's bundle executable included); and the program file's own
verified signature, read once per file stamp (identity ID-I1), so a renamed signed copy is
named by its original file name. One shared table
(`scripthostcases`) runs through every caller's tests, its rows marking a
script host and a program that runs foreign code (stock OpenCode as a Bun
standalone executable, an Electron program with `RunAsNode` on), and the answerer rule's
table (`TestAnswererRuleTable`) runs without a runtime. An unsigned,
renamed interpreter on macOS or Linux carries no name this check reads; that
gap stays open until app registration reads a declared `script_host` role
(TODO, after 0.4.0). This is a boundary between cooperating programs of one
account (RIGHTS-R15).

**[ASK-S1] A system-access question is the person's, once, and writes no
rule.** `system.securitylog.read` is the question a runtime admits as itself,
with the requesting program as asker, when an owning service needs the
platform's administrator approval for one exact act (abstraction-logging
LOG-N1, abstraction-facade FAC-H2). Its options are `once` and `refuse`, with
the standard `never`. Public application admission MUST refuse this id even
without a configured ask policy. No answerer answers it under any grant: it is
no first-use question, so ASK-R3, ASK-R5 and ASK-R6 refuse every answerer, and
only the operator presents it (VISION 2026-10-02: an assistant never answers an
ask that leads to an administrator prompt). Its text names the act, says that
Windows asks for administrator approval next, and carries a request code taken
from the frozen request. The answer writes no Rights rule. `once` authorizes one
execution of that frozen request by the installed helper, claimed within 120
seconds of the answer (FAC-H2); the operator presenter raises the platform
prompt right after recording it (FAC-H3). `refuse` and `never` authorize
nothing. The runtime releases the answered question, as expiry does (ASK-A7),
once its execution is delivered, failed or expired, and the next request admits
a new one; a `refuse` is asked again after 10 minutes, as ASK-R2; a `never`
stands. A replayed answer never authorizes a second execution. Its ticket
lives in the admitting runtime's memory: the catalogue marks the question
`RuntimeBound`, and a runtime MUST call `Book.WithdrawRuntimeBound` once it
owns its endpoints and before it serves any request, which retires every
pending one as DismissQuestion does, so a question from a previous runtime is
never answered after a restart. A runtime that fails to own its endpoints
withdraws nothing, leaving a live runtime's questions alone. The Panel's
Questions page and the `openabstractions questions answer` command accept
`refuse` and `never` and leave `once` to the native prompt, which raises the
administrator prompt right after it. Tests:
`TestSystemAccessQuestionIsOnceAndRuntimeOnly` (go),
`TestProtectedReadAdmitsOnceAndDeliversOnce`, `TestProtectedReadRefusals`
(serve), `TestQuestionsPageLeavesSystemAccessOnceToThePrompt` (monitor),
`TestQuestionsSystemAccessOnceStaysWithThePrompt` (serve),
`TestWithdrawRuntimeBoundAtStartup` (go),
`TestProtectedQuestionDoesNotOutliveItsRuntime`,
`TestProtectedOnceDoesNotSurviveARestart`,
`TestRuntimeWithoutItsEndpointsWithdrawsNothing` (serve).

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

**[ASK-O6] A presented question is re-read before its answer.** An operator
UI that presents a question in its own prompt MUST show it only in the
person's unlocked interactive session. A prompt interrupted by a session lock
sends nothing, and the question is presented again after unlock. Before
`AnswerQuestion`, the UI MUST re-read the question through `ListQuestions`.
It MUST NOT send the answer when the question is absent, no longer pending,
or differs in any listed field from what it showed. It then reports
`unknown` for an absent question and `conflict` otherwise, the outcomes
`AnswerQuestion` gives for those states (ASK-O3). The UI MUST send the answer
through `AnswerQuestionIfPending` (ASK-O8), so an answer that lands elsewhere
between the re-read and the answer is `conflict` for any option. The
requesting application's outcomes
are unchanged. It observes `pending` until an answer lands and retries its
refused operation. A question that expires while its prompt is open
(ASK-A7) is absent from the re-read, and the UI closes the prompt without
sending anything.

**[ASK-O14] Presentation by ask type.** An operator UI that presents
`operator@1` questions in its own prompt MUST show each question in its own
dialog above the person's other windows: on Windows the native renderer for
its declared family, running OS
version and system theme, on macOS a non-activating panel at
the modal-panel window level on every Space and beside full-screen windows.
Asks owns semantic presentation selection. Every catalogue question and typed
permission MUST declare its intended native presentation family; a missing
declaration or optional display payload MUST NOT silently select another
family. Windows retains its TaskDialog renderer for legacy OS generations and
genuine viewport fallback. Ordinary application choices remain application
owned, and OS elevation remains OS owned. This classification changes no
admission, responder eligibility or permission effect. OS-like consent is
reserved for owning-service-admitted runtime-only and typed permission
occurrences. The application receiver MUST reject runtime-only keys before
admission. Application-callable catalogue records (including approval wording)
and ordinary application choices MUST NOT select OS-like consent or automatic
native popups; they retain application UI or explicit ordinary manual review.
The native question entry point takes a retained Book occurrence (its id,
its retained content and its presentation family) and no free text. The
operator UI's own confirmations, reports and failures go to its window's
status line or to a notification. One operation has one confirmation: the
person's click in the operator UI for an operator-level decision, or the
owning service's question for a decision that is person-only by contract.
Question text names the apps and the act and shows no internal reason.
Showing the prompt MUST leave keyboard focus with the application in front,
and its answer controls arm only after it appears. A question the session
cannot show waits for unlock (ASK-O6). The platform's notification service
carries `notification@1` occurrences only (ASK-N7), under the person's own
notification settings. A prompt is consent shown to the person, not a
security boundary: a program running as the same account can drive the
operator UI's input or its routes, so no prompt proves that a person
answered. On Windows, a native prompt whose answer can grant authority (a
consent or pairing family, or a catalogue question with a yes option) counts
an answer button only after a mouse release over it, or an Enter, Space or
access-key press, that Windows did not flag as injected. An answer by window
message, UI Automation or injected input is ignored and logged with the
prompt's key and the input's source, and the prompt stays open. Code inside
the presenter, and assistive tools whose input Windows flags as injected,
are outside this check (`TestRealInputIgnoresProgrammaticAnswers`,
`TestRealInputAnswersFromThePerson`). An installer's application registration confirmation is such a
prompt; registering grants no permission, and the account default already
admits self-registration (RIGHTS-R22). Presence-verified answers belong to
the planned presence-verified grants.

The runtime-only `setup.models` occurrence uses the existing Setup question's
title, instruction and disclosures, rendered by Asks from closed native facts.
Only the authenticated native application.manage operator admits its frozen
planner facts; no responder answers it. Facts name model/plan references,
download/free bytes, licence names, fixed permission kinds and at most sixteen
target rows. They contain no caller-authored question prose. Each decoded string
MUST satisfy its field's control and size rules; unknown/duplicate JSON members
and extra rows refuse. Only Setup target rows use the existing 800-byte slot
carrier limit; other catalogue admission retains its existing 200-rune limit.
No public carrier bound changes. Book content and its existing operator reads
and answer history retain these facts under ASK-O16. Every native presenter and
history consumer renders that same catalogue body; absent or invalid content
permits no answer through a coarse substitute. Set up and Not now answer the
exact occurrence through `AnswerQuestionIfPending`. Display and answer confer
no install, registration, Keep or permission authority; those effects retain
their existing independent owners and exact-plan checks.
Tests: `TestSetupFactsStrictDecodedRows`,
`TestSetupFactsRetainedByBookAcrossNewOccurrenceAndRestart` (go),
`TestStepQuestionCataloguePreservesFrozenWording` (Panel Setup),
`TestSetupQuestionPresenterUsesRetainedFactsAndExactOccurrence` (Panel).

**[ASK-O7] `ReadAttributedQuestion`.** `ReadAttributedQuestion(id)` MUST
return one retained question's recorded metadata and attribution to an
authorized operator. `found` carries its metadata and the publisher recorded
under ASK-A6, with the program's recorded description and original file name
present only beside a trusted publisher. `unknown` means no retained question,
`invalid` a malformed id, and `forbidden` and `unavailable` follow ASK-O1.
Authorization MUST be rechecked before disclosure. A recorded `answered_by`
names `operator` or `responder`; only a responder answer carries its bound
program in `answerer`. Pending questions and older answers may lack that
route. Only `found` carries metadata, publisher/program evidence or answer
attribution. The service sets these fields; caller text supplies none.

This metadata-only method MUST remain readable when a legacy permission
target cannot be recovered. It grants no authority and makes no target claim.
The Book keeps the actor in its version 2 application profile. Version 1
canonical files migrate on their next edit; an older runtime refuses version 2.
A responder's answer carries its bounded permit (ASK-R4); an operator UI MUST
NOT turn it into a permanent rule through retry and offers no retry for it.
Publisher presentation MUST use recorded admission evidence. The method is
new since published 0.3.0; an earlier service returns `unknown_method`.

**[ASK-O8] `AnswerQuestionIfPending`.** `AnswerQuestionIfPending(id,
option)` MUST record the option only while the question is pending. It
applies ASK-O3's bounds, catalog check, atomic edit and authorization
rechecks. An answered question is `conflict` for every option, the same one
included, and carries no record. A question's id names content that never
changes (ASK-A3), so the pending state is the whole precondition the
presenter's view needs. `unknown`, `invalid`, `forbidden` and `unavailable`
follow ASK-O3. After an uncertain reply the caller MUST inspect history
before retrying, since an explicit retry of an answer that landed reads
`conflict`. `AnswerQuestion` keeps its same-option replay unchanged. The
call joined `operator@1` as a new method: a method argument cannot be
optional (idl/LANGUAGE.md, "Optional/default arguments ... remain refused"),
and every earlier receiver refuses an unknown argument field. An earlier
service answers the new method `unknown_method` and records nothing.

**[ASK-O11] `ReadQuestionAsker`.** `ReadQuestionAsker(id)` MUST return
trusted `QuestionDetails` and the bound asking program from one retained
occurrence. Details carry ASK-O7's metadata/attribution and, for a first-use
question, its complete immutable `PermissionRequest`. The target and program
are service-owned, and authorization MUST be rechecked before disclosure.
The receiving account scopes every target. An unrecoverable legacy target
MUST return `unavailable` with no details or program; metadata remains readable
through `ReadAttributedQuestion`. Typed permission occurrences use their own
permission-operator methods and MUST refuse this legacy read.

Only `found` carries a program; it may be absent for the runtime's own question
or an older admission. Its derived display name identifies the asker and
confers no authority. Presentation shortening changes no target. Before
answering, a presenter MUST reread the pending occurrence and compare the
complete target it displayed. The method is new since published 0.3.0; an
earlier service returns `unknown_method`. A client MAY select the known legacy
metadata/prose reader only after that explicit error. Missing/malformed current
details, `unavailable` and other failures MUST NOT take that fallback.

## Ordinary choices

**[ASK-A9] The application owns an ordinary choice.** `application@2`
MUST admit a caller-scoped `ChoiceRequest` through the same receiving-account
Program proof and authorization rechecks as ASK-A1. A configured ask policy
receives the fixed policy key `ordinary.choice`. The service MUST derive the
requester scope, display name and recorded publisher from the bound caller,
using ASK-A6's attribution bounds; the payload carries none of them. A title
and each label MUST be nonblank, single-line text of at most 80 Unicode
characters without controls. The request MUST offer 1..3 distinct IDs of
1..64 UTF-8 bytes without controls or blank IDs. One offered option is valid.
The application maps the selected stable ID to its own behavior. Admission,
answering and dismissal MUST convey no OA permission or configuration
authority. A resource service still enforces the authority for every resulting
request. `application@1`'s catalogue, closed records, retry promises and
ASK-A7 lifetime remain unchanged.

The application owns ordinary presentation. `Answer(request_ref,option,if_pending)`
and `Dismiss(request_ref)` MUST bind the requester through the same receiving-account
Program proof, derive its scope and actor, and recheck that binding inside the
atomic edit. Lookup MUST match both the owning requester scope and the entire
immutable reference inside that edit. Another program's reference or occurrence
ID MUST disclose no record and confer no answering authority. These methods
address only ordinary admissions; permission records remain under their existing
operator and responder rules. Own existing decisions and cleanup require ownership
and the occurrence deadlines; ask policy governs new admission and MUST NOT gate
these methods. No operator grant, enrollment or new right is required.

`Answer(...,true)` MUST conflict with every terminal outcome, including the same
option. `Answer(...,false)` MAY explicitly reconcile the same recorded option;
it MUST preserve its original actor and time. A changed option or dismissal
conflicts. An identical dismissal replays unchanged. Operator and requester
decisions MUST share one atomic decision owner: the first terminal outcome wins,
and each path rechecks the effective time floor, deadlines and offered option.
Only `answered` or `dismissed` carries a decision record. `answered_by` is the
proven answering program, whether requester or operator, and asserts no human
presence. Clients MUST validate the returned full reference, outcome, record state
and answer against their call. SDKs MUST NOT retry automatically or assume an
answer when an older service returns `unknown_method` for these additive methods.
A valid unexpired absent or foreign reference returns `unknown` without a record.
An elapsed replay deadline returns `expired` independently of lookup; an
acknowledged own reference returns `gone`; malformed input returns `invalid`.

**[ASK-A12] An application may share an ordinary choice with an assistant.**
`ShareChoice(request, assistant_options)` MUST admit or replay an ASK-A9
choice exactly as `Ask` does, under the same ask policy key, and record
`assistant_options`: 1..3 distinct IDs the request offers. An enrolled
assistant may choose only those options (ASK-R9); every other option and
dismissal stay with the person and the application. The request and
`assistant_options` together are the immutable content of the reference:
`Ask` and `ShareChoice` under one reference, or another option list, MUST
return `conflict`. The service MUST record the bound requester program beside
the record; a shared choice without one MUST be refused as `invalid`.
Sharing MUST convey no OA permission or configuration authority. `Ask` shares
nothing. An older service answers `unknown_method`; a client MUST then decide
explicitly whether to `Ask` unshared and MUST NOT assume an assistant can
answer.

**[ASK-W5] An ordinary occurrence has immutable absolute deadlines.** The
caller MUST create and retain the entire `RequestRef{key,replay_until_ms}`
once for an intent. Its key follows ASK-A2's request-key bounds. Both
deadlines MUST be positive Unix milliseconds; `pending_until_ms` MUST be
no later than `replay_until_ms`. A new admission requires a future pending
deadline and replay no more than seven days ahead of effective ordinary
service time. An elapsed pending or replay deadline returns `expired`.
The same full reference and identical content MUST replay the occurrence;
changed content MUST return `conflict`. The same key with a different replay
deadline names explicit new intent. SDKs MUST NOT create fresh deadlines or
retry automatically. `Observe(request_ref,wait_ms)` uses ASK-W1/W3's wait
and call budgets, rechecks binding and deadlines after waking, and cancellation
changes no decision. Pending expiry selects no option and remains terminal
until replay expiry. Answered or dismissed records remain observable until
acknowledgement or replay expiry.

**[ASK-P4] Ordinary retention and observed expiry are bounded.** The new
partition MUST enforce at most 1024 admissions, including acknowledgement
tombstones, and 1 MiB of encoded ordinary state, including its time floor.
The legacy 4 MiB partition MUST exclude ordinary records and metadata, so
legacy capacity alone cannot refuse an ordinary admission. Ordinary exhaustion
returns `exhausted`. Collection MUST touch only this new partition: it MUST
preserve legacy pending questions under ASK-A7 and legacy answered permissions,
unapplied Allows, admissions and delegated Undo evidence. Legacy answered
capacity remains subject to ASK-P2; this collector does not reclaim it.

`Acknowledge(request_ref,answer)` MUST compare the exact recorded occurrence
ID, option or absence, actor and answer time in the caller's scope. The caller
MUST acknowledge only once it no longer requires full answer replay from the
service and has retained the answer or result required by its own recovery
guarantees. Acknowledgement relinquishes that full service replay. A caller
whose crash or relaunch recovery still needs the answer MUST durably retain it
or leave the occurrence unacknowledged. A one-shot caller that has completed its
durable result and offers no same-intent recovery after restart MAY relinquish
replay without a separate local answer receipt. For any explicit retry after an
uncertain acknowledgement, the caller MUST retain and use the exact recorded
answer. A matching
acknowledgement replaces the full record and native admission evidence with
its immutable reference, request fingerprint, ID and decision hash, and the
bound requester's program fingerprint (ASK-P5). Identical
acknowledgements MUST replay after lost replies and restart. A different
answer returns `conflict`; subsequent Ask/Observe returns `gone` until the
replay deadline. Acknowledgement proves no application effect. At replay
expiry the service MAY collect the occurrence and tombstone. A late full
reference MUST still return `expired` and MUST never reopen.

Version 4 MUST keep one nonnegative, validated ordinary time floor. Effective
ordinary time is the maximum of current service UTC and this durably recorded
time. The first edit migrating an older profile initializes its nonnegative
floor from service time. Legacy edits retain their previous clock behavior;
an ordinary operation without a positive effective time is `unavailable`.
A successful expiry, collection or expired-reference refusal that depends
on a later time MUST commit that time before reporting success; storage failure
returns `unavailable`. Real admissions, decisions and acknowledgements also
retain their observed time. Unexpired observation and history reads MUST NOT
advance the floor solely to record a poll. A service-observed forward clock
jump can expire choices permanently; a later clock correction or restart
MUST NOT resurrect them. This rule changes no legacy clock or lifetime rule.
Versions 1..3 MUST NOT carry ordinary fields. Version 3/4 structured permission
validation and the receiving-account check remain mandatory, including pinned
permission reads; missing or malformed targets MUST NOT fall back to prose.

**[ASK-O9] Ordinary operator methods have separate closed results.**
Opt-in `operator@1` methods `ListChoices`, `ReadChoice`, `AnswerChoice` and
`DismissChoice` MUST use ASK-O1's explicit authorization and same-account
Program proof. Listing uses ASK-O2's page, cursor, frame and authorization
bounds, with a revision of ordinary content. Older permission methods MUST
exclude every ordinary record. An older service refuses these additive
methods with `unknown_method`. Exact first-use responder delegations (ASK-R3)
MUST NOT authorize ordinary answering. An enrollment (ASK-R5) answers only
shared choices, and only their shared options, through `responder@1`
(ASK-R9); an exact ordinary delegation is not implemented by this profile.

A human presenter MUST obtain the person's choice, re-read the exact immutable
pending record, and submit `AnswerChoice(id,option,true)`. The atomic edit
MUST recheck authority, binding, deadline and offered option after lock waiting.
Any terminal outcome races with this conditional answer as `conflict`.
`AnswerChoice(...,false)` MAY explicitly reconcile the same recorded answer:
it MUST preserve the original actor and time and MUST NOT change or reopen a
terminal outcome. A changed option or dismissal is `conflict`. Dismissal
records a terminal outcome with no option; repeating that exact dismissal
preserves its original attribution. Only `answered` or `dismissed` carries a
decision record. The service MUST capture `answered_by` as the bound answering program's
full valid subject program, rather than payload or display text. Clients MUST
validate that outcome, record state, option and attribution agree before
accepting a decision.

**[ASK-R9] An enrolled answerer answers shared ordinary choices.** The person's
one enrollment (ASK-R5) covers routine first-use questions and the choices
applications share (VISION 2026-10-07, "one yes covers both").
`responder@1` `ListChoices(cursor, limit)` MUST return only pending shared
choices the bound caller may answer now, each with only its shared options in
the order the application offered them, within ASK-O2's page, cursor and frame
bounds; cursors also bind the ordinary revision. `AnswerChoice(id, option)`
MUST record the option only when, inside the Book edit after lock waiting:
the choice is pending and before its deadline; the option is shared; the
trusted host check passes. The host check MUST require a same-account
Program peer with a current enrollment, refuse a program that runs foreign code (ASK-R7), an
operator program and the runtime, and refuse every party to the choice: the
requester's program, a program with the same file, a program whose trusted
code signer is the requester's recorded trusted publisher, the bound
peer the admission recorded, and the parties the host's current trusted
relationships name, as in ASK-R3: an assistant never answers a choice its
messaging pair shared. The parties compare by the shared subject-program
rules, as in ASK-R3. A check that cannot be made refuses. The same answerer
repeating the same option MUST replay `answered` with the original actor and
time; every other terminal state MUST return `conflict`. An unshared choice or
option, or a refused check, returns `forbidden`. There is no responder
dismissal. `answered_by` is the bound answering program. Each recorded answer
renews the enrollment through the rights holder's renew-only edit (RIGHTS-R16);
a renewal failure leaves the recorded answer standing. The answer carries no
OA permission or policy effect, and there is no Undo: the application may have
acted on it, and OA holds no effect to remove. Stop ends answering at the next
check. A known limit: when the answerer itself started the work that led
another application to share a choice, the party rule sees two programs and
the answerer may answer; recording that chain needs actor provenance at
admission, which this profile does not carry. First-use answering (ASK-R3,
ASK-R5) has the same limit. The messaging pair relationship closes it only
between the two programs of a pair; a chain through a program outside every
pair stays open.

**[ASK-R10] An answerer asks for its enrollment like any other right.**
`responder@1` `RequestAnswering()` MUST bind the receiving account and
program from the peer. A program that runs foreign code (ASK-R7), an operator program, the
runtime's own program or another account reads `forbidden` and admits
nothing. A current enrollment reads `permitted`. Otherwise the decision of
`abstraction.asks/question.answer` on `apps` is returned, and on
`not_granted` the runtime admits its ordinary first-use question for that
right (ASK-R2), presented in the native permission prompt. A refused
`Answer` or `AnswerChoice` asks the same way after its Book edit releases.
The question's tier is administrative (ASK-R6): the person answers it and no
answerer can. Allow writes the enrollment the operator UI writes, with its
30-day lapse (RIGHTS-R16, RIGHTS-O10); a script host, an operator program or
the runtime named by an older question reads `invalid` there. Refuse writes
nothing and is asked again only after ASK-R2's refuse interval; a lapsed or
stopped enrollment is asked again as any lapsed Allow is. An earlier
`responder@1` answers `unknown_method`. The operator UI keeps the list of
enrolled answerers, the warning and Stop. Its Stop writes, after the
enrollment's removal, a deny on it lasting ASK-R2's refuse interval, so the
next request asks the person no sooner; "Stop and don't ask again" writes
that deny with no expiry, and the person's later removal of it or a new yes
ends it.

**[ASK-R11] Only the person answers whether to keep a pending program.**
The existing catalogue question `application.self_registered` ("<program> was
added to OpenAbstractions as <title>. Keep it?") is admitted by the runtime
only for the current pending registration. This includes an operator-authorized
registry@3 PROGRAM with provider facets and an `applications@2` entry registered
under the account-wide default (rights RIGHTS-R22). Operator authorization
does not assert who added it or replace the person's Keep. The existing operator
confirmation path may ask again. Historical "added itself" questions retain
their original wording and remain readable. Its About slot is the captured
program path: the held Registration image, or the program the platform proved
for the `applications@2` registering connection, whatever spelling the descriptor gives
its program. The runtime records the publisher from that same captured source,
so a prompt names the program by its stem and shows
that path and publisher. It records no asker program. It is closed: its options are exactly
`keep` and `remove`, with no `never`. The application profile refuses it
with `forbidden`, as it refuses `rights.first_use`. No answerer may answer
it, enrolled or delegated, since it vouches for app-written title text; only
the operator profile records its answer. Each registration has its own
question: the runtime acts only on the answer to the question it recorded
for the PROGRAM's current revision, or the `applications@2` entry's current registration.
Keep marks that program confirmed; application facets project the same Keep.
Remove removes the current pending program registration (or its unconfirmed
`applications@2` entries) and writes a Block on
`application.register_self` for that program. Separately authorized operator
management remains available; no `provider.manage` Block is invented.
Closing the prompt or expiry
answers nothing and leaves the entry unconfirmed. No default option exists,
and Remove is never chosen by default. Like every operator prompt (ASK-O14),
the answer is the person's consent as shown, not proof that a person gave
it; keeping or removing an entry grants and revokes no resource permission.

Facade's unpublished applications@3 agent enrollment uses the separate runtime-only
catalogue key `application.agent_enrollment` under P-J15. Only the operator
profile records its person answer; the application and answerer profiles confer
no agent class. Its exact program is an About/detail slot, independent of the
app-written title in the sentence. Facade binds the occurrence to its current
registration and file identity, and applies its answer conditionally. Generic
application Keep retains the behavior above. The new agent-Keep sentence is
proposed source copy reserved for the owner's C-59 acceptance; source receiving
does not establish installed or live person acceptance.

The runtime-only `application.agent_update` occurrence is the single person
Keep for an exact prepared batch of retained connection images. It offers
Update and Not now. Applications captures the installed source, staged and old
image identities, registration revisions and exact membership before admission.
An assistant cannot answer it. Update supplies no resource permission or host
trust and authorizes only that prepared set. Applications conditionally admits
each final identity after publication; changed or uncertain inputs refuse.
Not now preserves the old connections and replays for the same intent.
The sentence lists only captured registered member titles, including its restart
instruction. A member-name discovery read recovers the retained occurrence before
an updated file can regain enrollment; it admits no question or consent.

**[ASK-R12] A kept agent connection is maintained under its original Keep.** A batch that only updates connection programs the person already kept through `application.agent_enrollment` MUST NOT admit an `application.agent_update` occurrence when every member satisfies all of these, as Applications captures them: the same account, binding, enrollment revision and old file identity; staged bytes held from the authenticated installed OA source with the same publisher and release channel; no downgrade; and declared rights, needs and capabilities that are a subset of the kept registration's. That batch commits under the retained Keep provenance. Any other batch — a widening, a new or different program, a foreign profile, a downgrade, or a revoked, disabled or replaced enrollment — MUST use the person's Update under ASK-R11. Maintenance synthesizes no Asks answer and copies no resource or responder grant. The first connection of an agent remains one `SetAgentEnrollment` Keep for one agent. Applications posts one completion notification from the Commit evidence and offers Undo to the prior version while its bytes are retained (CONV-U1). An unsigned helper's staged bytes are held from the installed OA source when they equal the installation's own `tools` copy, hashed and read through one held handle, in a folder that only SYSTEM, Administrators and TrustedInstaller can change. Its declaration is read from those bytes and compared with the kept image's declaration, read from the kept image's own bytes when the Keep recorded none. An image without the release resource declares nothing. No version order applies to unsigned bytes. A Keep recorded before release facts, or a person's answered Update, is the retained provenance (architect ruling 2026-11-11).

**[ASK-R13] One question per app for its kept connection program.** When a
first-use action decides `not_granted` for a kept connection program of a
host program (rights RIGHTS-A6), the runtime MUST pin the host program as the
`PermissionRequest` subject and present the host app's title. ASK-R2's key is
then the host program's (program, action, resource): refusals and `Request`
calls from either program raise or join one question, and Allow or Never
writes the host program's rule. `ObserveQuestion` from the connection program
reads that question as its own. ASK-R3's parties include both programs: an
enrolled connection program never answers a question about its host, and the
host never answers one about its connection program. Without the relation
each program asks for itself, as before.

**[ASK-R14] An enrolled answerer's open chat is woken for a question it may
answer.** Architect ruling 2026-11-12, for the person's assistant to
answer routine asks without the person (VISION 2026-10-02). The agent
connection's Messages watch (`oa_messages_watch`) carries `asks_wake`, an
ordered opaque key of the occurrences that connection's program may answer
now through `responder@1`: ASK-R5's routine first-use questions and ASK-R9's
shared choice options. The key is present only while the program holds a
current enrollment (ASK-R5) and something is answerable; it advances when a
new occurrence appears and is empty when none remains. It carries no question
text, asker, option or ID. While the program is enrolled, one held watch lasts
at most 15 seconds, so a new occurrence reaches the chat within that time. A
host adapter that receives a greater key for a chat with a live inbox
(attached and open) wakes that idle chat once, through its existing
message-wake path, with the fixed pointer `OpenAbstractions: a question is waiting for your answer.` from the
generated wake catalogue. An adapter that holds several open chats of one
program wakes only the one the person used last; the others take the key
without a turn. A busy chat takes it at its next idle; a returned drop
retries at the next idle; an unknown delivery is never resent for the same
key. The model then calls
`oa_asks_list_answerable` or `oa_asks_list_choices` and answers through
`responder@1` as before; every ASK-R5 and ASK-R9 check still applies to each
listed record and answer. A person-only question (an administrative one under
ASK-R6, `command.once`, one about the answerer's own program) is never listed,
so it never wakes a chat. The wake adds no authority beyond the person's
enrollment, and the adapter's background Messages Disable also holds it. A
closed or cold chat is never started for a question; that would change start
consent.

**[ASK-O12] `ReadChoiceDetails`.** `ReadChoiceDetails(id)` MUST return, for an
authorized operator, the same `ChoiceRead` as `ReadChoice` together with the
bound requester program, the shared options (empty for an unshared choice)
and the decision route: `requester` for the application's own `application@2`
decision, `operator` for `operator@1` and `responder` for ASK-R9. The route is
absent while pending, after pending expiry and for a decision recorded before
the service kept it; the requester program is absent for a choice admitted
before. Authorization MUST be rechecked before disclosure. Only `found`
carries details. Acknowledgement removes them with the record.

**[ASK-O13] Delegated answers are retained for audit.** Each answer recorded
under ASK-R9 MUST append one entry to a delegated-answer list: occurrence ID,
requester display and program, recorded publisher, title, chosen option ID
and label, answering program and answer time. Acknowledgement and replay
expiry MUST leave the entry in place. An entry is kept seven days after its
answer. The list holds at most 256 entries and 256 KiB encoded; when either
bound would be exceeded, the oldest entry goes and the Book records the time
at or before which an entry was evicted. `operator@1`
`ListAnswers(cursor, limit)` MUST carry these entries as `kind=choice` and
`answered_by=responder`, within ASK-O2's page, cursor and frame bounds and
authorization rechecks. Title and label are application-authored and decide
nothing. A filtered consumer MUST enumerate every page of that audit before
claiming completeness, retain its `complete_since_ms` watermark, and surface
`gap` instead of treating a partial filtered list as complete. This uses the
same audit bounds, eviction time and cursor revision (ASK-O15).

**[ASK-O15] Every recorded answer is retained for audit.** Each answer
recorded through `operator@1`, `permission-operator@1` or `responder@1` MUST
append one entry to the answer audit inside the same Book edit: a catalogue
question's answer (`AnswerQuestion`, `AnswerQuestionIfPending`, a typed
permission answer, responder `Answer`), recorded as the operator's except a
responder's and an ordinary choice's answer (`AnswerChoice` of either
service). The entry carries the occurrence ID, kind (`question` or `choice`),
the question's catalogue key and About slot, the title (the question's text,
a typed permission's trusted bundle label, or the choice's title), the chosen
option and its label (a question's option name), the asker display, bound
program and recorded publisher, `answered_by` (`operator` or `responder`),
the responder's bound program and the answer time from the ordinary time
floor. A replay, a dismissal, an expiry and an application's own
`application@2` decision append nothing. Release, dismissal, acknowledgement
and replay expiry leave the entry in place; it is kept seven days after its
answer within ASK-O13's bounds of 256 entries and 256 KiB, the oldest going
first and the eviction time recorded. An entry that is not well formed is not
appended, and the answer it describes still records. `operator@1`
`ListAnswers(cursor, limit)` MUST return the audit newest first with that time
(`complete_since_ms`), within ASK-O2's page, cursor and frame bounds and its
authorization rechecks; it is the person's account-wide record of who
answered what, for the operator alone: a remote peer, another account and a
program without operator authorization read nothing (ASK-O1). Titles and
labels may be application-authored and decide nothing. The method joined
`operator@1` as a new method; an earlier service answers `unknown_method`.
Tests: `TestAnswerAuditKeepsEveryRecordedAnswerPastItsRecord`,
`TestVersion8DelegatedAuditMovesIntoTheAnswerAudit` (go),
`TestOperatorListsTheAnswerAuditInBoundedPages` (go/application).

**[ASK-O16] Retained catalogue content is independent of rendered wording.**
New catalogue admissions MUST retain a detached copy of their admitted slots.
A trusted first-use admission without catalogue slots retains its exact immutable
permission request and an empty slot map; its authority remains ASK-O11's structured target,
never a slot. The existing versioned Book and answer audit own this content.
Answering MUST copy it into the same audit entry before the original question
can be dismissed. No replay changes it. Content MUST satisfy the catalogue slot
bounds and About binding; first-use content MUST agree with the retained exact
permission. Invalid current content refuses and MUST NOT fall back to English.

`ReadQuestionContent(id)` MUST return the existing authenticated QuestionAsker
projection plus optional retained content from that same occurrence. Only found
may carry content. `ListAnswerContents(cursor, limit)` MUST return the existing
answer audit entries plus their optional retained content in the page itself,
without per-entry question reads. It reuses ASK-O15's snapshot/revision and
completeness time and ASK-O2's authorization, cursor and encoded page bounds;
its cursor namespace is distinct from ListAnswers. Added content counts toward
the same 4-MiB question partition, 256-entry/256-KiB audit and 64-entry/256-KiB
page bounds. Retiring the question leaves audit content until the same audit
expiry or eviction. Refusals disclose neither entries nor content.

`asks-application@13` retains this content. Versions 1..12 remain readable under
their existing rules and emit the new profile only through a real Book edit;
reads do not rewrite them. Old readers refuse the new profile. Missing content
on an earlier occurrence stays absent, explicitly unavailable, and is never
reconstructed from rendered text. Existing published RecordMetadata,
QuestionAsker and AnswerEntry shapes remain unchanged. Both methods are additive
on operator@1; an earlier service answers unknown_method. A client may then use
its existing metadata reads with absent content; current read failures or
malformed content permit no fallback. Slots are untrusted display data and
confer no answer, permission, runtime-key admission or operator authority.
Tests: `TestCatalogueContentSurvivesTextChangeRetirementAndCallerMutation`,
`TestCatalogueContentLegacyReadsStayAbsentAndMigrateOnEdit`,
`TestCatalogueContentMalformedCurrentAndDowngradedProfilesRefuse` (go),
`TestOperatorContentCurrentRetiredPagesAndAuthorization`,
`TestOperatorContentAuditAndEncodedPageIncludeAddedBytes` (go/application),
`TestReadContentOnlyUnknownMethodUsesExistingMetadataCompatibility` (go/client).

**[ASK-E2] Ordinary outcomes establish no application effect.** Exactly
`pending`, `answered` and `dismissed` observations carry the occurrence ID;
exactly `answered` and `dismissed` carry the recorded decision. `expired`,
`gone`, `unknown`, `conflict`, `invalid`, `exhausted`, `forbidden` and
`unavailable` carry neither. A malformed shape returns `invalid`, capacity
refusal returns `exhausted`, and service/storage uncertainty returns
`unavailable`. Clients MUST NOT infer a person-selected option from any refusal.

## Typed permission requests

**[ASK-A10] Capability-owned bounded consent admission.** `permission@1`
`Request` MUST bind its beneficiary from native receiving account/program
proof. The request carries one keyed action/profile selection or one versioned
finite bundle, a stable request key and optional purpose. A caller MUST NOT
supply a beneficiary, FileKey authority, compiled definition or rule effect.
Trusted compiled capability bindings MUST resolve public target documents and
admit only actually served definitions with the selected first-use selector.
The ordinary `question.ask` gate (ASK-R1) remains on `application@1/@2`;
permission admission uses this capability-owned boundary. Admission itself
grants no permission and conveys no human-presence claim. A selection of one
of the runtime's exact first-use actions is asking before first use, and
ASK-A14 governs it.

**[ASK-R8] A live explicit block refuses consent admission.** The trusted
owner MUST evaluate the complete prepared proposal at one policy revision.
Any explicit block or unavailable decision admits no occurrence. A proposed
All consent may conservatively refuse when it intersects a live One block;
this admission rule MUST NOT change ordinary All collection execution, whose
One blocks filter the concrete resources. The requester MUST receive no
blocked resource names. Immediately before Book admission the owner MUST
quietly recheck the evaluated revision, with no Ask or effect callback.

**[ASK-A11] Immutable typed occurrence and replay.** The Book MUST retain the
exact native beneficiary, canonical owner-encoded target bytes, One/All
selectors, complete finite expansion and definition digest, effect digest,
origin, optional purpose and aligned trusted display. Stable-key replay MUST
return that occurrence without refreshing its wording. Changed authority or
purpose on the same key returns `conflict`. Fresh keys for the same pending
effect share that occurrence; a recorded Refuse stands for ten minutes even
when purpose changes. Existing Book admission, file and call bounds apply;
typed effects have at most 32 selections and each One target at most 4096
bytes. Old exact first-use encodings and receipts MUST retain their meaning.

**[ASK-A13] A pending typed request ends when its decision settles.** The
trusted owner MUST decide every pending typed occurrence's complete effect
again after each committed rights change (RIGHTS-R23), and once when it
starts. An occurrence whose effect a rule or an account-wide default now
permits, or an explicit Block refuses, MUST leave the Book with every
admission naming it and no tombstone, as expiry does (ASK-A7); waiting
observers are released, and the requester's next `Observe` of its key reads
`unknown` while a new `Request` reads the current decision. A decision that
still lacks a grant, or cannot be read, keeps the occurrence pending. An
answered occurrence is never withdrawn.

**[ASK-A14] An application asks before first use and waits for the answer.**
A `permission@1` `Request` whose selection is one of the runtime's exact
first-use actions (ASK-R2) under `abstraction.rights/legacy-exact-string@1`,
selector `one` and target the exact resource, with no purpose, MUST be
prepared by the runtime's first-use path for the proven caller. The served
cases are `abstraction.inference/complete` on `host:<name>` for a server the
router lists, `abstraction.model/lookup` on a registry the runtime lists,
`abstraction.job/acceptance.submit` on `job:<kind>` for a kind the job
owner's grammar names (JOB-A13), and the paired content
profiles; any other target reads `invalid`. The exact enrollment target
`abstraction.asks/question.answer` on `apps` shares `RequestAnswering`'s native
eligibility check and joins the same first-use question (ASK-R10). A current
permit reads `permitted` and a live Block `denied`, asking nothing.
Otherwise the request raises or joins the same question a refused call of
that action raises, under ASK-R2's key for the program, action and
resource, so any number of requests and refusals make one question. It
returns `pending` with the question's id, or `answered` with the person's
standing answer while ASK-R2 does not ask again. No typed occurrence is
admitted for it.
`ObserveQuestion` MUST wait at most `wait_ms` (0..30000) for the answer to
one first-use question whose permission names the proven caller's own
program, whether a refusal named it (inference INF-W11) or `Request`
returned it. Any other id, another program's question included, reads
`unknown`. A pending observation carries `pending_until_ms`, the question's
expiry (ASK-A7). Pending at the end of the wait is not consent. A recorded
Allow supplies no committed-rule witness. A trusted native host may quietly
observe current policy for the retained question and proven caller, returning
`permitted` or `denied` with its current revision. While Allow's effect remains
uncommitted, observation waits on the existing Book/rights change notifications
and reports authorization pending at the wait boundary. The stored answer
remains unchanged. Hosts without this observer retain the answered observation;
callers treat its Allow as waiting. After a current permitted witness, the
caller re-enters its enforcing call once with the same operation identity and
arguments and rechecks authorization.
Neither method answers, grants or widens anything: who may answer follows
ASK-R6 and the party rule of ASK-R3, so an assistant never answers a
question about its own program.

**[ASK-O10] Typed answers and effects remain separate commits.** Only an
explicitly authorized operator may call `AnswerPermissionIfPending`.
Allow/Refuse are the standard choices; a conditional repeat returns `conflict`.
A trusted service may admit an exact native command occurrence with Allow
once, Always allow and Not now. That occurrence uses the person's answer and
the service's exact command receipt. It does not coalesce with other calls.
Allow once writes no Rights rule. Application permission requests and
delegated responders cannot select this occurrence kind.
The operator MUST reread the full frozen effect/display before answering.
Every operator UI that answers typed occurrences, the native permission
dialog and the Panel's list of waiting requests alike, words them from the
same trusted display and answers through the same conditional call, refusing
to answer when the effect digest it showed no longer matches.
Rights effect completion and Undo MUST accept only the retained occurrence ID
and expected policy revision, resolve the pinned actual operator Allow, and
commit the entire effect atomically with its durable occurrence receipt.
Refuse writes no target rule. Lost or failed completion retains explicit Retry.
Undo removes only still-current occurrence-owned rules. Generic typed
occurrences remain operator-only. The FAC-M19 routine application companion
MAY let an enrolled nonparty responder answer the exact retained read,
reveal and propose selections on one `app:<name>`, together. It MUST use the
shared answerer and party rules, live enrollment and atomic effect
completion. Exact delegation supplies no fallback. Those selections cover
commands on a running instance only. An agent's start of a stopped
application is person-only (coordinator, 2026-11-09): FAC-M32 asks it as the
runtime-only once occurrence "Open <app>?", which no responder lists or
answers, which writes no policy, and whose fingerprint differs from the
routine permission question, so the two never coalesce. FAC-M20 standing
edit, commit and spend remains person-only, with all three operations visible.
The public single/storage-pair request limits remain unchanged.
Deprecated public application/operator ask, read, list and answer methods MUST
fail closed for typed entries.
Legacy typed-detail readers refuse unfamiliar responder attribution and
runtimes predating version 11 refuse its state. Compatible generic responder
list/answer clients may consume the complete truthful text projection.

For this finite responder projection, `RecordMetadata.text` MUST show the
bound beneficiary account/program, exact application and every retained
operation/disclosure on both list and answer. Stored archival Text remains
unchanged. Display words supply no policy targets. The native finite-service
occurrence admission companion MUST preserve same-key replay, pending
coalescing, quotas and in-lock admission checks. A fresh call after Refuse,
close or expiry may ask again; generic typed Refuse pacing remains ten minutes.

**[ASK-E3] Typed observations convey bounded state.** `pending` and `answered`
alone carry the retained answer shape; an unanswered record contains no
selected option. `permitted` and `denied` carry only the evaluated policy
revision. `invalid`, `conflict`, `forbidden`, `unavailable`, `unknown` and
`gone` carry neither. An observed Allow establishes the Book answer; each
resource service MUST independently enforce its current permission before
performing the requested operation. `Observe` reads only the proven caller's
request namespace and supports the existing bounded wait.

## Notifications

**[ASK-N1] Notification admission.** `notification@1` `Post` MUST bind the
poster through the same receiving-account Program proof as ASK-A1 and derive
its scope, bound program, display name and publisher evidence (ASK-A6); no
request field names a program, publisher or icon. A host serves notifications
only after `EnableNotifications`; without it every `Post` is `forbidden`.
The configured policy decides `abstraction.asks/notification.post` on
`account` before admission and again inside the admitting edit. A live
explicit block MUST return `suppressed`; an evaluated refusal `forbidden`; an
unobtained decision `unavailable`. None of them stores or shows anything.
The request bounds are: a nonblank single-line title of at most 80 Unicode
characters; an optional body of 1..240 Unicode characters whose only control
character is line feed; 0..2 distinct actions, each a stable ID of 1..64 UTF-8
bytes and a single-line title of at most 40 Unicode characters; an optional
tag of 1..64 UTF-8 bytes. An `operation_id` names one job operation in the
JOB-A13 grammar, and the trusted host MUST confirm that the same caller scope
accepted it before admission; another scope's or an unknown operation is
`invalid`. An `open_activity` action and `operation_end` delivery require
`operation_id`. Identical full reference and content MUST replay the
occurrence; changed content is `conflict`. A tag replaces, by withdrawal, the
caller's live notification with the same tag. `now` delivery posts at
admission; `operation_end` holds the notification `scheduled` until the
trusted runtime observes the operation end and posts it.
For a retained scheduled occurrence without native provenance, the live limit
also counts native records whose captured provenance matches its exact executable
scope. This count grants no access to those records. Posting remains deferred
when the limit requires withdrawing a record outside the retained scope, or a
matching tag would require changing such a record.

**[ASK-N2] One-way content.** A notification MUST grant no authority and
collect no text. Its title, body and action titles are application-authored.
A presenter MUST show them with the service-recorded sender in a place the
application cannot write, and MUST NOT let them select OA permission or
elevation presentation. A recorded action is application intent only; the
poster performs any effect itself and resource services enforce their own
authority.

**[ASK-N3] States and deadlines.** An occurrence is `scheduled` or `posted`,
then exactly one of `acted`, `dismissed`, `withdrawn` or `expired`. Terminal
states MUST NOT change. `expires_ms` MUST be positive, after effective service
time at admission and no later than `replay_until_ms`; the reference follows
ASK-W5, including the seven-day admission horizon and the ordinary time floor.
A backward clock MUST NOT reopen an expired occurrence. Reaching
`expires_ms` makes a live occurrence `expired`, which carries no result.
Reaching `replay_until_ms` collects the occurrence; later `Post` and
`Observe` of that reference return `expired`. `Observe(request_ref,wait_ms)`
follows ASK-W1/W3 and wakes on any outcome. `Withdraw(request_ref)` withdraws
the caller's own scheduled or posted occurrence; the same withdrawal replays
and any other terminal outcome is returned unchanged.

**[ASK-N4] Action invocation.** Only an explicitly authorized operator
(ASK-O1) records an action, through `Act(id,action,presentation)`. Every
operator read carries a presentation token bound to the record and to the
current host epoch. `Act` MUST refuse a token from another epoch as `stale`
and record nothing. Inside the atomic edit it MUST recheck operator
authorization, state `posted`, effective time before `expires_ms`, the offered
action ID and the poster's live block; a blocked poster's notification takes
no action (`forbidden`). The first terminal outcome wins; the same action
replays its original actor and time, and another outcome is `conflict`.
`decided_by` names the proven recording program and asserts no human
presence. `Dismiss(id)` records a person's dismissal without an action.
`WithdrawProgram(program)` withdraws every live notification of one bound
program and reports the count.

**[ASK-N5] Acknowledgement and retention.** `Acknowledge(request_ref,result)`
MUST compare the exact recorded result in the caller's scope and follows
ASK-P4's acknowledgement rules: it replaces the record with a fingerprint
tombstone until `replay_until_ms`, the same acknowledgement replays, and
later `Post` or `Observe` of that reference return `gone`. An expired
occurrence carries no result to acknowledge.

**[ASK-N6] Suppression, rate and audit.** The person turns one program's
notifications off with a Rights block on `abstraction.asks/notification.post`
for that program; a block wins over any default permit. Turning off SHOULD be
followed by `WithdrawProgram`. The partition MUST hold at most 1024
admissions including tombstones and 1 MiB of encoded state, separate from the
legacy and ordinary partitions. One program MUST be admitted at most 20 times
in any rolling hour, and keeps at most four `posted` notifications; a further
`now` post withdraws its oldest. A bound reached returns `exhausted` and
stores nothing. The service MUST count admissions per program per service UTC
day for seven days, for at most 256 programs, and `ListNotifiers` serves them.

**[ASK-N7] Presentation.** A presenter MUST read through
`notification-operator@1`, show a notification only in the person's unlocked
interactive session, remove its native item once the occurrence is terminal,
and send every click with the token it was shown with. Native expiry and a
native close are presentation facts: a presenter MAY report an observed close
through `Dismiss`, and MUST NOT infer one. The service record decides every
outcome.

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
| `Ask`, `Observe` | `unknown` | No admission for this key in the caller's subject, including after its question expired (ASK-A3, ASK-A7). |
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
| `AnswerQuestionIfPending` | `answered` | The choice was recorded by this call; never a replay (ASK-O8). |
| `AnswerQuestionIfPending` | `conflict` | The question was already answered, with any option (ASK-O8). |
| `AnswerQuestionIfPending` | `unknown`, `invalid`, `forbidden`, `unavailable` | As `AnswerQuestion` (ASK-O3, ASK-O8). |
| `ReadAttributedQuestion` | `found` | The retained question and its publisher recorded at admission (ASK-A6, ASK-O7). |
| `ReadAttributedQuestion` | `unknown`, `invalid`, `forbidden`, `unavailable` | No retained question, a malformed id, no operator authorization, or storage failure (ASK-O7). |
| `ListAnswerable` | `page` | Bounded pending routine runtime first-use questions and only the options this caller may currently choose (ASK-R3, ASK-R5, ASK-R6). |
| `ListAnswerable` | `gap` | Caller, host epoch or Book revision changed; restart from an empty cursor (ASK-R3). |
| `ListAnswerable` | `invalid`, `forbidden`, `unavailable` | Range invalid, no bound responder, or grant/Book cannot be checked (ASK-R3). |
| `Answer` | `answered` | The Book choice is recorded; inspect `effect` separately before claiming a target permit (ASK-R4). |
| `Answer` | `conflict`, `unknown`, `invalid`, `forbidden`, `unavailable` | A different choice, absent ID, invalid input, absent grant, a question that is not routine, a party or a program that runs foreign code, or an unavailable check refused (ASK-R3 to ASK-R7). |
| `RequestAnswering` | `permitted`, `not_granted`, `forbidden`, `unavailable` | Enrolled; the person is asked in the native permission prompt; a program that runs foreign code, an operator program, the runtime or another account; an unavailable decision (ASK-R10). |
| `ShareChoice` | as `Ask` of `application@2` | Shared options outside the offered IDs, repeated or more than three read `invalid` (ASK-A12). |
| `ListChoices` (responder) | `page`, `gap`, `invalid`, `forbidden`, `unavailable` | Pending shared choices this caller may answer, with only their shared options; refusals as `ListAnswerable` (ASK-R9). |
| `AnswerChoice` (responder) | `answered` | The shared option is recorded, or the same answerer's same option replays (ASK-R9). |
| `AnswerChoice` (responder) | `conflict`, `expired`, `gone`, `unknown`, `invalid`, `forbidden`, `unavailable` | Another terminal outcome; an elapsed deadline; an acknowledged choice; no occurrence; malformed input; an unshared choice or option or a refused answerer; an unavailable check (ASK-R9). |
| `ReadChoiceDetails` | `found` and the read refusals | As `ReadChoice`, with requester program, shared options and route (ASK-O12). |
| `ListAnswers` | `page`, `gap`, `invalid`, `forbidden`, `unavailable` | The answer audit newest first and the completeness time (ASK-O15). |

## Bounds

| what | bound | rule |
| --- | --- | --- |
| slot value | at most 200 Unicode characters, no control character | ASK-A2 |
| idempotency key (`request_key`) | 1..128 UTF-8 bytes, no control character | ASK-A2 |
| `Observe` wait | 0..30000 ms | ASK-W1 |
| Go client default call budget | 5 seconds | ASK-W3 |
| Go client `WithTimeout` budget | up to 35 seconds | ASK-W3 |
| simultaneous host calls | at most 32, each bounded to 35 seconds | ASK-W3 |
| application-profile file | at most 4 MiB + 1 MiB + 256 KiB + 1 MiB + 328 KiB + 4096 bytes; independent encoded partition bounds | ASK-P1, ASK-P4, ASK-O15, ASK-P5 |
| legacy admissions and retained records | at most 4096 each and 4 MiB | ASK-P2 |
| ordinary admissions, including ack tombstones | at most 1024 and 1 MiB including time floor | ASK-P4 |
| one program's share | an eighth of each partition: 512 `application@1` and `permission@1` admissions and 512 retained records; 128 ordinary admissions and 128 KiB | ASK-P5 |
| ordinary title / each label | at most 80 Unicode characters, nonblank, single line, no controls | ASK-A9 |
| ordinary options / each ID | 1..3 distinct IDs; each 1..64 UTF-8 bytes, nonblank, no controls | ASK-A9 |
| ordinary replay admission horizon | at most seven days; caller retains the immutable full reference | ASK-W5 |
| shared options | 1..3 distinct offered IDs | ASK-A12 |
| answer audit, delegated answers included | at most 256 entries and 256 KiB encoded; each kept seven days after its answer; a question's title at most 8192 UTF-8 bytes | ASK-O13, ASK-O15 |
| `ListAnswers` page | 1..64 entries, at most 256 KiB encoded; cursor at most 256 UTF-8 bytes | ASK-O2, ASK-O15 |
| notification title / body / action title | 80 / 1..240 (line feed allowed) / 40 Unicode characters | ASK-N1 |
| notification actions / tag | 0..2 distinct IDs of 1..64 UTF-8 bytes; tag 1..64 UTF-8 bytes | ASK-N1 |
| notification admissions, including tombstones | at most 1024 and 1 MiB with the per-program audit | ASK-N6 |
| notification rate / live per program | 20 admissions per rolling hour / 4 posted | ASK-N6 |
| notification audit | seven service UTC days, at most 256 programs | ASK-N6 |
| `ListQuestions` page | 1..64 records, at most 256 KiB encoded, within the shared 1 MiB frame | ASK-O2 |
| `ListQuestions` cursor | at most 256 UTF-8 bytes | ASK-O2 |
| `AnswerQuestion`, `DismissQuestion` id and option | 1..128 UTF-8 bytes, no control character | ASK-O3, ASK-O4 |
| `ReadAttributedQuestion` and `ReadQuestionAsker` id; `AnswerQuestionIfPending` id and option | 1..128 UTF-8 bytes, no control character | ASK-O7, ASK-O8 |
| pending question lifetime | 15 minutes by default; a host may configure another positive value | ASK-A7 |
| enrollment | 30 days after the last answer; Stop ends it | ASK-R5, RIGHTS-R16 |
| system-access `once` | claimed by the helper within 120 seconds of the answer | ASK-S1 |
| recorded publisher subject | at most 256 UTF-8 bytes, no control or format character | ASK-A6 |
| recorded program description and original file name | each at most 256 UTF-8 bytes, no control or format character | ASK-A6 |

## Divergences

None declared for this release.

## Not built

- `operator@2` with the wire method `DismissQuestion` (D50;
  `research/vocabulary/RENAME-PLAN.md` §4, third release). Until then
  `DismissQuestion` is the prose name, and `operator@1` keeps
  `RetireQuestion` and the outcome `retired`.
- A structured reason on `unavailable`, an `INF-A2`-style family
  (`lease:<outcome>`, `engine:<name>`). A caller tells a storage failure from
  a capacity refusal by diagnostic text alone.
- Ordinary-choice evidence beyond Go's native client and Python's generated
  codec: a native Python caller, an application adopter, ordinary agent
  delegation and a native presenter.
- A Linux fixture receipt for the unix-socket listener shared with `rights`.
- A Linux presenter for questions and notifications (asks audit P2-1).
  Linux questions are answered on the Panel's Questions page.
- The person-present check of the macOS question panel (ASK-O14) over a
  full-screen app.
- An application adopter of notifications (ASK-N1..N7).
- A conformance scenario corpus that cites this layer. The `go/` tests cover
  the peer-boundary case alone: every operation with a valid credential and
  no identity, five refusals.
