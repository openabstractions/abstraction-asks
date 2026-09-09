package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	asks "github.com/openabstractions/abstraction-asks/go"
	identity "github.com/openabstractions/abstraction-identity"
	logging "github.com/openabstractions/abstraction-logging/go"
)

func main() {
	endpoint := flag.String("endpoint", asks.DefaultEndpoint(), "where applications connect")
	state := flag.String("state", asks.DefaultStateDir(), "asks.json and admin.secret live here")
	flag.Parse()
	slog.SetDefault(slog.New(logging.Default("asksd")))

	s, err := asks.Start(*endpoint, *state)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	slog.Info("listening", "endpoint", *endpoint, "answers", *state)
	if limits := identity.Ceiling(); limits.Bindable {
		slog.Info("questions show who asked", "how", limits.String())
	} else {
		slog.Info("this machine cannot say which program asks", "why", limits.Binding)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	s.Close()
}
