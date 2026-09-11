package asks

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openabstractions/abstraction-identity/listen"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestClientRefusalCompatibility(t *testing.T) {
	for _, tc := range []struct{ name, frame, code, message string }{
		{"legacy", `{"error":"old server refusal"}`, "", "old server refusal"},
		{"unknown", `{"code":"future_code","error":"new refusal"}`, "future_code", "new refusal"},
		{"code_only", `{"code":"future_code"}`, "future_code", ""},
		{"known", `{"code":"` + CodeUnknownQuestion + `","error":"human reason"}`, CodeUnknownQuestion, "human reason"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at := filepath.Join(t.TempDir(), "service.sock")
			if runtime.GOOS == "windows" {
				at = listen.Endpoint(fmt.Sprintf("asks-codes-%d-%d", os.Getpid(), time.Now().UnixNano()))
			}
			listener, err := listen.Listen(at)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan error, 1)
			go func() {
				c, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer c.Close()
				if _, err := bufio.NewReader(c).ReadString('\n'); err != nil {
					done <- err
					return
				}
				_, err = fmt.Fprintln(c, tc.frame)
				done <- err
			}()
			_, err = (&Client{Endpoint: at}).do(context.Background(), Request{})
			var remote *RemoteError
			if !errors.As(err, &remote) {
				t.Fatalf("not a remote refusal: %v", err)
			}
			if remote.Code != tc.code || remote.Message != tc.message {
				t.Fatalf("lost refusal: %+v", remote)
			}
			if err.Error() == "" {
				t.Fatal("empty refusal text")
			}
			if serverErr := <-done; serverErr != nil {
				t.Fatal(serverErr)
			}
		})
	}
}

func TestRefusalWireCompatibility(t *testing.T) {
	raw, err := json.Marshal(Response{Code: CodeUnknownQuestion, Error: "unchanged"})
	if err != nil {
		t.Fatal(err)
	}
	var old struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	if old.Error != "unchanged" {
		t.Fatal("old client lost error")
	}
	if (Response{}).Err() != nil {
		t.Fatal("success became refusal")
	}
}

func TestRefusalKeepsSentinel(t *testing.T) {
	original := fmt.Errorf("context: %w", ErrUnknownQuestion)
	raw, err := json.Marshal(failure(original))
	if err != nil {
		t.Fatal(err)
	}
	var received Response
	if err := json.Unmarshal(raw, &received); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(received.Err(), ErrUnknownQuestion) {
		t.Fatalf("identity lost: %v", received.Err())
	}
	if received.Error != original.Error() {
		t.Fatal("legacy text changed")
	}
}

func TestServiceRefusalHasCode(t *testing.T) {
	_, app, _ := start(t, t.TempDir())
	_, err := app.Ask(context.Background(), Ask{Asker: "example", Key: "unknown"})
	if !errors.Is(err, ErrUnknownQuestion) {
		t.Fatalf("question identity lost: %v", err)
	}
	_, err = app.Ask(context.Background(), Ask{Asker: "example", Key: "download.reach"})
	if !errors.Is(err, ErrBadSlot) {
		t.Fatalf("slot identity lost: %v", err)
	}
}

func TestEmptyNativeErrorStillRefusesLegacyClient(t *testing.T) {
	out := failure(errors.New(""))
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var old struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	if old.Error == "" || out.Code != CodeInternal {
		t.Fatalf("legacy success: %s", raw)
	}
}
