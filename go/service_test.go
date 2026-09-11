package asks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

func start(t *testing.T, dir string) (*Service, *Client, *Client) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("UNPROVEN successful service calls: current Darwin transport cannot meet Program proof; XPC is planned. TestUnsupportedPlatformRefusesBeforeProvider verifies refusal")
	}
	endpoint := testEndpoint(t)
	s, err := Start(endpoint, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	admin, _ := os.ReadFile(filepath.Join(dir, "admin.secret"))
	return s, &Client{Endpoint: endpoint}, &Client{Endpoint: endpoint, Admin: string(admin)}
}

// Socket pathname limits include the temporary directory. Darwin's default
// temporary root plus a descriptive subtest name can exceed that limit.
func testEndpoint(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\asks-test-%d-%s`, os.Getpid(), t.Name())
	}
	dir, err := os.MkdirTemp("/tmp", "asks-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

func TestUnsupportedPlatformRefusesBeforeProvider(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin transport proof ceiling")
	}
	if err := identity.CanEver(listen.Program); !errors.Is(err, identity.ErrNotProven) {
		t.Fatalf("expected current transport's Program refusal, got %v", err)
	}
	dir := filepath.Join(t.TempDir(), "provider-not-created")
	endpoint := testEndpoint(t)
	s, err := Start(endpoint, dir)
	if s != nil {
		s.Close()
		t.Fatal("unsupported service started")
	}
	if !errors.Is(err, identity.ErrNotProven) {
		t.Fatalf("expected proof refusal, got %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("provider state touched: %v", err)
	}
	if _, err := os.Stat(endpoint); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("listener created: %v", err)
	}
	t.Log("UNPROVEN successful Darwin service calls; startup refuses before provider state or endpoint creation")
}

var reach = Ask{Asker: "dl", Key: "download.reach", Slots: map[string]string{"host": "example.com"}}

func TestAnAnswerIsKeptAndTheAskerIsNotAskedAgain(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, app, tool := start(t, dir)

	first, err := app.Ask(ctx, reach)
	if err != nil || !first.Pending {
		t.Fatalf("first ask = %+v, %v", first, err)
	}
	again, _ := app.Ask(ctx, reach)
	if again.ID != first.ID {
		t.Fatalf("asking twice made two questions: %s and %s", first.ID, again.ID)
	}
	other, _ := app.Ask(ctx, Ask{Asker: "curl", Key: "download.reach", Slots: map[string]string{"host": "example.com"}})
	if other.ID == first.ID {
		t.Fatal("two applications asking the same thing share one question")
	}

	awaited := make(chan Answer, 1)
	go func() {
		a, err := app.Await(ctx, reach)
		if err != nil {
			t.Error(err)
		}
		awaited <- a
	}()
	ps, _ := tool.Pending()
	if len(ps) != 2 {
		t.Fatalf("pending = %v", ps)
	}
	if _, err := tool.Answer(first.ID, "allow"); err != nil {
		t.Fatal(err)
	}
	if a := <-awaited; !a.Yes || !a.Kept || a.Option != "allow" {
		t.Fatalf("awaited = %+v", a)
	}

	s.Close()
	_, app, tool = start(t, dir)
	a, err := app.Ask(ctx, reach)
	if err != nil || a.Pending || !a.Yes {
		t.Fatalf("after a restart the kept answer was not found: %+v, %v", a, err)
	}
	if ps, _ = tool.Pending(); len(ps) != 1 || ps[0].Asker != "curl" {
		t.Fatalf("the unanswered question did not survive the restart: %v", ps)
	}
	if err := tool.Forget(a.ID); err != nil {
		t.Fatal(err)
	}
	if a, _ = app.Ask(ctx, reach); !a.Pending {
		t.Fatalf("after forget the question was not asked again: %+v", a)
	}
}

func TestNeverIsAnAnswerToEveryQuestionAndOnceIsNotKept(t *testing.T) {
	ctx := context.Background()
	_, app, tool := start(t, t.TempDir())
	a, _ := app.Ask(ctx, reach)
	if _, err := tool.Answer(a.ID, "once"); err != nil {
		t.Fatal(err)
	}
	if a, _ = app.Ask(ctx, reach); !a.Pending {
		t.Fatalf("'once' was kept: %+v", a)
	}
	if _, err := tool.Answer(a.ID, "never"); err != nil {
		t.Fatal(err)
	}
	if a, _ = app.Ask(ctx, reach); a.Pending || a.Yes || !a.Kept {
		t.Fatalf("'never' did not stick: %+v", a)
	}
	if rs, _ := tool.Answered(); len(rs) != 2 {
		t.Fatalf("the person keeps %d answers, wanted 2", len(rs))
	}
}

func TestAnApplicationPhrasesNothing(t *testing.T) {
	ctx := context.Background()
	_, app, tool := start(t, t.TempDir())
	if _, err := app.Ask(ctx, Ask{Asker: "x", Key: "x.anything"}); err == nil {
		t.Fatal("an unknown question was accepted")
	}
	if _, err := app.Ask(ctx, Ask{Asker: "x", Key: "download.reach"}); err == nil {
		t.Fatal("a missing slot was accepted")
	}
	if _, err := app.Ask(ctx, Ask{Asker: "x", Key: "download.reach", Slots: map[string]string{"host": "a\nb"}}); err == nil {
		t.Fatal("a slot with a newline was accepted")
	}
	a, _ := app.Ask(ctx, reach)
	if _, err := tool.Answer(a.ID, "yes"); err == nil {
		t.Fatal("an option the question does not offer was accepted")
	}
	if _, err := app.Pending(); err == nil {
		t.Fatal("an application read the person's queue")
	}
}

func TestAWaitingAskerIsReleasedWhenItsContextEnds(t *testing.T) {
	_, app, _ := start(t, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := app.Await(ctx, reach); done <- err }()
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("err = %v", err)
	}
}
