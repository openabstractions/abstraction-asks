package asks

import "testing"

// With the reentry check on, a Book lock taken again by the goroutine that
// holds it panics instead of hanging, through Lock and through TryLock;
// another goroutine still waits as usual.
func TestBookLockReentryPanics(t *testing.T) {
	defer CheckBookLockReentry()()
	for name, again := range map[string]func(*bookMutex){
		"Lock":    func(m *bookMutex) { m.Lock() },
		"TryLock": func(m *bookMutex) { m.TryLock() },
	} {
		t.Run(name, func(t *testing.T) {
			var m bookMutex
			m.Lock()
			defer m.Unlock()
			defer func() {
				if got := recover(); got != ErrBookLockReentered {
					t.Fatalf("reentry: %v", got)
				}
			}()
			again(&m)
		})
	}
	var m bookMutex
	m.Lock()
	acquired := make(chan bool)
	go func() { acquired <- m.TryLock() }()
	if <-acquired {
		t.Fatal("another goroutine took a held lock")
	}
	m.Unlock()
	go func() { m.Lock(); acquired <- true; m.Unlock() }()
	if !<-acquired {
		t.Fatal("another goroutine could not take a free lock")
	}
}
