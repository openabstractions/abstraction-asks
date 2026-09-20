// Package application hosts caller-bound questions over shared framed IPC.
package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
	"os/user"

	"sync"
	"time"
)

const MaxFrameBytes = 1 << 20

type Host struct {
	lifecycle sync.Mutex
	serving   bool
	listener  listen.Listener
	owner     string
	book      *asks.Book
	ctx       context.Context
	cancel    context.CancelFunc
	once      sync.Once
	workers   sync.WaitGroup
	slots     chan struct{}
	OnError   func(error)
	// Assign before Serve. Called when admission stops, before calls drain.
	OnStopped     func()
	operator      AuthorizeOperator
	operatorEpoch string
	askPolicy     AskPolicy
}

// Listen requires a separate application-profile Book owned by this host.
func Listen(endpoint string, book *asks.Book) (*Host, error) {
	if !book.ApplicationProfile() {
		return nil, errors.New("asks: application-profile Book required")
	}
	owner, e := user.Current()
	if e != nil {
		return nil, e
	}
	if owner.Uid == "" {
		return nil, errors.New("asks: service principal unavailable")
	}
	var epoch [16]byte
	if _, e = rand.Read(epoch[:]); e != nil {
		return nil, e
	}
	l, e := listen.Listen(endpoint)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Host{listener: l, owner: owner.Uid, book: book, ctx: ctx, cancel: cancel, slots: make(chan struct{}, 32), operatorEpoch: hex.EncodeToString(epoch[:])}, nil
}
func (h *Host) Close() error {
	var e error
	h.once.Do(func() { h.cancel(); e = h.listener.Close() })
	return e
}
func (h *Host) Serve(ctx context.Context) error {
	h.lifecycle.Lock()
	if h.serving {
		h.lifecycle.Unlock()
		return errors.New("asks: host already served")
	}
	h.serving = true
	h.lifecycle.Unlock()
	stop := context.AfterFunc(ctx, func() { h.Close() })
	defer stop()
	defer h.workers.Wait()
	defer func() {
		if h.OnStopped != nil {
			h.OnStopped()
		}
	}()
	defer h.Close()
	for {
		conn, e := h.listener.Accept()
		if e != nil {
			if ctx.Err() != nil || h.ctx.Err() != nil {
				return nil
			}
			return e
		}
		select {
		case h.slots <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		h.workers.Add(1)
		go func() {
			defer h.workers.Done()
			defer func() { <-h.slots }()
			defer conn.Close()
			callCtx, cancel := context.WithTimeout(h.ctx, 35*time.Second)
			defer cancel()
			call, e := listen.ReceiveFramed(callCtx, conn, listen.Program, MaxFrameBytes)
			if call != nil {
				defer call.Close()
			}
			if e == nil {
				var reply []byte
				reply, e = wire.ServeEndpoint(call.Frame, "openabstractions", "",
					&wire.QuestionApplicationDispatcher{Handler: &receiver{host: h, call: call}},
					&wire.QuestionOperatorDispatcher{Handler: &operatorReceiver{receiver: receiver{host: h, call: call}, ctx: call.WaitContext()}})
				if e == nil {
					e = call.Reply(reply)
				}
			}
			if e != nil && h.OnError != nil && h.ctx.Err() == nil {
				h.OnError(e)
			}
		}()
	}
}
