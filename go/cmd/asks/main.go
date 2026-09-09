package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
)

const usage = `asks — every question an application has for you, in one place

  asks pending                 questions waiting for an answer
  asks answer <id> <option>    answer one; 'never' is always an option
  asks answered                every answer you have given, and when
  asks forget <id>             withdraw an answer; the question may be asked again
  asks questions               the questions an application is able to put
`

func main() {
	endpoint := flag.String("endpoint", asks.DefaultEndpoint(), "where the service listens")
	state := flag.String("state", asks.DefaultStateDir(), "where the service keeps admin.secret")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if args[0] == "questions" {
		for _, q := range asks.Questions {
			fmt.Printf("%-18s %s\n%19s answers: %s\n", q.Key, indent(q.Text, 19), "", list(q.OptionNames()))
		}
		return
	}

	admin, err := os.ReadFile(filepath.Join(*state, "admin.secret"))
	if err != nil {
		fail("no admin.secret in %s: is asksd running with -state there?", *state)
	}
	c := &asks.Client{Endpoint: *endpoint, Admin: string(admin)}

	need := func(n int) {
		if len(args) != n+1 {
			flag.Usage()
			os.Exit(2)
		}
	}
	switch args[0] {
	case "pending":
		rs, err := c.Pending()
		check(err)
		if len(rs) == 0 {
			fmt.Println("nothing is waiting for you")
		}
		for _, r := range rs {
			fmt.Printf("%s  %s\n        asked %s ago via %s\n%s        answers: %s\n", r.ID, indent(r.Text, 8), ago(r.Asked), r.Via, onBehalf(r), list(r.Options))
		}
	case "answer":
		need(2)
		r, err := c.Answer(args[1], args[2])
		check(err)
		if r.Kept {
			fmt.Printf("%s: %s, and %s will not be asked this again\n", r.ID, r.Option, r.Asker)
		} else {
			fmt.Printf("%s: %s, this once\n", r.ID, r.Option)
		}
	case "answered":
		rs, err := c.Answered()
		check(err)
		if len(rs) == 0 {
			fmt.Println("you have answered nothing yet")
		}
		for _, r := range rs {
			kept := "this once"
			if r.Kept {
				kept = "kept"
			}
			fmt.Printf("%s  %-7s %-9s %s  %s\n", r.ID, r.Option, kept, r.Answered.Local().Format("2006-01-02 15:04"), indent(r.Text, 8))
		}
	case "forget":
		need(1)
		check(c.Forget(args[1]))
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func indent(text string, n int) string {
	return strings.ReplaceAll(text, "\n", "\n"+strings.Repeat(" ", n))
}

func list(rs []string) string { return strings.Join(rs, ", ") }

// What the connected program says it bound about the one it asks for. Its word,
// not this service's: the person is told so.
func onBehalf(r asks.Record) string {
	if r.For == nil {
		return ""
	}
	return fmt.Sprintf("        for %s, on the word of the asker\n", r.For)
}

func ago(t time.Time) string { return time.Since(t).Round(time.Second).String() }

func check(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
