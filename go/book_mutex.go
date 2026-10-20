package asks

import (
	"bytes"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
)

// bookMutex is the Book's lock. It is a sync.Mutex; with the reentry check on,
// a Lock by the goroutine that already holds it panics with
// ErrBookLockReentered instead of hanging every question and popup
// (2026-11-09: two such hangs in one day, both a recheck callback that read
// the Book while an admission held its lock).
type bookMutex struct {
	mu     sync.Mutex
	holder atomic.Int64
}

// ErrBookLockReentered is the panic value of a Book lock taken again by the
// goroutine that holds it, while the reentry check is on.
const ErrBookLockReentered = "asks: the Book lock was taken again by the goroutine that holds it; a callback run under the lock read the Book"

var bookReentryCheck atomic.Bool

// CheckBookLockReentry turns the reentry check on for every Book until the
// returned restore runs. Tests turn it on; it costs one stack read per lock.
func CheckBookLockReentry() (restore func()) {
	previous := bookReentryCheck.Swap(true)
	return func() { bookReentryCheck.Store(previous) }
}

func (m *bookMutex) Lock() {
	if !bookReentryCheck.Load() {
		m.mu.Lock()
		return
	}
	id := goroutineID()
	if id != 0 && m.holder.Load() == id {
		panic(ErrBookLockReentered)
	}
	m.mu.Lock()
	m.holder.Store(id)
}

// TryLock also panics on a reentry: a caller that spins on TryLock while its
// own goroutine holds the lock never acquires it.
func (m *bookMutex) TryLock() bool {
	if !bookReentryCheck.Load() {
		return m.mu.TryLock()
	}
	id := goroutineID()
	if id != 0 && m.holder.Load() == id {
		panic(ErrBookLockReentered)
	}
	if !m.mu.TryLock() {
		return false
	}
	m.holder.Store(id)
	return true
}

func (m *bookMutex) Unlock() {
	m.holder.Store(0)
	m.mu.Unlock()
}

// goroutineID is the running goroutine's number from its stack header
// ("goroutine N [...]"), or zero when it cannot be read.
func goroutineID() int64 {
	var buf [64]byte
	header := buf[:runtime.Stack(buf[:], false)]
	header = bytes.TrimPrefix(header, []byte("goroutine "))
	if i := bytes.IndexByte(header, ' '); i > 0 {
		if id, err := strconv.ParseInt(string(header[:i]), 10, 64); err == nil {
			return id
		}
	}
	return 0
}
