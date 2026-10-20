namespace * abstraction.asks.presenter

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
enum LeaseOutcome {
 1: held
 2: idle
 3: stale
 4: forbidden
 5: unavailable
 6: released
 7: busy
 8: completion
 9: invalid
 10: unknown
}(unknown="refuse",reader="act")

struct Lease {
 1: required LeaseOutcome outcome
 2: optional string id(length="1..128",omit="zero")
 3: optional string token(length="96..96",omit="zero")
 4: optional i64 expires_ms(omit="zero")
}(document="true",unknown_fields="refuse")

struct HoldNotice {
 1: required LeaseOutcome outcome
 2: optional string program(length="1..4096",omit="zero")
 3: optional string id(length="1..128",omit="zero")
 4: optional string token(length="32..32",omit="zero")
 5: optional i64 expires_ms(omit="zero")
}(unknown_fields="refuse")

service Presenter {
 HoldNotice ReadHoldNotice()(readonly="true",remote="local")
 Lease AckHoldNotice(1:string token(length="32..32"))(idempotent="true",remote="local")
 Lease Start(1:string token(length="96..96"))(idempotent="true",remote="local")
 Lease Claim(1:string id(length="..128"))(idempotent="false",remote="local")
 Lease BeginProviderCompletion(1:string token(length="96..96"))(idempotent="false",remote="local")
 Lease Renew(1:string token(length="96..96"))(idempotent="false",remote="local")
 Lease Release(1:string token(length="96..96"),2:bool shown)(idempotent="false",remote="local")
}(wire_name="abstraction.asks/presenter@1",doc="Local operator presentation lease. Native account, process and interactive session evidence bind every call. A lease conveys no answer authority.")
