package asks

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"

	identity "github.com/openabstractions/abstraction-identity"
	logging "github.com/openabstractions/abstraction-logging/go"
)

// Serve is this layer's resident half as something another program can call.
//
// It used to be the body of asksd's main, which meant the only way to run the
// service was to run that binary. One program registered once per capability is
// the shape a Windows service, a launchd job and a systemd unit all activate,
// and a program cannot host a capability whose entry point is a main.
//
// The CLI is unaffected: `asks` is what an adopter types and it has not moved.
func Serve(args []string) error {
	fs := flag.NewFlagSet("asks", flag.ContinueOnError)
	endpoint := fs.String("endpoint", DefaultEndpoint(), "where applications connect")
	state := fs.String("state", DefaultStateDir(), "asks.json and admin.secret live here")
	if err := fs.Parse(args); err != nil {
		return err
	}
	slog.SetDefault(slog.New(logging.Default("asks")))

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
