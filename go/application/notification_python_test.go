package application

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

// pythonNotificationBook serves the generated Python client's frames from the
// Go dispatcher and Book. It records the person's action on the first Observe.
type pythonNotificationBook struct {
	book    *asks.Book
	program string
	acted   bool
}

func (p *pythonNotificationBook) Post(q wire.NotificationRequest) (wire.NotificationObservation, error) {
	r, _, err := p.book.PostNotificationFrom("python-fixture", p.program, q, listen.Seen{}, asks.Publisher{}, nil)
	return r, err
}
func (p *pythonNotificationBook) Observe(ref wire.RequestRef, _ int64) (wire.NotificationObservation, error) {
	if !p.acted {
		seen, err := p.book.ObserveNotification("python-fixture", ref)
		if err != nil {
			return seen, err
		}
		if _, err = p.book.DecideNotification(seen.ID, asks.NotificationAct, "open", p.program, nil); err != nil {
			return seen, err
		}
		p.acted = true
	}
	return p.book.ObserveNotification("python-fixture", ref)
}
func (p *pythonNotificationBook) Withdraw(ref wire.RequestRef) (wire.NotificationObservation, error) {
	d, err := p.book.WithdrawOwnNotification("python-fixture", ref, p.program, nil)
	if err != nil || d.Record == nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeUnavailable}, err
	}
	return wire.NotificationObservation{Outcome: wire.NotificationOutcomeWithdrawn, ID: d.Record.ID, Result: d.Record.Result}, nil
}
func (p *pythonNotificationBook) Acknowledge(ref wire.RequestRef, result wire.NotificationResult) (wire.ChoiceAcknowledgement, error) {
	return p.book.AcknowledgeNotification("python-fixture", ref, result, nil)
}

// The generated Python client exchanges real frames with the Go dispatcher
// and Book: post, replay, conflict, an action, acknowledgement and withdrawal.
func TestNotificationPythonGeneratedClientAndBook(t *testing.T) {
	python, args := "python3", []string{}
	if runtime.GOOS == "windows" {
		python, args = "py", []string{"-3.12"}
	}
	if _, err := exec.LookPath(python); err != nil {
		t.Skip("Python interpreter unavailable")
	}
	b, _ := book(t)
	actor, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pythonPath := generatedClientPythonPath(t, python, args)
	args = append(args, "testdata/notification_client.py")
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Env = append(os.Environ(), "PYTHONPATH="+pythonPath, "PYTHONIOENCODING=utf-8")
	cmd.Stderr = os.Stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	provider := &pythonNotificationBook{book: b, program: filepath.Clean(actor)}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), MaxFrameBytes)
	for scanner.Scan() {
		frame, err := base64.StdEncoding.DecodeString(scanner.Text())
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		reply, err := wire.ServeEndpoint(frame, "test-provider", "", &wire.NotificationApplicationDispatcher{Handler: provider})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if _, err = fmt.Fprintln(input, base64.StdEncoding.EncodeToString(reply)); err != nil {
			cancel()
			t.Fatal(err)
		}
	}
	if err := scanner.Err(); err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if !provider.acted {
		t.Fatal("the Python client never observed")
	}
}
