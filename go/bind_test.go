package asks

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"

	identity "github.com/openabstractions/abstraction-identity"
)

type unbound struct{ net.Conn }

func (unbound) Bind() (*identity.Binding, error) { return nil, identity.ErrNoBinding }

func TestUnboundCallerRefusedBeforeProvider(t *testing.T) {
	// Deliberately no book/provider: refusal must precede every entry point's
	// provider access, including on platforms that refuse Start altogether.
	s := &Service{}
	for _, op := range []string{OpAsk, OpPending, OpAnswered, OpAnswer, OpForget} {
		t.Run(op, func(t *testing.T) {
			resp := serveUnbound(t, s, Request{Op: op, Ask: &reach})
			if resp.Code != CodeCallerRefused || !strings.Contains(resp.Error, identity.ErrNoBinding.Error()) {
				t.Fatalf("unbound caller not refused: %+v", resp)
			}
		})
	}
}

func TestEveryEntryPointRefusesACallerTheKernelCannotIdentify(t *testing.T) {
	s, app, tool := start(t, t.TempDir())
	asked, err := app.Ask(context.Background(), reach)
	if err != nil {
		t.Fatal(err)
	}
	ghost := Ask{Asker: "ghost", Key: "download.reach", Slots: map[string]string{"host": "example.com"}}
	every := []Request{
		{Op: OpAsk, Ask: &ghost},
		{Op: OpPending, Admin: tool.Admin},
		{Op: OpAnswered, Admin: tool.Admin},
		{Op: OpAnswer, Admin: tool.Admin, ID: asked.ID, Option: "allow"},
		{Op: OpForget, Admin: tool.Admin, ID: asked.ID},
	}
	for _, req := range every {
		resp := serveUnbound(t, s, req)
		if !strings.HasPrefix(resp.Error, "asks: refused, ") || !strings.Contains(resp.Error, identity.ErrNoBinding.Error()) {
			t.Errorf("%s with a valid credential and no identity: %+v", req.Op, resp)
		}
	}
	ps, _ := tool.Pending()
	if len(ps) != 1 || ps[0].ID != asked.ID {
		t.Fatalf("an unidentified caller changed the book: %+v", ps)
	}
}

func serveUnbound(t *testing.T, s *Service, req Request) Response {
	t.Helper()
	server, client := net.Pipe()
	defer client.Close()
	go s.serve(unbound{server})
	raw, _ := json.Marshal(req)
	if _, err := client.Write(append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
	var resp Response
	if err := json.NewDecoder(client).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}
