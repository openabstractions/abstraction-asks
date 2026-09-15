namespace * abstraction.asks.api

// Shared asks concepts; native binding supplements are explicit below.
encoding json {
 escape="minimal"
 indent="2"
 map_keys="utf8-bytes"
 numbers="integer-decimal"
 opaque="verbatim"
 terminator="newline"
 duplicate_keys="refuse"
 depth_limit="64"
}
refusal {
 1: malformed(stage="grammar")
 2: bad_string(stage="grammar")
 3: number_spelling(stage="grammar")
 4: wrong_type(stage="grammar")
 5: depth_exceeded(stage="grammar")
 6: duplicate_key(stage="grammar")
 7: duplicate_field(stage="structure")
 8: unknown_field(stage="structure")
 9: missing_field(stage="structure")
 10: bad_enum(stage="structure")
 11: trailing_bytes(stage="document")
}
struct Question {
 1: required string asker
 2: required string key
 3: optional map<string,string> slots(omit="zero")
}(unknown_fields="refuse",doc="Own Ask fields. The service owns question text/options; callers supply a known key and slots. Native Ask.For carries optional peer observation evidence and remains a transport supplement, never caller authority.")
struct Request {
 1: required string op
 2: optional Question ask(omit="absent")
 3: optional bool wait(omit="zero")
 4: optional string id(omit="absent")
 5: optional string option(omit="absent")
 6: optional string admin(omit="absent")
}(document="true",unknown_fields="refuse",doc="Existing question operation concepts. Wait cancellation belongs to the native client context. This metadata descriptor does not replace the existing handwritten line transport.")
struct Answer {
 1: required string id
 2: optional bool pending(omit="zero")
 3: optional string option(omit="absent")
 4: optional bool yes(omit="zero")
 5: optional bool kept(omit="zero")
}(unknown_fields="refuse")
struct RecordMetadata {
 1: required string id
 2: required string asker
 3: required string key
 4: optional string about(omit="absent")
 5: required string text
 6: required list<string> options
 7: required string asked
 8: optional string option(omit="absent")
 9: optional string answered(omit="absent")
 10: optional bool kept(omit="zero")
 11: optional bool yes(omit="zero")
}(unknown_fields="refuse",doc="Own question-history fields; timestamps retain existing RFC3339 representation. Native Record includes Via/For Seen evidence from shared identity. Empty option means pending.")
struct ResponseMetadata {
 1: optional string code(omit="absent")
 2: optional string error(omit="absent")
 3: optional Answer answer(omit="absent")
 4: optional list<RecordMetadata> records(omit="zero")
}(unknown_fields="refuse",doc="Own response projection. Any nonempty code/error refuses, including unknown future codes. Evidence supplement and existing wire handling remain native; no generated complete service interface is claimed.")
const list<string> operations = ["ask", "pending", "answered", "answer", "forget"]
const list<string> refusal_codes = ["internal", "invalid_request", "caller_refused", "unknown_operation", "not_administrator", "withdrawn", "unknown_question", "unknown_option", "bad_slot", "nothing_pending", "no_record"]

enum ObservationOutcome {
 1: pending
 2: answered
 3: unknown
 4: gone
 5: invalid
 6: conflict
 7: forbidden
 8: unavailable
}(unknown="refuse")
struct ApplicationQuestion {
 1: required string request_key
 2: required string key
 3: required map<string,string> slots
}(unknown_fields="refuse",doc="Stable caller-owned request key (1..128 UTF-8 bytes, no control characters), known question key and exactly its declared slots. Each slot is one line of at most 200 Unicode characters. No asker or authority fields are accepted. The receiving account and executable-path proof scope the request. Retry uncertain submission with this same key and unchanged content; a fresh key requests a new admission.")
struct QuestionObservation {
 1: required ObservationOutcome outcome
 2: optional Answer answer(omit="absent")
}(unknown_fields="refuse",doc="Answer is present exactly for pending/answered. Pending has no option/yes/kept decision. Unknown is an unobserved key in this caller's scope. Gone retains a forgotten admission's key and never admits it again. Conflict means the key was previously used with different content. Unavailable includes storage/capacity refusal and establishes no human decision. Answered reports the recorded human choice; it grants no resource authority by itself.")
service QuestionApplication {
 QuestionObservation Ask(1:ApplicationQuestion question)(doc="Admit or replay one caller-scoped question. No automatic retry. Identical key/content returns the original admission even after an answer marked once and a service restart. Existing unscoped kept answers are not promoted.")
 QuestionObservation Observe(1:string request_key,2:i64 wait_ms)(doc="Observe only this caller's admission, optionally waiting 0..30000 milliseconds. Waiting expiry may return pending. Client timeout/cancellation ends waiting and neither answers nor cancels the question. Default SDK calls retain a five-second total waiting budget; callers explicitly requesting a longer budget must supply it through their transport/context API.")
}(wire_name="abstraction.asks/application@1",doc="Application-side human question admission and observation. Server derives caller scope and text, preserves native Via evidence, and owns a versioned bounded Book. Operator answering/history use the separately authorized QuestionOperator service; this application service exposes no administrative secret or operator methods.")


enum OperatorPageOutcome {
 1: page
 2: gap
 3: invalid
 4: forbidden
 5: unavailable
}(unknown="refuse")
struct OperatorPage {
 1: required OperatorPageOutcome outcome
 2: required list<RecordMetadata> records
 3: required string next
 4: required bool complete
}(unknown_fields="refuse",doc="Latest-book enumeration for an explicitly authorized operator, including pending and answered questions. Page has at most the requested 1..64 records and 256 KiB per encoded reply, with conservative record, indentation and envelope accounting. Complete means this snapshot was exhausted; otherwise next is nonempty. Refusals contain no records, next or complete flag. Opaque cursors (at most 256 UTF-8 bytes) bind receiving caller scope, host epoch and exact book revision. Change, restart or scope mismatch returns gap: restart from empty cursor. No historical change replay or stable snapshot across edits is promised. Native Via/For evidence stays server-side; display fields confer no authority.")
enum OperatorDecisionOutcome {
 1: answered
 2: conflict
 3: unknown
 4: invalid
 5: forbidden
 6: unavailable
}(unknown="refuse")
struct OperatorDecision {
 1: required OperatorDecisionOutcome outcome
 2: optional RecordMetadata record(omit="absent")
}(unknown_fields="refuse",doc="Record is present exactly for answered. Same ID and option replays the original decision including its timestamp and kept/once meaning; another option conflicts. Unknown means no retained question. Invalid includes an unknown option or invalid ID. Unavailable establishes no decision or noncommit claim: retry the same ID/option or inspect fresh history. Answering records a human choice, never a resource grant.")
enum OperatorRetirementOutcome {
 1: retired
 2: unknown
 3: invalid
 4: forbidden
 5: unavailable
}(unknown="refuse")
struct OperatorRetirement {
 1: required OperatorRetirementOutcome outcome
 2: optional RecordMetadata record(omit="absent")
}(unknown_fields="refuse",doc="Retired removes a retained pending or answered question from the Book. Record carries its last metadata on the retiring call and is absent when replaying an already retired ID. A pending question retired without an answer carries no decision. Admission tombstones remain: application replay and observation of its key report gone, and that key never admits again. Unknown means no retained or retired question. Invalid means a malformed ID. Unavailable establishes no retirement claim: retry the same ID or inspect fresh history.")
service QuestionOperator {
 OperatorPage ListQuestions(1:string cursor,2:i64 limit)(doc="Read bounded current-book pages. Empty cursor starts enumeration; a gap requires an explicit restart. Receiver checks same-account Program identity and its configured operator authorization on every call before reading the Book. Slow/disconnected callers retain no server enumeration state.")
 OperatorDecision AnswerQuestion(1:string id,2:string option)(doc="Record an explicitly authorized operator's choice using the catalog's existing options. ID and option are 1..128 UTF-8 bytes without control characters. Atomic same-option replay preserves the original decision. No automatic retry and no implicit resource authorization. A missing operator policy refuses; account identity alone is insufficient.")
 OperatorRetirement RetireQuestion(1:string id)(doc="Retire one question for an explicitly authorized operator. ID is 1..128 UTF-8 bytes without control characters. Authorization and context are rechecked inside the atomic edit before removal. Retiring an already retired ID replays retired without a record. No automatic retry; retirement grants and revokes no resource authority.")
}(wire_name="abstraction.asks/operator@1",doc="Operator history, answering and retirement on the configured application Book. Host must explicitly authorize the receiving operator; requests contain no credentials, authority claims or provider paths. Authorization callbacks are trusted service configuration and may use the rights service. Retirement and native Forget both retain admission tombstones.")
