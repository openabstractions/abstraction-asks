package main

import (
	"fmt"
	"os"

	asks "github.com/openabstractions/abstraction-asks/go"
)

func main() {
	if err := asks.Serve(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
