# abstraction-asks

**In development.** Tagged `go/v0.1.1`, but no conformance scenario cites this
layer, and the API carries no stability promise.

A question an application puts to a person is a record with options, phrased
by the service and not the asker, answered once, and kept — so the same
question is not asked twice and *never* is an answer to every question.

## The problem

Every application that needs a person's answer draws its own dialog, phrases
its own question, and forgets the answer when it exits. Here an application
names a question the service already knows; it never writes the prompt. The
answer is kept, so the same question is not asked twice.

## Words

| word | meaning |
|---|---|
| **question** | one name from the closed list in [`go/questions.go`](go/questions.go), with the slots it declares |
| **ask** | an application filling a question's slots; pending until a person answers, and one record however many times it is asked |
| **answer** | a record the person keeps: the text as the person saw it, who asked, what the platform proved about who connected, and when |
| **kept** | an answer that stops the question being asked again; `once` is not kept, `never` always is |
| **via** / **for** | the program that connected, bound by this service; and the program it asks on behalf of, on its word |

No rule on this page carries a tag; the contract below is held by the tests
named in it.

## Obtain

- **Go.** `go get github.com/openabstractions/abstraction-asks/go`. Current
  release [`go/v0.1.1`](https://github.com/openabstractions/abstraction-asks/releases/tag/go%2Fv0.1.1).
  Standard library plus
  [abstraction-identity](https://github.com/openabstractions/abstraction-identity)
  (which brings `golang.org/x/sys`).
- **Python, C++.** None.

## Run

    go build -o bin/ ./go/cmd/...
    bin/asksd                        # foreground; prints where it listens

An application asks; here `dl` before its first fetch from a host:

    $ dl http://example.com/
    dl has asked whether it may fetch from example.com
      a person answers:  asks pending  then  asks answer 0da747 allow|once|refuse|never

    $ asks pending
    0da747  dl wants to fetch from example.com
            asked 1s ago via ...\dl.exe  ...  no signature
            answers: allow, once, refuse, never
    $ asks answer 0da747 allow
    0da747: allow, and dl will not be asked this again

The second `dl http://example.com/` asks nothing. `asks answered` lists every
answer given and when; `asks forget <id>` withdraws one, and the question may
be asked again. `asks questions` prints the closed list an application can
choose from.

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
- **Answering is the person's act**, proven by possessing `admin.secret`, a file
  only that user can read. Applications cannot read the queue.
- **Every operation binds its caller before it reads the request.** All five
  go through `listen.Receive`; a caller the kernel cannot identify to
  `listen.Program` is refused with the reason (`asks: refused, identity: ...`)
  and its request is never parsed. The service refuses to start on a machine
  that cannot bind (`identity.CanEver`). A test sends every operation with a
  valid credential and no identity and expects five refusals.
- **`via` is the program that connected, bound by this service. `for` is the
  program it asks on behalf of, on its word.** `rights` asking for `keepawake`
  is recorded `via rightsd, for keepawake`; `for` is what `rightsd` bound at its
  own end and forwarded, and the person is told it is the asker's word. This is
  HTTP's `Via` beside `X-Forwarded-For`: each hop binds its own peer and copies
  nothing from the message into `via`.
- **This holds among programs sharing one kernel and one user account.**

## Today

**Go only.** Verified on Windows 11 over a named pipe. **Not examined: Linux,
macOS** — the unix socket listener is shared with `rights` and has never run
there either.

Two callers:
[`rights`](https://github.com/openabstractions/abstraction-rights) — a
registration is `rights.register`, and its answers are not kept, because a
kept *allow* would let any later process with the same name in without the
person seeing its path; and `dl` — `download.reach` before the first fetch from
a host. Without a running `asksd`, `dl` says so and fetches unasked.

What may break: the pipe name belongs to the first listener
(`FILE_FLAG_FIRST_PIPE_INSTANCE`): a second `asksd` on the same name refuses to
start, and so does one started while a client of the previous one has not hung
up. There is no notification: a person finds out there is a question by running
`asks pending`, or because the application said so. Pending questions are
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
registers through it, and
[abstraction-download](https://github.com/openabstractions/abstraction-download)'s
`dl` asks before reaching a host.

One layer of [openabstractions](https://github.com/openabstractions/abstractions).
Every layer names one thing local tools rebuild on their own; the name means the
same in each language that implements it, and the conformance scenarios are what
hold an implementation to it.

## Requirements

Go 1.26 or newer. Windows 11 verified; Linux and macOS not run.

## Licence

Apache-2.0. See [LICENSE](https://github.com/openabstractions/abstraction-asks/blob/main/LICENSE).
