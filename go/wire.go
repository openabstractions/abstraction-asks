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
}

func (r Record) Pending() bool { return r.Option == "" }
