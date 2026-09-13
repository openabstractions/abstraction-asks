# C++ application questions

`abstraction::asks_client` is supplied by the installed `abstraction_asks` CMake
package. It uses generated QuestionApplication codecs and shared identity IPC.

`asks::Client(endpoint)` has a fresh five-second budget for each call. The
`Client(endpoint, ipc::Deadline)` constructor retains one absolute budget;
`WithCancellation(token)` preserves shared cancellation. `Ask(question)` never
retries. After an uncertain reply, retain the request key and identical content.
`Observe(key, wait_ms)` accepts 0..30000 milliseconds; the transport budget still
bounds the call. Cancellation ends waiting without answering the question.

Pending/answered results carry validated answers; other outcomes carry none.
The receiver owns question text, choices, admission scope and persistence.
Operator answering is a separate native integration. An answer supplies a human
choice; resource authorization remains the receiving service's responsibility.

For resolution use optional `abstraction_facade_asks`, target
`abstraction::facade_asks`, header `abstraction/facade/asks.hpp`.
These are source packages with no claim of a published release version.
