package application

import (
	"context"
	"time"

	watch "github.com/openabstractions/abstraction-watch/go"
)

// waited is how an observation wait ended for the receivers' outcome words.
type waited int

const (
	waitRead waited = iota
	waitForbidden
	waitCancelled
	waitUnavailable
)

// recheckRefusal marks the caller's binding failing its recheck.
type recheckRefusal struct{ error }

// observe waits up to waitMS on the book's shared waiter (watch.Gate) while
// read reports nothing settled. recheck confirms the caller's binding after
// every read and before anything is returned (WATCH-W4).
func (h *Host) observe(ctx context.Context, waitMS int64, read func() bool, recheck func() error, exactPending ...func() string) (waited, error) {
	return h.observeFor(ctx, time.Duration(waitMS)*time.Millisecond, read, recheck, exactPending...)
}

// observeFor is observe with a wait already cut to the record's own expiry.
func (h *Host) observeFor(ctx context.Context, wait time.Duration, read func() bool, recheck func() error, exactPending ...func() string) (waited, error) {
	var observed string
	var stop func()
	defer func() {
		if stop != nil {
			stop()
		}
	}()
	var answered bool
	deadline := time.Now().Add(wait)
	outcome, err := h.book.Gate().Wait(ctx, wait, watch.Probe{
		Read: func() (bool, error) {
			answered = read()
			return answered, nil
		},
		Recheck: func() error {
			if err := recheck(); err != nil {
				return recheckRefusal{err}
			}
			if wait > 0 && len(exactPending) == 1 {
				id := ""
				if !answered && ctx.Err() == nil && time.Now().Before(deadline) {
					id = exactPending[0]()
				}
				if id != observed {
					if stop != nil {
						stop()
						stop = nil
					}
					if id != "" {
						_, stop = h.book.ObserveQuestionWait(id)
					}
					observed = id
				}
			}
			return nil
		},
	})
	if _, refused := err.(recheckRefusal); refused {
		return waitForbidden, nil
	}
	if err != nil {
		return waitCancelled, err
	}
	if outcome == watch.Busy || outcome == watch.Failed {
		return waitUnavailable, nil
	}
	return waitRead, nil
}
