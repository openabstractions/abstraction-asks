package asks

import (
	"time"

	"github.com/openabstractions/abstraction-identity/listen"
)

const (
	OpAsk      = "ask"
	OpPending  = "pending"
	OpAnswered = "answered"
	OpAnswer   = "answer"
	OpForget   = "forget"
)

type Request struct {
	Op     string `json:"op"`
	Ask    *Ask   `json:"ask,omitempty"`
	Wait   bool   `json:"wait,omitempty"`
	ID     string `json:"id,omitempty"`
	Option string `json:"option,omitempty"`
	Admin  string `json:"admin,omitempty"`
}

type Response struct {
	Code    string   `json:"code,omitempty"`
	Error   string   `json:"error,omitempty"`
	Answer  *Answer  `json:"answer,omitempty"`
	Records []Record `json:"records,omitempty"`
}

type Ask struct {
	Asker string            `json:"asker"`
	Key   string            `json:"key"`
	Slots map[string]string `json:"slots,omitempty"`
	For   *listen.Seen      `json:"for,omitempty"`
}

type Answer struct {
	ID      string `json:"id"`
	Pending bool   `json:"pending,omitempty"`
	Option  string `json:"option,omitempty"`
	Yes     bool   `json:"yes,omitempty"`
	Kept    bool   `json:"kept,omitempty"`
}

type Record struct {
	ID       string       `json:"id"`
	Asker    string       `json:"asker"`
	Key      string       `json:"key"`
	About    string       `json:"about,omitempty"`
	Text     string       `json:"text"`
	Options  []string     `json:"options"`
	Asked    time.Time    `json:"asked"`
	Via      listen.Seen  `json:"via"`
	For      *listen.Seen `json:"for,omitempty"`
	Option   string       `json:"option,omitempty"`
	Answered time.Time    `json:"answered,omitzero"`
	Kept     bool         `json:"kept,omitempty"`
	Yes      bool         `json:"yes,omitempty"`
	// Publisher and PublisherTrusted are the requester's code signer as the
	// admitting service verified it at admission (ASK-A6). Only a service
	// sets them; the legacy line protocol never does.
	Publisher        string `json:"publisher,omitempty"`
	PublisherTrusted bool   `json:"publisher_trusted,omitempty"`
	// ProgramDescription and ProgramOriginalName are the FileDescription and
	// OriginalFilename of the requester's signed version resource, recorded
	// with a trusted signature only (ASK-A6).
	ProgramDescription  string `json:"program_description,omitempty"`
	ProgramOriginalName string `json:"program_original_name,omitempty"`
	// AnsweredBy names the service the recorded answer arrived through,
	// AnsweredByOperator or AnsweredByResponder (ASK-O7). Empty while pending
	// and for an answer recorded before the service kept it.
	AnsweredBy string `json:"answered_by,omitempty"`
	// Answerer is the subject program of the answerer that recorded a
	// responder answer, bound by the service from the answering peer: the
	// actor of an answer given for the person (ASK-O7, ASK-R5). Empty for an
	// operator's answer, while pending, and for an answer recorded before the
	// service kept it.
	Answerer string `json:"answerer,omitempty"`
	// AskerProgram is the bound program the application service derived the
	// admission's subject from (ASK-A1), the clean absolute executable path
	// the platform proved. Asker is its display name, AskerDisplay of it.
	// Empty for a record admitted before the service kept it, for the
	// runtime's own questions and for the legacy line protocol.
	AskerProgram string `json:"asker_program,omitempty"`
	// Permission is set only by trusted runtime admission and stays immutable.
	Permission      *PermissionRequest `json:"permission,omitempty"`
	TypedPermission *TypedPermission   `json:"typed_permission,omitempty"`
	// Set only after canonical @1/@2 decoding; never serialized or caller supplied.
	legacyPermission bool
}

// The services an application-profile answer arrives through (ASK-O7).
const (
	AnsweredByOperator  = "operator"
	AnsweredByResponder = "responder"
)

func (r Record) Pending() bool { return r.Option == "" }
