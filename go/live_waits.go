package asks

import "sync"

// Live waits are advisory. At capacity the operation can still wait, while
// the presentation owner has no priority observation for the extra waiter.
const maxLiveQuestionWaits = 1024

// ObserveQuestionWait subscribes to settlement and records a live caller
// waiting on an exact pending question. The caller must release when it stops
// waiting, including on cancellation. Background settlement followers use Decided.
func (b *Book) ObserveQuestionWait(id string) (<-chan struct{}, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.liveWaitCount >= maxLiveQuestionWaits {
		return b.decidedLocked(id), func() {}
	}
	record, found := b.get(id)
	if !found || !record.Pending() {
		return b.decidedLocked(id), func() {}
	}
	done := b.decided[id]
	if done == nil {
		done = make(chan struct{})
		b.decided[id] = done
	}
	if b.liveWaits == nil {
		b.liveWaits = make(map[string]int)
	}
	b.liveWaits[id]++
	b.liveWaitCount++
	var once sync.Once
	return done, func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			// Settlement already removed this generation's observations.
			if b.decided[id] != done {
				return
			}
			b.liveWaitCount--
			b.liveWaits[id]--
			if b.liveWaits[id] == 0 {
				delete(b.liveWaits, id)
			}
		})
	}
}

// LiveQuestionWaiters reports process-local observations for one occurrence.
// This is a scheduling fact. It confers no answer or presentation authority.
func (b *Book) LiveQuestionWaiters(id string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.liveWaits[id]
}
