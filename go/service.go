package asks

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/openabstractions/abstraction-identity/listen"
)

const (
	maxLine       = 64 << 10
	firstLineWait = 10 * time.Second
)

type Service struct {
	Endpoint string
	book     *Book
	admin    string
	l        listen.Listener
	mu       sync.Mutex
	closed   bool
	live     map[listen.Conn]struct{}
	wg       sync.WaitGroup
}

// ErrNoStateRoot refuses a start whose state folder is empty or relative:
// the questions and admin secret are kept in an absolute folder of this
// account and nowhere else (storage STO-L3).
var ErrNoStateRoot = errors.New("asks: no state root: asks.json and admin.secret need an absolute folder of this account")

func Start(endpoint, stateDir string) (*Service, error) {
	if err := startGuard(endpoint); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(stateDir) {
		return nil, fmt.Errorf("%w (given %q)", ErrNoStateRoot, stateDir)
	}
	book, err := LoadBook(filepath.Join(stateDir, "asks.json"))
	if err != nil {
		return nil, err
	}
	admin := random(32)
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stateDir, "admin.secret"), []byte(admin), 0o600); err != nil {
		return nil, err
	}
	l, err := listen.Listen(endpoint)
	if err != nil {
		return nil, err
	}
	s := &Service{Endpoint: endpoint, book: book, admin: hashOf(admin), l: l, live: map[listen.Conn]struct{}{}}
	go s.loop()
	return s, nil
}

func hashOf(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// Close returns once every connection is closed, so a listener started after
// it finds the name free.
func (s *Service) Close() error {
	err := s.l.Close()
	s.mu.Lock()
	s.closed = true
	for c := range s.live {
		c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
	return err
}

func (s *Service) loop() {
	for {
		c, err := s.l.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			slog.Warn("accept", "err", err)
			continue
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			c.Close()
			continue
		}
		s.live[c] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.wg.Done()
			s.serve(c)
			s.mu.Lock()
			delete(s.live, c)
			s.mu.Unlock()
		}()
	}
}

func (s *Service) serve(c listen.Conn) {
	kill := time.AfterFunc(firstLineWait, func() { c.Close() })
	k, err := listen.Receive(c, listen.Program, maxLine)
	kill.Stop()
	defer k.Close()
	if err != nil {
		reply(k, Response{Code: CodeCallerRefused, Error: "asks: refused, " + err.Error()})
		return
	}
	var req Request
	if err := json.Unmarshal(k.Frame, &req); err != nil {
		reply(k, Response{Code: CodeInvalidRequest, Error: "not a request: " + err.Error()})
		return
	}
	if req.Op != OpAsk {
		reply(k, s.administer(req, k.Caller))
		return
	}
	s.ask(k, req)
}

func reply(c io.Writer, r Response) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = c.Write(append(raw, '\n'))
	return err
}

func (s *Service) ask(k *listen.Call, req Request) {
	if req.Ask == nil {
		reply(k, Response{Code: CodeInvalidRequest, Error: "asks: nothing was asked"})
		return
	}
	r, asked, err := s.book.Ask(*req.Ask, k.Caller)
	if err != nil {
		reply(k, failure(err))
		return
	}
	if asked {
		slog.Info("pending", "id", r.ID, "text", r.Text)
	}
	if r.Pending() {
		if !req.Wait {
			reply(k, Response{Answer: &Answer{ID: r.ID, Pending: true}})
			return
		}
		decided, stopWaiting := s.book.ObserveQuestionWait(r.ID)
		select {
		case <-decided:
			stopWaiting()
		case <-k.Gone():
			stopWaiting()
			return
		}
		if r, _ = s.book.Get(r.ID); r.Pending() {
			reply(k, Response{Code: CodeWithdrawn, Error: "asks: the question was withdrawn"})
			return
		}
	}
	reply(k, Response{Answer: &Answer{ID: r.ID, Option: r.Option, Yes: r.Yes, Kept: r.Kept}})
}

func (s *Service) administer(req Request, by listen.Seen) Response {
	if subtle.ConstantTimeCompare([]byte(hashOf(req.Admin)), []byte(s.admin)) != 1 {
		return Response{Code: CodeNotAdministrator, Error: "asks: not the person's tool"}
	}
	fail := func(err error) Response { return failure(err) }
	switch req.Op {
	case OpPending:
		return Response{Records: s.book.List(true)}
	case OpAnswered:
		return Response{Records: s.book.List(false)}
	case OpAnswer:
		r, err := s.book.Answer(req.ID, req.Option)
		if err != nil {
			return fail(err)
		}
		slog.Info("answered", "id", r.ID, "option", r.Option, "by", by.Path)
		return Response{Records: []Record{r}}
	case OpForget:
		if err := s.book.Forget(req.ID); err != nil {
			return fail(err)
		}
		slog.Info("forgotten", "id", req.ID, "by", by.Path)
		return Response{}
	}
	return Response{Code: CodeUnknownOperation, Error: "asks: unknown op " + req.Op}
}

// startGuard refuses an endpoint whose transport can never prove the calling
// program: a macOS socket refuses, an xpc: endpoint passes, and each request
// is still checked against its own peer evidence.
func startGuard(endpoint string) error {
	if err := listen.CanEver(endpoint, listen.Program); err != nil {
		return fmt.Errorf("asks: a question names who asked it, and this machine cannot say which program is calling: %w", err)
	}
	return nil
}
