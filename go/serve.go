package asks

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"

	identity "github.com/openabstractions/abstraction-identity"
	logging "github.com/openabstractions/abstraction-logging/go"
)

const asksUsage = `Usage: openabstractions serve asks [options]

Holds pending questions for a person to answer, and keeps each answer for
its asker.

  --endpoint EP   where applications connect (default: the installed
                  runtime's pipe for this service)
  --state DIR     directory holding asks.json and admin.secret (default:
                  this account's state directory)

Exit codes: 0 a clean stop, 1 startup failure, 2 usage error.
`

func isHelp(arg string) bool { return arg == "--help" || arg == "-h" || arg == "help" }

// ErrUsage wraps an error Serve returns for a bad flag or argument, after it
// has already printed asksUsage. A caller that hosts several capabilities
// tells this apart from a startup failure with errors.Is(err, ErrUsage).
var ErrUsage = errors.New("asks: usage error")

// Serve is this layer's resident half as something another program can call.
//
// It used to be the body of asksd's main, which meant the only way to run the
// service was to run that binary. One program registered once per capability is
// the shape a Windows service, a launchd job and a systemd unit all activate,
// and a program cannot host a capability whose entry point is a main.
//
// The CLI is unaffected: `asks` is what an adopter types and it has not moved.
func Serve(args []string) error {
	if len(args) > 0 && isHelp(args[0]) {
		_, err := io.WriteString(os.Stdout, asksUsage)
		return err
	}
	fs := flag.NewFlagSet("asks", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	// asksUsage already documents every option; the fallback on a genuine
	// parse error prints it once, not a second time as flag.PrintDefaults'
	// own single-dash listing.
	var usageErr error
	fs.Usage = func() { _, usageErr = fmt.Fprint(fs.Output(), asksUsage) }
	endpoint := fs.String("endpoint", DefaultEndpoint(), "where applications connect")
	state := fs.String("state", DefaultStateDir(), "asks.json and admin.secret live here")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return usageErr
		}
		return errors.Join(fmt.Errorf("%w: %v", ErrUsage, err), usageErr)
	}
	// asksd can start with no logging runtime running. Its own diagnostics go
	// to the explicitly selected environment chain, heard on stderr when nothing
	// is configured [LOG-S8].
	slog.SetDefault(slog.New(logging.LegacyDefault("asks")))

	s, err := Start(*endpoint, *state)
	if err != nil {
		return err
	}
	defer s.Close()
	slog.Info("listening", "endpoint", *endpoint, "answers", *state)
	if limits := identity.Ceiling(); limits.Bindable {
		slog.Info("questions show who asked", "how", limits.String())
	} else {
		slog.Info("this machine cannot say which program asks", "why", limits.Binding)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	return nil
}
