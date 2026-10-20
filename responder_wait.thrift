namespace * abstraction.asks.responderwait

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
 11: bad_value(stage="structure")
 12: trailing_bytes(stage="document")
}
enum Outcome {
 1: page
 2: gap
 3: invalid
 4: forbidden
 5: unavailable
}(unknown="refuse",reader="act")

typedef string QuestionID(length="1..128")
struct WaitingPage {
 1: required Outcome outcome
 2: required list<QuestionID> ids(length="0..64")
 3: required string next(length="0..256")
 4: required bool complete
}(document="true",unknown_fields="refuse")

service ResponderWait {
 WaitingPage ListWaiting(1:string cursor(length="0..256"),2:i64 limit(range="1..64"))(readonly="true",remote="local")
}(wire_name="abstraction.asks/responder-wait@1",doc="Private opt-in read of exact live question waits for an enrolled responder. Uses the answerable responder page and grants no authority.")
