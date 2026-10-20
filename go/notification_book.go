package asks

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

// Notification partition bounds (ASK-N1, ASK-N6). They are provider choices
// for the first cut; the contract's bounds table names them.
const (
	MaxNotificationAdmissions = 1024
	MaxNotificationBytes      = 1 << 20
	// MaxLiveNotifications is how many posted notifications one program keeps;
	// a further untagged post withdraws its oldest.
	MaxLiveNotifications = 4
	// MaxNotificationsPerHour bounds admissions per program in any hour.
	MaxNotificationsPerHour = 20
	// MaxNotifiers bounds the per-program audit rows.
	MaxNotifiers                = 256
	notificationAuditDays       = 7
	dayMilliseconds       int64 = 24 * 60 * 60 * 1000
	hourMilliseconds      int64 = 60 * 60 * 1000
)

var (
	// ErrNotificationSuppressed is the authorization result for a program
	// under a live explicit block on abstraction.asks/notification.post.
	ErrNotificationSuppressed = errors.New("asks: notifications from this program are turned off")
	errNotificationCapacity   = errors.New("asks: notification capacity reached")
)

// notificationAdmission is one caller-scoped notification. An acknowledged
// occurrence keeps only its reference, fingerprints and ID until its replay
// deadline.
type notificationAdmission struct {
	Scope       string                   `json:"scope"`
	Reference   wire.RequestRef          `json:"reference"`
	Fingerprint string                   `json:"fingerprint"`
	ID          string                   `json:"id"`
	Record      *wire.NotificationRecord `json:"record,omitempty"`
	ResultHash  string                   `json:"result_hash,omitempty"`
	Via         *listen.Seen             `json:"via,omitempty"`
}

// notifier is one program's audit: admission times within the last hour and
// admissions per UTC day for the last seven days.
type notifier struct {
	Program string        `json:"program"`
	Recent  []int64       `json:"recent,omitempty"`
	Days    []notifierDay `json:"days"`
	LastMs  int64         `json:"last_ms"`
}
type notifierDay struct {
	DayMs int64 `json:"day_ms"`
	Posts int64 `json:"posts"`
}

func notificationLine(s string, limit int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= limit && strings.TrimSpace(s) != "" && strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
	}) < 0
}
func notificationBody(s string) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= 240 && strings.TrimSpace(s) != "" && strings.IndexFunc(s, func(r rune) bool {
		return r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r))
	}) < 0
}

// ValidOperationID reports the job operation id grammar a notification may
// reference: 1..128 lowercase ASCII letters, digits and '-' (JOB-A13).
func ValidOperationID(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, c := range []byte(s) {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// ValidNotificationRequest checks immutable shape independently of the
// service clock (ASK-N1).
func ValidNotificationRequest(q wire.NotificationRequest) bool {
	if !ValidChoiceReference(q.RequestRef) || !notificationLine(q.Title, 80) || q.Body != "" && !notificationBody(q.Body) || len(q.Actions) > 2 || q.ExpiresMs <= 0 || q.ExpiresMs > q.RequestRef.ReplayUntilMs {
		return false
	}
	if q.OperationID != "" && !ValidOperationID(q.OperationID) || q.Tag != "" && !ValidChoiceOptionID(q.Tag) {
		return false
	}
	switch q.Delivery {
	case wire.NotificationDeliveryNow:
	case wire.NotificationDeliveryOperationEnd:
		if q.OperationID == "" {
			return false
		}
	default:
		return false
	}
	ids := map[string]bool{}
	for _, a := range q.Actions {
		if !ValidChoiceOptionID(a.ID) || !notificationLine(a.Title, 40) || ids[a.ID] {
			return false
		}
		switch a.Activation {
		case wire.NotificationActivationApp:
		case wire.NotificationActivationOpenActivity:
			if q.OperationID == "" {
				return false
			}
		default:
			return false
		}
		ids[a.ID] = true
	}
	return true
}

func cloneNotification(r wire.NotificationRecord) wire.NotificationRecord {
	r.Request.Actions = append([]wire.NotificationAction{}, r.Request.Actions...)
	if r.Result != nil {
		result := *r.Result
		r.Result = &result
	}
	return r
}

func notificationOffers(q wire.NotificationRequest, id string) bool {
	return slices.ContainsFunc(q.Actions, func(a wire.NotificationAction) bool { return a.ID == id })
}

func notificationLive(s wire.NotificationState) bool {
	return s == wire.NotificationStateScheduled || s == wire.NotificationStatePosted
}

// ValidNotificationResult checks one recorded terminal outcome.
func ValidNotificationResult(r wire.NotificationResult) bool {
	if !ValidRequestKey(r.ID) || r.DecidedMs <= 0 || !validAnswerer(r.DecidedBy) {
		return false
	}
	switch r.State {
	case wire.NotificationStateActed:
		return ValidChoiceOptionID(r.Action)
	case wire.NotificationStateDismissed, wire.NotificationStateWithdrawn:
		return r.Action == ""
	}
	return false
}

// ValidNotificationRecord checks a stored or served record's consistency.
func ValidNotificationRecord(r wire.NotificationRecord) bool {
	if !ValidRequestKey(r.ID) || !validAnswerer(r.Program) || r.Asker != AskerDisplay(r.Program) || !ValidNotificationRequest(r.Request) || r.AdmittedMs <= 0 || r.AdmittedMs >= r.Request.ExpiresMs || r.Request.RequestRef.ReplayUntilMs-r.AdmittedMs > MaxChoiceReplayMilliseconds {
		return false
	}
	p := Record{Publisher: r.Publisher, PublisherTrusted: r.PublisherTrusted, ProgramDescription: r.ProgramDescription, ProgramOriginalName: r.ProgramOriginalName}
	if !validRecordedPublisher(&p) || r.PostedMs < 0 || r.PostedMs != 0 && r.PostedMs < r.AdmittedMs {
		return false
	}
	switch r.State {
	case wire.NotificationStateScheduled:
		return r.PostedMs == 0 && r.Result == nil
	case wire.NotificationStatePosted:
		return r.PostedMs > 0 && r.Result == nil
	case wire.NotificationStateExpired:
		return r.Result == nil
	case wire.NotificationStateActed, wire.NotificationStateDismissed, wire.NotificationStateWithdrawn:
		if r.Result == nil || !ValidNotificationResult(*r.Result) || r.Result.ID != r.ID || r.Result.State != r.State || r.Result.DecidedMs < r.AdmittedMs {
			return false
		}
		if r.State == wire.NotificationStateActed {
			return r.PostedMs > 0 && notificationOffers(r.Request, r.Result.Action)
		}
		return true
	}
	return false
}

// notificationPartitionBytes is the encoded size the notification bound counts.
func notificationPartitionBytes(f book) (int, error) {
	raw, err := json.Marshal(struct {
		Notifications []notificationAdmission
		Notifiers     []notifier
	}{f.Notifications, f.Notifiers})
	return len(raw), err
}

func validateNotificationPartition(f book) error {
	if f.Profile != ApplicationProfile && f.Profile != notificationApplicationProfile && (len(f.Notifications) != 0 || len(f.Notifiers) != 0) {
		return errors.New("asks: notifications need application profile version 8 or later")
	}
	if n, e := notificationPartitionBytes(f); e != nil || n > MaxNotificationBytes || len(f.Notifications) > MaxNotificationAdmissions || len(f.Notifiers) > MaxNotifiers {
		return errNotificationCapacity
	}
	if len(f.Notifications) > 0 && f.ChoiceTimeMs == 0 {
		return errors.New("asks: invalid notification time")
	}
	refs, ids := map[string]bool{}, map[string]bool{}
	for _, a := range f.Notifications {
		key := a.Scope + "\x00" + choiceFingerprint(a.Reference)
		if !ValidRequestKey(a.Scope) || !ValidChoiceReference(a.Reference) || !validChoiceHash(a.Fingerprint) || !ValidRequestKey(a.ID) || refs[key] || ids[a.ID] {
			return errors.New("asks: invalid notification admission")
		}
		if a.Record == nil {
			if !validChoiceHash(a.ResultHash) || a.Via != nil {
				return errors.New("asks: invalid notification acknowledgement")
			}
		} else if a.ResultHash != "" || a.Record.Presentation != "" || !ValidNotificationRecord(*a.Record) || a.Record.AdmittedMs > f.ChoiceTimeMs || a.Record.Result != nil && a.Record.Result.DecidedMs > f.ChoiceTimeMs || a.Record.ID != a.ID || a.Record.Request.RequestRef != a.Reference || choiceFingerprint(a.Record.Request) != a.Fingerprint {
			return errors.New("asks: invalid notification record")
		}
		refs[key], ids[a.ID] = true, true
	}
	programs := map[string]bool{}
	for _, n := range f.Notifiers {
		if !validAnswerer(n.Program) || programs[n.Program] || n.LastMs <= 0 || n.LastMs > f.ChoiceTimeMs || len(n.Recent) > MaxNotificationsPerHour || len(n.Days) == 0 || len(n.Days) > notificationAuditDays {
			return errors.New("asks: invalid notifier")
		}
		if !slices.IsSorted(n.Recent) || len(n.Recent) > 0 && (n.Recent[0] <= 0 || n.Recent[len(n.Recent)-1] > n.LastMs) {
			return errors.New("asks: invalid notifier times")
		}
		for i, d := range n.Days {
			if d.DayMs < 0 || d.DayMs%dayMilliseconds != 0 || d.Posts <= 0 || i > 0 && d.DayMs <= n.Days[i-1].DayMs {
				return errors.New("asks: invalid notifier days")
			}
		}
		programs[n.Program] = true
	}
	return nil
}

// collectNotifications removes admissions past their replay deadline and
// marks scheduled or posted ones past their expiry expired (ASK-N3). It
// returns the IDs whose waiters settle.
func (f *book) collectNotifications(now int64) []string {
	var settled []string
	kept := f.Notifications[:0]
	for _, a := range f.Notifications {
		if now >= a.Reference.ReplayUntilMs {
			settled = append(settled, a.ID)
			continue
		}
		if a.Record != nil && notificationLive(a.Record.State) && now >= a.Record.Request.ExpiresMs {
			a.Record.State = wire.NotificationStateExpired
			settled = append(settled, a.ID)
		}
		kept = append(kept, a)
	}
	f.Notifications = kept
	return settled
}

func notificationObservation(a notificationAdmission) wire.NotificationObservation {
	if a.Record == nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeGone}
	}
	r := cloneNotification(*a.Record)
	outcome, _ := wire.ParseNotificationOutcome(r.State.String())
	result := wire.NotificationObservation{Outcome: outcome}
	if r.State != wire.NotificationStateExpired {
		result.ID, result.Result = r.ID, r.Result
	}
	return result
}

func (f *book) notificationIndex(scope string, ref wire.RequestRef) int {
	return slices.IndexFunc(f.Notifications, func(a notificationAdmission) bool { return a.Scope == scope && a.Reference == ref })
}
func (f *book) notificationID(id string) int {
	return slices.IndexFunc(f.Notifications, func(a notificationAdmission) bool { return a.ID == id })
}

// notice counts one admission for program at now (ASK-N6). It reports false
// when the hourly bound is reached; nothing is counted then.
func (f *book) notice(program string, now int64) bool {
	i := slices.IndexFunc(f.Notifiers, func(n notifier) bool { return n.Program == program })
	if i < 0 {
		if len(f.Notifiers) >= MaxNotifiers {
			oldest := 0
			for j, n := range f.Notifiers {
				if n.LastMs < f.Notifiers[oldest].LastMs {
					oldest = j
				}
			}
			f.Notifiers = slices.Delete(f.Notifiers, oldest, oldest+1)
		}
		f.Notifiers = append(f.Notifiers, notifier{Program: program})
		i = len(f.Notifiers) - 1
	}
	n := &f.Notifiers[i]
	n.Recent = slices.DeleteFunc(n.Recent, func(t int64) bool { return t <= now-hourMilliseconds })
	if len(n.Recent) >= MaxNotificationsPerHour {
		if len(n.Days) == 0 {
			f.Notifiers = slices.Delete(f.Notifiers, i, i+1)
		}
		return false
	}
	n.Recent = append(n.Recent, now)
	day := now - now%dayMilliseconds
	n.Days = slices.DeleteFunc(n.Days, func(d notifierDay) bool { return d.DayMs <= day-notificationAuditDays*dayMilliseconds })
	if len(n.Days) > 0 && n.Days[len(n.Days)-1].DayMs == day {
		n.Days[len(n.Days)-1].Posts++
	} else {
		n.Days = append(n.Days, notifierDay{DayMs: day, Posts: 1})
	}
	n.LastMs = now
	return true
}

// withdrawRecord records a withdrawal of one live record.
func withdrawRecord(r *wire.NotificationRecord, actor string, now int64) {
	r.State = wire.NotificationStateWithdrawn
	r.Result = &wire.NotificationResult{ID: r.ID, State: wire.NotificationStateWithdrawn, DecidedMs: now, DecidedBy: actor}
}

// PostNotificationFrom admits one caller-scoped notification (ASK-N1). The
// service supplies scope, the bound program and publisher evidence; authorize
// runs inside the same edit and returns ErrNotificationSuppressed for a live
// block. The caller has already confirmed that a referenced operation is its
// own. Settled lists IDs a presenter may need to remove.
func (b *Book) PostNotificationFrom(scope, program string, q wire.NotificationRequest, via listen.Seen, publisher Publisher, authorize func() error) (wire.NotificationObservation, []string, error) {
	if !b.application || !ValidRequestKey(scope) || !validAnswerer(program) || !ValidNotificationRequest(q) {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeInvalid}, nil, nil
	}
	// The stored request is the fingerprinted one: no actions encode as an
	// empty list either way, so a reload compares the same bytes.
	q.Actions = append([]wire.NotificationAction{}, q.Actions...)
	b.mu.Lock()
	defer b.mu.Unlock()
	result := wire.NotificationObservation{Outcome: wire.NotificationOutcomeUnknown}
	var replaced []string
	err := b.changeApplication(func(f *book) error {
		if authorize != nil {
			if e := authorize(); e != nil {
				return e
			}
		}
		now := f.choiceTime(b.clock().UnixMilli())
		if now <= 0 {
			return errors.New("asks: ordinary service time unavailable")
		}
		if now >= q.RequestRef.ReplayUntilMs {
			f.noteChoiceTime(now)
			result.Outcome = wire.NotificationOutcomeExpired
			return nil
		}
		if i := f.notificationIndex(scope, q.RequestRef); i >= 0 {
			if f.Notifications[i].Fingerprint != choiceFingerprint(q) {
				result.Outcome = wire.NotificationOutcomeConflict
			} else {
				result = notificationObservation(f.Notifications[i])
			}
			return nil
		}
		if q.ExpiresMs <= now {
			f.noteChoiceTime(now)
			result.Outcome = wire.NotificationOutcomeExpired
			return nil
		}
		if q.RequestRef.ReplayUntilMs-now > MaxChoiceReplayMilliseconds {
			result.Outcome = wire.NotificationOutcomeInvalid
			return nil
		}
		if len(f.Notifications) >= MaxNotificationAdmissions || !f.notice(program, now) {
			return errNotificationCapacity
		}
		live := []int{}
		for i := range f.Notifications {
			a := &f.Notifications[i]
			if a.Scope != scope || a.Record == nil || !notificationLive(a.Record.State) {
				continue
			}
			if q.Tag != "" && a.Record.Request.Tag == q.Tag {
				withdrawRecord(a.Record, program, now)
				replaced = append(replaced, a.ID)
				continue
			}
			if a.Record.State == wire.NotificationStatePosted {
				live = append(live, i)
			}
		}
		if q.Delivery == wire.NotificationDeliveryNow && len(live) >= MaxLiveNotifications {
			// Admission order is posting order for posted records of one scope.
			oldest := live[0]
			for _, i := range live[1:] {
				if f.Notifications[i].Record.PostedMs < f.Notifications[oldest].Record.PostedMs {
					oldest = i
				}
			}
			withdrawRecord(f.Notifications[oldest].Record, program, now)
			replaced = append(replaced, f.Notifications[oldest].ID)
		}
		publisher = publisher.recorded()
		record := wire.NotificationRecord{ID: random(16), Asker: AskerDisplay(program), Program: program, Request: q, State: wire.NotificationStateScheduled, AdmittedMs: now,
			Publisher: publisher.Subject, PublisherTrusted: publisher.Trusted, ProgramDescription: publisher.Description, ProgramOriginalName: publisher.OriginalName}
		if q.Delivery == wire.NotificationDeliveryNow {
			record.State, record.PostedMs = wire.NotificationStatePosted, now
		}
		record = cloneNotification(record)
		a := notificationAdmission{Scope: scope, Reference: q.RequestRef, Fingerprint: choiceFingerprint(q), ID: record.ID, Record: &record, Via: &via}
		f.Notifications = append(f.Notifications, a)
		f.noteChoiceTime(now)
		result = notificationObservation(a)
		return nil
	})
	if errors.Is(err, errNotificationCapacity) {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeExhausted}, nil, nil
	}
	if err != nil {
		return wire.NotificationObservation{}, nil, err
	}
	for _, id := range replaced {
		b.settle(id)
	}
	return result, replaced, nil
}

// ObserveNotification reads this caller's own full reference.
func (b *Book) ObserveNotification(scope string, ref wire.RequestRef) (wire.NotificationObservation, error) {
	if !b.application || !ValidRequestKey(scope) || !ValidChoiceReference(ref) {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeInvalid}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := b.readChoices()
	if err != nil {
		return wire.NotificationObservation{}, err
	}
	now := f.choiceTime(b.clock().UnixMilli())
	if now >= ref.ReplayUntilMs {
		if now > f.ChoiceTimeMs {
			if err := b.changeApplication(func(f *book) error { f.noteChoiceTime(now); return nil }); err != nil {
				return wire.NotificationObservation{}, err
			}
		}
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeExpired}, nil
	}
	if i := f.notificationIndex(scope, ref); i >= 0 {
		return notificationObservation(f.Notifications[i]), nil
	}
	return wire.NotificationObservation{Outcome: wire.NotificationOutcomeUnknown}, nil
}

// NotificationKind names the terminal outcome a decision records.
type NotificationKind int

const (
	NotificationAct NotificationKind = iota + 1
	NotificationDismiss
	NotificationWithdraw
)

func (k NotificationKind) state() wire.NotificationState {
	switch k {
	case NotificationAct:
		return wire.NotificationStateActed
	case NotificationDismiss:
		return wire.NotificationStateDismissed
	}
	return wire.NotificationStateWithdrawn
}

// DecideNotification records an operator's act or dismissal on one occurrence
// (ASK-N4). authorize runs inside the edit with the poster's bound program; an
// Act refuses with ErrNotificationSuppressed when that program is blocked.
func (b *Book) DecideNotification(id string, kind NotificationKind, action, actor string, authorize func(program string) error) (wire.NotificationDecision, error) {
	if !b.application || !ValidRequestKey(id) || !validAnswerer(actor) || kind == NotificationAct && !ValidChoiceOptionID(action) || kind != NotificationAct && action != "" || kind == NotificationWithdraw {
		return wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeInvalid}, nil
	}
	return b.decideNotification(func(f *book) int { return f.notificationID(id) }, nil, kind, action, actor, authorize)
}

// WithdrawOwnNotification withdraws the proven poster's own full reference.
func (b *Book) WithdrawOwnNotification(scope string, ref wire.RequestRef, actor string, authorize func(program string) error) (wire.NotificationDecision, error) {
	if !b.application || !ValidRequestKey(scope) || !ValidChoiceReference(ref) || !validAnswerer(actor) {
		return wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeInvalid}, nil
	}
	return b.decideNotification(func(f *book) int { return f.notificationIndex(scope, ref) }, &ref, NotificationWithdraw, "", actor, authorize)
}

func (b *Book) decideNotification(find func(*book) int, ref *wire.RequestRef, kind NotificationKind, action, actor string, authorize func(string) error) (wire.NotificationDecision, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeUnknown}
	err := b.changeApplication(func(f *book) error {
		now := f.choiceTime(b.clock().UnixMilli())
		if now <= 0 {
			return errors.New("asks: ordinary service time unavailable")
		}
		if ref != nil && now >= ref.ReplayUntilMs {
			f.noteChoiceTime(now)
			result.Outcome = wire.NotificationDecisionOutcomeExpired
			return nil
		}
		i := find(f)
		if i < 0 {
			return nil
		}
		a := &f.Notifications[i]
		if now >= a.Reference.ReplayUntilMs {
			f.noteChoiceTime(now)
			result.Outcome = wire.NotificationDecisionOutcomeExpired
			return nil
		}
		if a.Record == nil {
			result.Outcome = wire.NotificationDecisionOutcomeGone
			return nil
		}
		r := a.Record
		if authorize != nil {
			if e := authorize(r.Program); e != nil {
				return e
			}
		}
		if r.State == wire.NotificationStateExpired || notificationLive(r.State) && now >= r.Request.ExpiresMs {
			r.State = wire.NotificationStateExpired
			f.noteChoiceTime(now)
			result.Outcome = wire.NotificationDecisionOutcomeExpired
			return nil
		}
		state := kind.state()
		if !notificationLive(r.State) {
			if r.State != state || r.Result.Action != action {
				result.Outcome = wire.NotificationDecisionOutcomeConflict
				if kind == NotificationWithdraw {
					// The poster learns the standing outcome unchanged.
					copy := cloneNotification(*r)
					parsed, _ := wire.ParseNotificationDecisionOutcome(r.State.String())
					result = wire.NotificationDecision{Outcome: parsed, Record: &copy}
				}
				return nil
			}
		} else {
			if kind == NotificationAct && (r.State != wire.NotificationStatePosted || !notificationOffers(r.Request, action)) {
				result.Outcome = wire.NotificationDecisionOutcomeInvalid
				return nil
			}
			r.State = state
			r.Result = &wire.NotificationResult{ID: r.ID, State: state, Action: action, DecidedMs: now, DecidedBy: actor}
			f.noteChoiceTime(now)
		}
		parsed, _ := wire.ParseNotificationDecisionOutcome(state.String())
		copy := cloneNotification(*r)
		result = wire.NotificationDecision{Outcome: parsed, Record: &copy}
		return nil
	})
	if err == nil && result.Record != nil {
		b.settle(result.Record.ID)
	}
	return result, err
}

// WithdrawProgramNotifications withdraws every live notification of one bound
// program, for an operator after the person turned its notifications off.
func (b *Book) WithdrawProgramNotifications(program, actor string, authorize func() error) (int64, []string, error) {
	if !b.application || !validAnswerer(program) || !validAnswerer(actor) {
		return 0, nil, ErrNoRecord
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var changed []string
	err := b.changeApplication(func(f *book) error {
		changed = changed[:0]
		if authorize != nil {
			if e := authorize(); e != nil {
				return e
			}
		}
		now := f.choiceTime(b.clock().UnixMilli())
		if now <= 0 {
			return errors.New("asks: ordinary service time unavailable")
		}
		for i := range f.Notifications {
			r := f.Notifications[i].Record
			if r != nil && r.Program == program && notificationLive(r.State) && now < r.Request.ExpiresMs {
				withdrawRecord(r, actor, now)
				changed = append(changed, r.ID)
			}
		}
		if len(changed) > 0 {
			f.noteChoiceTime(now)
		}
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	for _, id := range changed {
		b.settle(id)
	}
	return int64(len(changed)), changed, nil
}

// AcknowledgeNotification relinquishes service replay of the exact recorded
// result (ASK-N5).
func (b *Book) AcknowledgeNotification(scope string, ref wire.RequestRef, result wire.NotificationResult, authorize func() error) (wire.ChoiceAcknowledgement, error) {
	if !b.application || !ValidRequestKey(scope) || !ValidChoiceReference(ref) || !ValidNotificationResult(result) {
		return wire.ChoiceAcknowledgement{Outcome: wire.ChoiceAckOutcomeInvalid}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	ack := wire.ChoiceAcknowledgement{Outcome: wire.ChoiceAckOutcomeUnknown}
	err := b.changeApplication(func(f *book) error {
		if authorize != nil {
			if e := authorize(); e != nil {
				return e
			}
		}
		now := f.choiceTime(b.clock().UnixMilli())
		if now <= 0 {
			return errors.New("asks: ordinary service time unavailable")
		}
		if now >= ref.ReplayUntilMs {
			f.noteChoiceTime(now)
			ack.Outcome = wire.ChoiceAckOutcomeExpired
			return nil
		}
		i := f.notificationIndex(scope, ref)
		if i < 0 {
			return nil
		}
		a := &f.Notifications[i]
		hash := choiceFingerprint(result)
		if a.ID != result.ID || a.Record == nil && a.ResultHash != hash || a.Record != nil && (a.Record.Result == nil || *a.Record.Result != result) {
			ack.Outcome = wire.ChoiceAckOutcomeConflict
			return nil
		}
		if a.Record != nil {
			a.Record, a.ResultHash, a.Via = nil, hash, nil
			f.noteChoiceTime(now)
		}
		ack.Outcome = wire.ChoiceAckOutcomeAcknowledged
		return nil
	})
	return ack, err
}

// NotificationHistory lists retained records in admission order with a
// revision for operator cursors.
func (b *Book) NotificationHistory() ([]wire.NotificationRecord, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return nil, "", errors.New("asks: application Book required")
	}
	f, err := b.readChoices()
	if err != nil {
		return nil, "", err
	}
	rows := []wire.NotificationRecord{}
	for _, a := range f.Notifications {
		if a.Record != nil {
			rows = append(rows, cloneNotification(*a.Record))
		}
	}
	return rows, choiceFingerprint(f.Notifications), nil
}

// NotificationRecord reads one retained occurrence by ID.
func (b *Book) NotificationRecord(id string) (wire.NotificationRecord, bool, error) {
	rows, _, err := b.NotificationHistory()
	for _, r := range rows {
		if r.ID == id {
			return r, true, err
		}
	}
	return wire.NotificationRecord{}, false, err
}

// NotifierCounts lists per-program audit rows for the last seven service days.
func (b *Book) NotifierCounts() ([]wire.NotifierCount, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return nil, "", errors.New("asks: application Book required")
	}
	f, err := b.readChoices()
	if err != nil {
		return nil, "", err
	}
	now := f.choiceTime(b.clock().UnixMilli())
	from := now - now%dayMilliseconds - (notificationAuditDays-1)*dayMilliseconds
	rows := []wire.NotifierCount{}
	for _, n := range f.Notifiers {
		row := wire.NotifierCount{Program: n.Program, Asker: AskerDisplay(n.Program), Days: []wire.NotifierDay{}, LastPostedMs: n.LastMs}
		for _, d := range n.Days {
			if d.DayMs >= from {
				row.Days = append(row.Days, wire.NotifierDay{DayMs: d.DayMs, Posts: d.Posts})
			}
		}
		rows = append(rows, row)
	}
	return rows, choiceFingerprint(f.Notifiers), nil
}

// ScheduledNotification is one held notification waiting for its operation.
type ScheduledNotification struct {
	ID          string
	Scope       string
	Program     string
	OperationID string
}

// ScheduledNotifications lists held notifications for the trusted runtime
// that observes their operations (ASK-N1 operation_end).
func (b *Book) ScheduledNotifications() ([]ScheduledNotification, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return nil, errors.New("asks: application Book required")
	}
	f, err := b.readChoices()
	if err != nil {
		return nil, err
	}
	var out []ScheduledNotification
	for _, a := range f.Notifications {
		if a.Record != nil && a.Record.State == wire.NotificationStateScheduled {
			out = append(out, ScheduledNotification{ID: a.ID, Scope: a.Scope, Program: a.Record.Program, OperationID: a.Record.Request.OperationID})
		}
	}
	return out, nil
}

// PostScheduledNotification posts one held notification after the trusted
// runtime observed its operation end. It reports whether the record changed;
// an expired, withdrawn or dismissed record stays as it is.
func (b *Book) PostScheduledNotification(id string) (bool, error) {
	if !b.application || !ValidRequestKey(id) {
		return false, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	posted := false
	err := b.changeApplication(func(f *book) error {
		now := f.choiceTime(b.clock().UnixMilli())
		if now <= 0 {
			return errors.New("asks: ordinary service time unavailable")
		}
		i := f.notificationID(id)
		if i < 0 || f.Notifications[i].Record == nil || f.Notifications[i].Record.State != wire.NotificationStateScheduled || now >= f.Notifications[i].Record.Request.ExpiresMs {
			return nil
		}
		r := f.Notifications[i].Record
		live := []int{}
		for j := range f.Notifications {
			o := f.Notifications[j].Record
			if j != i && f.Notifications[j].Scope == f.Notifications[i].Scope && o != nil && o.State == wire.NotificationStatePosted {
				live = append(live, j)
			}
		}
		if len(live) >= MaxLiveNotifications {
			oldest := live[0]
			for _, j := range live[1:] {
				if f.Notifications[j].Record.PostedMs < f.Notifications[oldest].Record.PostedMs {
					oldest = j
				}
			}
			withdrawRecord(f.Notifications[oldest].Record, r.Program, now)
		}
		r.State, r.PostedMs = wire.NotificationStatePosted, now
		f.noteChoiceTime(now)
		posted = true
		return nil
	})
	return posted, err
}

func (b *Book) notificationPending(id string) bool {
	if !b.application {
		return false
	}
	f, err := b.readApplication()
	i := f.notificationID(id)
	return err == nil && i >= 0 && f.Notifications[i].Record != nil && notificationLive(f.Notifications[i].Record.State)
}
