# abstraction-asks

*For applications that need a person's answer. Operators answer through the
[asks operator client](#operator-answering-and-history).*

Asks keeps application requests and answers in a shared service. Applications
own ordinary choice UI; OA owns permission presentation. The service holds
pending records, verified requester attribution and bounded answer replay.

The [permission journey](https://github.com/openabstractions/abstractions/blob/main/examples/rights-playground/README.md)
explains how Identity, Rights and Asks connect native consent to an applied
rule and an actual credential-bearing operation. It preserves the accepted
broader-access design and the current limits of the Windows/macOS demonstration.

**Development source:** the 0.4.0 work below has separate source, native and
installed evidence. [Released 0.3.0](https://github.com/openabstractions/redist/releases/tag/v0.3.0)
has its own package contents and qualification.

| Job | What asks does | Today |
| --- | --- | --- |
| **App-owned choices** | `application@2` admits an app's title and stable offered option IDs, with pending/replay deadlines, own answering, observation and acknowledgement. The application maps the answer to its own behavior; the record grants no OA permission (ASK-A9). | Caller-scoped Answer/Dismiss integrated at `ed97c10f8`; Go terminal example integrated at `f27944846`; ordinary agent answering planned |
| **OA permission requests** | `rights.first_use` carries the exact program, action and structured target. Rights applies the answer's permission effect. Authorized operators answer; enrolled responders answer eligible routine first-use requests under the existing scope and tier rules. Legacy `application@1` keeps its fixed catalogue. | Windows 11 renderer integrated through `957df22bd`, with bounded native guards passed; final visual acceptance and system/privacy forms remain open |
| **Notifications with actions** | `notification@1` admits an app's one-way notification with up to two actions and an absolute expiry; the service holds the person's chosen action after the app exits, and the app observes and acknowledges it on its next run. A Rights block on `abstraction.asks/notification.post` turns one app's notifications off (ASK-N1..N7). | Service, Go client and generated bindings in source; native presenters and an application journey in progress |

| Before | With asks |
| --- | --- |
| A request disappears with the application that asked it. | The service retains pending and answer state; the app owns an ordinary choice's UI and local effect. |
| An application that keeps asking keeps interrupting. | The person's rule decides whether that application may ask at all. |
| Reading the system log or managing one service means running the whole tool as administrator. | Designed: one identified program gets that one capability through one question. A refused first-use capability (a spending action or a lend into another app's folder) already works this way. |
| An approval waits until the person comes back to the machine. | An answerer the person enrolled in the Panel answers routine first-use questions for other apps; sensitive and administrative questions stay with the person. |

The Panel reads metadata and answer attribution through `ReadAttributedQuestion`
and trusted permission details through `ReadQuestionAsker`. `ListAnswers` carries
the complete audit; assistant views filter its fully enumerated pages by kind
and answerer route. Revoke answering access ends future answers, with optional cleanup
of current permissions granted through the app. Undo removes only the
still-current permission effect of one answer. Paired write/read effects
commit together or not at all.

The legacy `application@1` catalogue remains closed. Current `application@2`
admits ordinary app-authored choices. Both require explicit asking authority;
ordinary requests have separate bounded capacity and replay retention.

The additive `permission@1` API lets an identified application request one
typed permission, an explicit All collection, or a versioned finite bundle.
It accepts a stable request key and owner-defined selection, with optional
purpose. The service binds the native caller and resolves targets through
trusted compiled capability definitions. This bounded consent admission needs
no prior ordinary `question.ask` grant. A current explicit block refuses it.
Purpose text supplies no authority. `Observe` reports the retained decision;
Allow and the Rights effect are separate commits, and the application retries
the resource service after permission is actually applied. Typed answering is
operator-only; existing routine responder grants retain their legacy scope.
This consent profile retains at most 32 selections with at most 4,096 bytes per
One target; a larger target supported by its resource owner can be refused here.
See [typed permission rules](CONTRACT.md#typed-permission-requests).

Keep a request and its answer across an application's observation/recovery.
For legacy catalogue requests the service supplies wording/options; ordinary
choices carry app-authored titles and stable IDs.

## Presentation ownership

An application presents ordinary choices in its own window, CLI or existing
controls. Automatic OA ordinary popup calls are removed at `0b6c877ae`; the
Windows mixed-pending production-polling regression passed. Fresh native Mac
verification remains open. Earlier `e27199249` native fixtures retain their
source-bound functional verdict.

`application@2` exposes `Ask`, `Observe`, `Answer`, `Dismiss` and `Acknowledge`.
Its `Answer`/`Dismiss` methods use the proven application's own immutable
`RequestRef`; account-wide `AnswerChoice`/`DismissChoice` remain with an
explicitly authorized `operator@1`. Program attribution establishes no human
presence or permission effect.
Acknowledgement confirms an already-recorded answer; it supplies no new answer.
The caller-scoped write path is integrated at `ed97c10f8`. The Go terminal
example at `f27944846` prints the full destination, submits its own selection
and observes the retained winner before its guarded output effect. Its suite
and Windows redirected-input runtime checks passed; actual console typing
remains manual acceptance, and Linux/macOS checks are compile-only. ComfyUI
uses its existing model controls at `48ca691a2`.
Ordinary agent delegation remains planned. Routine first-use
responder enrollment gives no ordinary answering
authority (ASK-O9).

OA permission requests use the hidden session presenter without a persistent
Windows tray or Mac menu-bar icon. Native Windows permission interaction and
Mac public-XPC fixtures have run. Actual owner Mac answering, lock/unlock,
installed login/upgrade and final visual acceptance retain separate limits.
The blue-header reference slice at `06d3a328d` passed native Refuse and Cancel
checks. The owner rejected its appearance as the modern Windows default; it
remains a Windows 10 reference. A dedicated Windows 11 native renderer is
integrated through `957df22bd`, including bounded expanded details and compact
chrome. Successful answers finish without a second confirmation dialog at
`67922c707`; failed or uncertain completion retains its report. Combined native
lifetime/answer guards and result-report checks passed on reviewed worker
`d26c823b9`, whose Monitor source matches `957df22bd`, using synthetic requests
and a fake Rights owner. The owner-supplied screenshot shows Windows 11
renderer baseline `3c3b31abf`; final visual acceptance of the compact revision,
keyboard/UIA inspection, cross-monitor DPI/high contrast and Windows 10
appearance remain open.
System-access and privacy-access forms remain planned; actual administrator
and device/data consent use their platform-owned authorization paths.
Action notifications remain a separate planned compact surface.

Permission presentation explains the program, recorded publisher, exact action,
resource, effect and duration. Readable display grouping and stable action code
are distinct from the routine/sensitive/administrative tiers that decide who may
answer. An administrative OA tier alone supplies no OS elevation requirement.
The common operation/typed-target permission schema was accepted as
implementation direction at `3ffb5db05`; the reviewed source inventory at
`35315c601` maps existing definitions and enforcement gaps. Source `e5c1eebb1`
integrates 34 compiled built-in action definitions, the Credentials-owned typed
named Use resource and shared native/web first-use explanations. Credential
first-use reuses existing service methods. Named Use follows the current
account-owned name slot through
rotation or recreated registration, under its current host/consumer restrictions.
An isolated native IPC/loopback test passed missing permission → exact
caller-bound ask → recorded Allow/applied permit → retry using the named secret
on reviewed worker `ab468aa7d`. Native visual evidence retains its separate
limits above. Typed one/all selectors, versioned full control, breadth-warning
presentation, raw secret Read and the protected OS helper remain unimplemented.
Breadth warnings and actual OS elevation remain separate.

## The problem

Applications need shared pending and answer state across process exit and
reconnection. Ordinary choices keep their app-owned presentation and effect;
the service retains their immutable request and answer within declared bounds.
The legacy walkthrough below names a question the service already knows.

The example is in [Ask from an application](#ask-from-an-application).

Legacy `application@1` questions come from the fixed catalogue below. It includes application prompts,
the runtime's first-use rights prompt, legacy registration, and the native
OpenCode session's folder-access prompt. An application
names one of these and fills the slots it declares. An ordinary app-defined
choice uses the separate `application@2` profile. A person
answers later, through the Panel or the command line, and the answer is a
durable record that outlives the application that asked. A new question is
added to the legacy catalogue by appending one entry to `Questions` in
[`go/questions.go`](go/questions.go).

Applications resolve `abstraction.asks/application@1` through the facade, then
call `AskContext` and `ObserveContext` with a stable request key. The application
chooses a question from the service's fixed catalogue and supplies its values.
Operators (people at the command line) use the separate asks operator client
to list and answer pending questions. The
receiving account and native program identity define the subject of every
application request.

[CONTRACT.md](CONTRACT.md) specifies replay, waiting, authorization and operator
integration. The native CLI/provider workflow below remains an explicit adopter
path.

A legacy catalogue question is a record with fixed options, service-owned wording and one
retained answer. A kept answer prevents the same application question from
being asked again. The `never` option is a retained refusal.

## Words

| word | meaning |
|---|---|
| **question** | one name from the closed list in [`go/questions.go`](go/questions.go), with the slots it declares |
| **ask** | an application filling a question's slots; pending until a person answers, and one record however many times it is asked |
| **answer** | a record the person keeps: the text as the person saw it, who asked, what the platform proved about who connected, and when |
| **kept** | an answer that stops the question being asked again; `once` is not kept, `never` always is |
| **via** / **for** | the program that connected, bound by this service; and the program it asks on behalf of, on its word (deprecated, kept for explicitly selected native integration; CONTRACT.md ASK-A5) |

CONTRACT.md's rule ids, `ASK-A1` and the rest, are what tests and refusals
cite.

## Catalogue

The closed list a legacy `application@1` caller chooses from, generated from
[`go/questions.go`](go/questions.go). `go -C go test ./... -count=1` fails
when this table and that file disagree; run it with `-update` to regenerate
the table in place.

<!-- questions-table:start -->
| id | wording shown to the person | what answering allows |
|---|---|---|
| `content.replace` | `{asker} requests a one-time replacement of {target} with {replacement}. Request {code} expires {until}.` | `once`: permitted now, asked again next time · `never`: refused permanently, remembered |
| `rights.register` | `{asker} wants to: {rights}` | `allow`: permitted now, asked again next time · `refuse`: refused now, asked again next time · `never`: refused permanently, remembered |
| `opencode.project.edit` | `OpenCode requests permission to read, run, create, edit and delete files throughout {folder}, including its subfolders, until this sandbox session exits. Request {code} expires {until}.` | `once`: permitted now, asked again next time · `refuse`: refused now, asked again next time · `never`: refused permanently, remembered |
| `download.reach` | `{asker} wants to fetch from {host}` | `allow`: permitted permanently, remembered · `once`: permitted now, asked again next time · `refuse`: refused now, asked again next time · `never`: refused permanently, remembered |
| `rights.first_use` | `{program} wants to {action} on {resource}` | `allow`: permitted now, asked again next time · `refuse`: refused now, asked again next time · `never`: refused permanently, remembered |
| `application.self_registered` | `{program} added itself to OpenAbstractions as {title}. Keep it?` | `keep`: the app stays registered, confirmed · `remove`: the app's entry is removed and it may not register itself again |
| `system.securitylog.read` | `{asker} wants to read one page of the Windows Security log. Windows asks for administrator approval next. Request {code} expires {until}.` | `once`: permitted now, asked again next time · `refuse`: refused now, asked again next time · `never`: refused permanently, remembered |
<!-- questions-table:end -->

These entries are templates; the three first-use permission risk tiers are a
separate policy. `rights.first_use` is active in the installed runtime, and
`rights.register` remains in the legacy registration flow. `download.reach`
is a historical `dl` question used in examples. `content.replace` has no
current product consumer; its ComfyUI canvas-replacement caller was removed
after the owner's decision that the person's click authorizes that in-app
action. The catalogue still supports decoding and replay of retained records.

A new legacy catalogue question is added by appending one `Question` to `Questions` in
`go/questions.go`; the table above is regenerated from that change.

When the asks service cannot be resolved, `ResolveAsks` fails the way any
capability resolution fails. The calling application then decides what
happens next. The removed standalone `dl` caller logged that no one answered
and fetched unasked, as described under [Today](#today).

## Ask from an application

`Ask` and `Observe` decide `abstraction.asks/question.ask` on `account`
before admitting a question. A person grants that rule explicitly, through
`openabstractions rights grant --program /absolute/path/to/built/binary
--action abstraction.asks/question.ask --resource account`; the runtime's own
first-use question covers job submit, model lookup and inference complete.

```go
package main

import (
	"context"
	"log"

	asks "github.com/openabstractions/abstraction-asks/go/client"
	facade "github.com/openabstractions/abstraction-facade/go"
)

func main() {
	ctx := context.Background()
	q, err := facade.Discover().ResolveAsks(ctx, facade.Requirements{})
	if err != nil {
		log.Fatal(err)
	}
	obs, err := q.AskContext(ctx, asks.ApplicationQuestion{
		RequestKey: "dl-fetch-example.com",
		Key:        "download.reach",
		Slots:      map[string]string{"host": "example.com"},
	})
	if err != nil {
		log.Fatal(err)
	}
	if obs.Outcome.String() == "pending" {
		log.Println("waiting for a person to answer; dl will retry the same key")
	}
}
```

`AskContext` admits the question and returns whatever is already known about
it; a first ask reads `pending`, and the application shows that a person has
not answered yet rather than inventing a wait of its own. `ObserveContext(ctx,
key, waitMS)` long-polls the same key up to 30 seconds for a later answer.
Retrying `AskContext` with the same `RequestKey` and the same slots after an
uncertain reply reaches the same record rather than opening a second question.

## Obtain

- **Go.** `go get github.com/openabstractions/abstraction-asks/go`. Current
  release [`go/v0.1.1`](https://github.com/openabstractions/abstraction-asks/releases/tag/go%2Fv0.1.1).
  Standard library plus
  [abstraction-identity](https://github.com/openabstractions/abstraction-identity)
  (which brings `golang.org/x/sys`).
- **Other languages.** See generated protocol and shared transport/client packages
  in this repository and the facade. Native provider support is separate.

## Run

Install the runtime first: https://openabstractions.org/adopt.html. It owns the
question book and exposes the authorized operator over local IPC. Use the
runtime command to inspect and answer questions:

    openabstractions questions pending
    openabstractions questions answer 0da747 once
    openabstractions questions permissions

`pending` and `answer` resolve `abstraction.asks/operator@1`, whose list
holds ordinary questions. `permissions` resolves
`abstraction.asks/permission-operator@1` and lists pending typed permission
requests; the person answers those in the OA permission dialog or the Panel.
The runtime checks the calling program's operator authority.
`openabstractions questions --help` shows the supported options and answer
outcomes.

### Legacy standalone mode

The source still includes `asksd` and the `asks` command for standalone
compatibility. They use the legacy standalone question book and `admin.secret`
protocol; they are separate from the installed runtime's application book and
operator IPC. Do not start them beside the runtime on its default endpoint.
The old command supports `pending`, `answer`, `answered`, `forget`, and
`questions`.

Applications use the fixed question catalogue and the service-owned book in
either mode. A standalone `asksd` must not be pointed at the runtime's
`application.json` state.

## Contract

Your application can distinguish an unknown question from an invalid answer
without parsing an error message. The message is for the person; the code is for the program.

Service refusals carry an optional stable `code` alongside the existing diagnostic
`error` text. Either nonempty field means refusal. Old text-only replies remain
valid; unknown codes remain refusals and must not be treated as success. Servers
continue sending diagnostic text for older clients. Go clients return
`*RemoteError`, retaining the code and message; `Response.Err()` applies the same
rule to a decoded reply. Known native sentinels remain accessible through
`errors.Is`. The [code constants](go/errors.go) define the vocabulary; an
unclassified service failure uses `internal`. Diagnostic wording is not an API.

- **An application phrases nothing.** It names a question from `Questions` in
  [`go/questions.go`](go/questions.go) and fills the slots that question
  declares — one line each, at most 200 characters. An unknown question, a
  missing slot or a slot with a newline is refused. Adding a question is a
  change to this layer, which is the friction that keeps forty dialogs from
  coming back through one door.
- **An answer is a record the person keeps.** It carries the text as the person
  saw it, who asked, what the platform proved about who connected, and when.
  It survives the service; `asks.json` is the whole state.
- **A kept answer is not asked again.** Each option says whether it is kept;
  `never` is offered on every question and is always kept. An answer that is
  not kept — `once` — is history, and the next ask is a new question.
- **The service never says yes.** Until a person answers, the ask is pending.
  A pending question is one record however many times it is asked, and it
  outlives the asker: an application that gives up waiting finds the answer
  next time.
- **A question is one asker's.** Two applications asking the same thing are two
  questions; a question may name one slot as what it is *about* (`host` for
  `download.reach`), and answers are kept per asker and subject.
- **Answering is the person's act.** In the runtime path, the operator profile
  is reached over local IPC and the runtime checks the caller's identity. The
  legacy standalone operator proves standing by possessing `admin.secret`.
  Applications cannot read the queue.
- **Every operation binds its caller before it reads the request.** All five
  go through `listen.Receive`; a caller the kernel cannot identify to
  `listen.Program` is refused with the reason (`asks: refused, identity: ...`)
  and its request is never parsed. The service refuses to start on a machine
  that cannot bind (`identity.CanEver`). A test sends every operation with a
  valid credential and no identity and expects five refusals.
- **`via` is the program that connected, bound by this service. `for` is the
  program it asks on behalf of, on its word.** `rights` asking for `keepawake`
  is recorded `via openabstractions, for keepawake`; `for` is what the rights service bound at its
  own end and forwarded, and the person is told it is the asker's word. This is
  HTTP's `Via` beside `X-Forwarded-For`: each hop binds its own peer and copies
  nothing from the message into `via`.
- **This holds among programs sharing one kernel and one user account.**

## Today

**Current native source evidence.** Windows named-pipe and TaskDialog fixtures
and native Mac public-XPC permission/ordinary fixtures passed at `e27199249`.
The app-owned ordinary presentation correction has source and Windows
production-polling evidence; its fresh native Mac verification remains open.
The modern Windows default appearance, Linux native presentation and installed
0.4.0 acceptance remain open. Mac XPC evidence is separate from a standalone
Unix socket provider's qualification.

In the installed runtime path, applications ask through the application profile
and operators answer through `openabstractions questions`. The separate native
Go standalone implementation retains
[`rights`](https://github.com/openabstractions/abstraction-rights) as its legacy
registration caller. A registration is `rights.register`, and its Allow answer
is not kept, because a
kept *allow* would let any later process with the same name in without the
person seeing its path. The removed standalone `dl` caller asked
`download.reach` before the first fetch from a host; without a running `asksd`,
it said so and fetched unasked. That template remains in the catalogue and
examples.

What may break: the pipe name belongs to the first listener
(`FILE_FLAG_FIRST_PIPE_INSTANCE`): a second `asksd` on the same name refuses to
start, and so does one started while a client of the previous one has not hung
up. There is no notification: in standalone mode, a person finds out there is a
question by running `asks pending`, or because the application said so. Pending questions are
unbounded, and an application that asks in a loop fills the file with one
record — the dedupe holds — but a hostile one can vary the subject.

## Conformance

No scenario corpus; there is one implementation. The tests in `go/` send every
operation with a valid credential and no identity and expect five refusals.

## Where it sits

Below: [abstraction-identity](https://github.com/openabstractions/abstraction-identity)
(who connected) and [abstraction-cas](https://github.com/openabstractions/abstraction-cas)
(`asks.json`). Above:
[abstraction-rights](https://github.com/openabstractions/abstraction-rights)
retains its legacy registration flow through it.
[abstraction-download](https://github.com/openabstractions/abstraction-download)'s
removed `dl` caller asked before reaching a host.

One layer of [openabstractions](https://github.com/openabstractions/abstractions).
Every layer names one thing local tools rebuild on their own; the name means the
same in each language that implements it, and the conformance scenarios are what
hold an implementation to it.

## Requirements

Go 1.26 or newer. The [presentation section](#presentation-ownership) states
current source/native limits; package installation evidence follows the exact
release. Linux native prompt qualification remains open.

## Status

**In development.** Tagged `go/v0.1.1`, but no conformance scenario cites this
layer, and the API carries no stability promise.

## Licence

Apache-2.0. See [LICENSE](https://github.com/openabstractions/abstraction-asks/blob/main/LICENSE).

### Operator answering and history

The generated `QuestionOperator` service provides bounded latest-book history
and explicit catalog-option answering. A host enables it with a typed peer
authorization callback before serving. The callback can use the rights service;
ordinary same-account applications remain unauthorized by default.

Go applications use `client.NewOperator(selectedEndpoint)` and
`ListQuestionsContext` / `AnswerQuestionContext`. Resolve the
`abstraction.asks/operator@1` contract before selecting its endpoint. A history
gap requires restarting enumeration from an empty cursor. After an uncertain
answer reply, explicitly retry the same ID and option to recover the original
decision. Recorded answers do not grant resource authority.

`RetireQuestionContext(ctx, id)` dismisses a retained pending or answered
question (CONTRACT.md's `DismissQuestion`, ASK-O4). It returns `retired`
with the question's last metadata, `unknown`, `invalid`, `forbidden` or
`unavailable`. Repeat the same ID to reconcile an uncertain
reply. An application's later Ask or Observe for that key reports `gone`, and
the key never admits a new question. Python operators use
`Machine.resolve_asks_operator()`. See [CONTRACT.md](CONTRACT.md) for limits and
restart behavior.

<!-- BEGIN GENERATED PERMISSION DEFINITIONS -->

## Permission definitions

Generated from the existing compiled catalogue. [RIGHTS.json](RIGHTS.json) contains the complete action/profile records, disclosure, lifecycle, first-use behavior, selector support and finite bundles.

This reference records compiled receiving support; RIGHTS.json names the compilation context. A connected runtime advertises current availability and selector support. These definitions supply no grant.

| Action | Resource profile | Owning operations | First-use targets | Compiled receiver |
|---|---|---|---|---|
| `abstraction.asks/notification.post` | `abstraction.rights/legacy-exact-string@1` | `abstraction.asks/notification@1#Post`, `abstraction.asks/notification-operator@1#Act` | `none` | served in compiled context |
| `abstraction.asks/question.answer` | `abstraction.rights/legacy-exact-string@1` | `abstraction.asks/responder@1#ListAnswerable`, `abstraction.asks/responder@1#Answer`, `abstraction.asks/responder@1#ListChoices`, `abstraction.asks/responder@1#AnswerChoice`, `abstraction.asks/responder@1#RequestAnswering` | Legacy `exact` | served in compiled context |
| `abstraction.asks/question.ask` | `abstraction.rights/legacy-exact-string@1` | `abstraction.asks/application@1#Ask`, `abstraction.asks/application@2#Ask`, `abstraction.asks/application@2#ShareChoice` | `none` | served in compiled context |

<!-- END GENERATED PERMISSION DEFINITIONS -->
