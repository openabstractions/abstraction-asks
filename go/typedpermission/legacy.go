package typedpermission

import (
	"context"
	"errors"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/catalogue"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxChoiceReplayMilliseconds int64 = 7 * 24 * 60 * 60 * 1000
const MaxAnswerTitleBytes = 8192
const PermissionContentWrite = "abstraction.storage/content.write"
const PermissionContentRead = "abstraction.storage/content.read"

func ValidRequestKey(s string) bool { return operatorWord(s) }
func validAnswerer(program string, v Validators) bool {
	return v.NativeProgram != nil && len(program) <= 4096 && utf8.ValidString(program) && v.NativeProgram(program)
}
func AskerDisplay(program string) string { return filepath.Base(program) }

type recorded struct {
	Publisher                               string
	PublisherTrusted                        bool
	ProgramDescription, ProgramOriginalName string
}

func validRecordedPublisher(r *recorded) bool {
	return ValidPublisher(Publisher{Subject: r.Publisher, Trusted: r.PublisherTrusted, Description: r.ProgramDescription, OriginalName: r.ProgramOriginalName})
}

const MaxPublisher = 256

type Publisher struct {
	Subject      string
	Trusted      bool
	Description  string
	OriginalName string
}

func RecordPublisher(p Publisher) Publisher {
	out := Publisher{Subject: publisherSubject(p.Subject), Trusted: p.Trusted}
	if p.Trusted {
		out.Description = publisherSubject(p.Description)
		out.OriginalName = publisherSubject(p.OriginalName)
	}
	return out
}

func publisherSubject(subject string) string {
	if len(subject) > MaxPublisher || !utf8.ValidString(subject) ||
		strings.IndexFunc(subject, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) >= 0 {
		return ""
	}
	return subject
}

func ValidPublisher(p Publisher) bool  { return RecordPublisher(p) == p }
func PublisherSubject(s string) string { return publisherSubject(s) }

func choiceText(s string) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= 80 && strings.TrimSpace(s) != "" && strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
	}) < 0
}

func ChoiceText(s string) bool { return choiceText(s) }

func ValidChoiceOptionID(s string) bool {
	return len(s) > 0 && len(s) <= 64 && utf8.ValidString(s) && strings.TrimSpace(s) != "" && strings.IndexFunc(s, unicode.IsControl) < 0
}

func ValidChoiceReference(ref wire.RequestRef) bool {
	return ValidRequestKey(ref.Key) && ref.ReplayUntilMs > 0
}

func ValidChoiceAnswer(answer wire.ChoiceAnswer, validators Validators) bool {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return false
	}
	return ValidRequestKey(answer.ID) && answer.AnsweredMs > 0 && validAnswerer(answer.AnsweredBy, validators) && (answer.Option == "" || ValidChoiceOptionID(answer.Option))
}

func ValidChoiceRequest(q wire.ChoiceRequest) bool {
	if !ValidChoiceReference(q.RequestRef) || !choiceText(q.Title) || len(q.Options) < 1 || len(q.Options) > 3 || q.PendingUntilMs <= 0 || q.PendingUntilMs > q.RequestRef.ReplayUntilMs {
		return false
	}
	ids := map[string]bool{}
	for _, option := range q.Options {
		if !ValidChoiceOptionID(option.ID) || !choiceText(option.Label) || ids[option.ID] {
			return false
		}
		ids[option.ID] = true
	}
	return true
}

func choiceOption(q wire.ChoiceRequest, id string) bool {
	return slices.ContainsFunc(q.Options, func(o wire.ChoiceOption) bool { return o.ID == id })
}

func ChoiceOffers(q wire.ChoiceRequest, id string) bool { return choiceOption(q, id) }

func ValidAssistantOptions(q wire.ChoiceRequest, shared []string) bool {
	if len(shared) < 1 || len(shared) > 3 {
		return false
	}
	for i, id := range shared {
		if !choiceOption(q, id) || slices.Contains(shared[:i], id) {
			return false
		}
	}
	return true
}

func ValidChoiceRecord(r wire.ChoiceRecord, validators Validators) bool {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return false
	}
	if !ValidRequestKey(r.ID) || r.Asker == "" || !ValidChoiceRequest(r.Request) || r.AskedMs <= 0 || r.AskedMs >= r.Request.PendingUntilMs || r.Request.RequestRef.ReplayUntilMs-r.AskedMs > MaxChoiceReplayMilliseconds {
		return false
	}
	p := recorded{Publisher: r.Publisher, PublisherTrusted: r.PublisherTrusted, ProgramDescription: r.ProgramDescription, ProgramOriginalName: r.ProgramOriginalName}
	if !validRecordedPublisher(&p) {
		return false
	}
	switch r.State {
	case wire.ChoiceStatePending, wire.ChoiceStateExpired:
		return r.Answer == nil
	case wire.ChoiceStateAnswered, wire.ChoiceStateDismissed:
		if r.Answer == nil || !ValidChoiceAnswer(*r.Answer, validators) || r.Answer.ID != r.ID || r.Answer.AnsweredMs < r.AskedMs || r.Answer.AnsweredMs >= r.Request.PendingUntilMs {
			return false
		}
		return r.State == wire.ChoiceStateAnswered && choiceOption(r.Request, r.Answer.Option) || r.State == wire.ChoiceStateDismissed && r.Answer.Option == ""
	}
	return false
}

func ValidAnswerEntry(e wire.AnswerEntry, validators Validators) bool {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return false
	}
	p := recorded{Publisher: e.Publisher, PublisherTrusted: e.PublisherTrusted}
	if !ValidRequestKey(e.ID) || e.Asker == "" || e.AskerProgram != "" && (!validAnswerer(e.AskerProgram, validators) || e.Asker != AskerDisplay(e.AskerProgram)) ||
		!validRecordedPublisher(&p) || e.AnsweredMs <= 0 {
		return false
	}
	switch e.AnsweredBy {
	case wire.AnswererOperator:
		if e.Answerer != "" {
			return false
		}
	case wire.AnswererResponder:
		if !validAnswerer(e.Answerer, validators) {
			return false
		}
	default:
		return false
	}
	switch e.Kind {
	case wire.AnswerKindQuestion:
		q, ok := catalogue.Find(e.Key)
		if !ok {
			return false
		}
		_, ok = q.Option(e.Option)
		return ok && e.Label == e.Option && (e.About == "" || permissionString(e.About, 4096)) && permissionString(e.Title, MaxAnswerTitleBytes)
	case wire.AnswerKindChoice:
		return e.Key == "" && e.About == "" && choiceText(e.Title) && ValidChoiceOptionID(e.Option) && choiceText(e.Label)
	}
	return false
}

func notificationLine(s string, limit int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= limit && strings.TrimSpace(s) != "" && strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
	}) < 0
}

func NotificationLine(s string, limit int) bool { return notificationLine(s, limit) }

func notificationBody(s string) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= 240 && strings.TrimSpace(s) != "" && strings.IndexFunc(s, func(r rune) bool {
		return r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r))
	}) < 0
}

func NotificationBody(s string) bool { return notificationBody(s) }

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

func notificationOffers(q wire.NotificationRequest, id string) bool {
	return slices.ContainsFunc(q.Actions, func(a wire.NotificationAction) bool { return a.ID == id })
}

func NotificationOffers(q wire.NotificationRequest, id string) bool { return notificationOffers(q, id) }

func ValidNotificationResult(r wire.NotificationResult, validators Validators) bool {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return false
	}
	if !ValidRequestKey(r.ID) || r.DecidedMs <= 0 || !validAnswerer(r.DecidedBy, validators) {
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

func ValidNotificationRecord(r wire.NotificationRecord, validators Validators) bool {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return false
	}
	if !ValidRequestKey(r.ID) || !validAnswerer(r.Program, validators) || r.Asker != AskerDisplay(r.Program) || !ValidNotificationRequest(r.Request) || r.AdmittedMs <= 0 || r.AdmittedMs >= r.Request.ExpiresMs || r.Request.RequestRef.ReplayUntilMs-r.AdmittedMs > MaxChoiceReplayMilliseconds {
		return false
	}
	p := recorded{Publisher: r.Publisher, PublisherTrusted: r.PublisherTrusted, ProgramDescription: r.ProgramDescription, ProgramOriginalName: r.ProgramOriginalName}
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
		if r.Result == nil || !ValidNotificationResult(*r.Result, validators) || r.Result.ID != r.ID || r.Result.State != r.State || r.Result.DecidedMs < r.AdmittedMs {
			return false
		}
		if r.State == wire.NotificationStateActed {
			return r.PostedMs > 0 && notificationOffers(r.Request, r.Result.Action)
		}
		return true
	}
	return false
}

func PairedPermission(action, resource string) bool {
	return action == PermissionContentWrite && slices.Contains([]string{"input:image", "input:audio", "output:image", "output:speech", "output:video"}, resource)
}

func ValidPermissionTarget(t wire.PermissionTarget, validators Validators) bool {
	return permissionString(t.Account, 128) && permissionString(t.Program, 4096) && (validators.NativeProgram != nil && validators.RemoteProgram != nil && (validators.NativeProgram(t.Program) || validators.RemoteProgram(t.Program))) &&
		permissionString(t.Action, 128) && !strings.ContainsAny(t.Action, " \t") && permissionString(t.Resource, 1024)
}

func ValidPermissionRequest(p wire.PermissionRequest, validators Validators) bool {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return false
	}
	if len(p.Targets) < 1 || len(p.Targets) > 2 || !ValidPermissionTarget(p.Targets[0], validators) {
		return false
	}
	primary := p.Targets[0]
	if len(p.Targets) == 2 && providerPermissionName(p) != "" {
		return ValidPermissionTarget(p.Targets[1], validators)
	}
	paired := PairedPermission(primary.Action, primary.Resource)
	if paired != (len(p.Targets) == 2) {
		return false
	}
	if paired {
		want := primary
		want.Action = PermissionContentRead
		if p.Targets[1] != want {
			return false
		}
	}
	return true
}

// Catalogue membership belongs to native admission. Retained occurrences keep
// their structural meaning when a later runtime removes a catalogue entry.
func providerPermissionName(p wire.PermissionRequest) string {
	if len(p.Targets) != 2 {
		return ""
	}
	primary, companion := p.Targets[0], p.Targets[1]
	name, ok := strings.CutPrefix(primary.Resource, "credential:")
	if !ok || name == "" || primary.Action != "abstraction.credentials/apply" {
		return ""
	}
	want := primary
	want.Action = "abstraction.inference/complete"
	want.Resource = "host:" + name
	if companion != want {
		return ""
	}
	return name
}

func ProviderPermissionName(p wire.PermissionRequest, validators Validators) (string, bool) {
	if !ValidPermissionRequest(p, validators) {
		return "", false
	}
	name := providerPermissionName(p)
	return name, name != ""
}

func LegacyPermission(account, key, about, text string, validators Validators) (wire.PermissionRequest, bool) {
	program, action, resource, ok := catalogue.FirstUse(key, about, text)
	if !ok || validators.NativeProgram == nil || !validators.NativeProgram(program) {
		return wire.PermissionRequest{}, false
	}
	t := wire.PermissionTarget{Account: account, Program: program, Action: action, Resource: resource}
	if !ValidPermissionTarget(t, validators) {
		return wire.PermissionRequest{}, false
	}
	p := wire.PermissionRequest{Targets: []wire.PermissionTarget{t}}
	if PairedPermission(action, resource) {
		paired := t
		paired.Action = PermissionContentRead
		p.Targets = append(p.Targets, paired)
	}
	return p, true
}

func ValidCatalogueContent(key, about string, content wire.CatalogueContent, validators Validators) bool {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return false
	}
	if content.Slots == nil || len(content.Slots) > 32 {
		return false
	}
	if key == catalogue.FirstUseKey {
		if content.Permission == nil || !ValidPermissionRequest(*content.Permission, validators) {
			return false
		}
		if len(content.Slots) == 0 {
			return true
		}
		if len(content.Slots) != 3 {
			return false
		}
		target := content.Permission.Targets[0]
		return content.Slots["program"] == target.Program && content.Slots["action"] == target.Action && content.Slots["resource"] == target.Resource &&
			catalogue.ValidQuestionSlot(target.Program) && catalogue.ValidQuestionSlot(target.Action) && catalogue.ValidQuestionSlot(target.Resource)
	}
	if key == Key || content.Permission != nil {
		return false
	}
	a, _, err := catalogue.ApplicationQuestion("retained question", key, content.Slots)
	if err != nil {
		return false
	}
	q, ok := catalogue.Find(key)
	return ok && a.Slots[q.About] == about
}

// DetailsReader reads trusted details from the retained asker method.
type DetailsReader interface {
	ReadQuestionAskerContext(ctx context.Context, id string) (wire.QuestionAsker, error)
}

type AskerReader = DetailsReader

// ReadAskerContext reads one occurrence's trusted details and bound asker
// (ASK-O11). Only an explicit unknown_method permits the existing legacy
// metadata reader; errors and malformed current details never fall back.
func ReadAskerContext(ctx context.Context, reader DetailsReader, account, id string, validators Validators) (wire.QuestionAsker, error) {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil || validators.PackagedProgramPrefix == "" {
		return wire.QuestionAsker{}, errors.New("asks: missing native presenter owner")
	}
	result, err := reader.ReadQuestionAskerContext(ctx, id)
	if err == nil {
		if err := validatePermissionDetails(result.Details, account, id, validators); err != nil {
			return wire.QuestionAsker{}, err
		}
		if err := validateAskerProgram(result.Details, result.Program, validators); err != nil {
			return wire.QuestionAsker{}, err
		}
		return result, nil
	}
	var service *wire.ServiceError
	if !errors.As(err, &service) || service.Code != wire.ServiceErrorCodeUnknownMethod {
		return wire.QuestionAsker{}, err
	}
	legacy, ok := reader.(interface {
		ReadAttributedQuestionContext(ctx context.Context, id string) (wire.AttributedQuestion, error)
	})
	if !ok {
		return wire.QuestionAsker{}, err
	}
	attributed, err := legacy.ReadAttributedQuestionContext(ctx, id)
	if err != nil {
		return wire.QuestionAsker{}, err
	}
	details := wire.QuestionDetails{Question: attributed}
	q := attributed.Question
	if q.Record != nil && q.Record.ID != id {
		return wire.QuestionAsker{}, errors.New("asks: mismatched permission occurrence")
	}
	if q.Outcome == wire.OperatorQuestionOutcomeFound && q.Record != nil && q.Record.Key == catalogue.FirstUseKey {
		p, ok := LegacyPermission(account, q.Record.Key, q.Record.About, q.Record.Text, validators)
		if !ok {
			return wire.QuestionAsker{}, errors.New("asks: invalid legacy permission")
		}
		details.Permission = &p
	}
	return wire.QuestionAsker{Details: details}, nil
}

// ReadDetailsContext selects the trusted details from the same asker read.
func ReadDetailsContext(ctx context.Context, reader DetailsReader, account, id string, validators Validators) (wire.QuestionDetails, error) {
	read, err := ReadAskerContext(ctx, reader, account, id, validators)
	return read.Details, err
}

func validatePermissionDetails(details wire.QuestionDetails, account, id string, validators Validators) error {
	q := details.Question.Question
	if q.Record != nil && q.Record.ID != id {
		return errors.New("asks: mismatched permission occurrence")
	}
	if p := details.Permission; p != nil && (q.Outcome != wire.OperatorQuestionOutcomeFound || q.Record == nil || q.Record.Key != catalogue.FirstUseKey || !ValidPermissionRequest(*p, validators) || p.Targets[0].Account != account) {
		return errors.New("asks: foreign or malformed permission details")
	}
	if q.Outcome == wire.OperatorQuestionOutcomeFound && q.Record != nil && q.Record.Key == catalogue.FirstUseKey && details.Permission == nil {
		return errors.New("asks: missing structured permission")
	}
	return nil
}

func validateAskerProgram(details wire.QuestionDetails, program string, validators Validators) error {
	if program == "" {
		return nil
	}
	q := details.Question.Question
	if q.Outcome != wire.OperatorQuestionOutcomeFound || q.Record == nil || len(program) > 4096 || !utf8.ValidString(program) ||
		!validators.NativeProgram(program) || strings.HasPrefix(program, validators.PackagedProgramPrefix) || q.Record.Asker != AskerDisplay(program) {
		return errors.New("asks: malformed asker program")
	}
	return nil
}

func validateDetails(result wire.QuestionDetails, id string, validators Validators) error {
	if err := validateQuestionRead(result.Question.Question, id); err != nil {
		return err
	}
	q := result.Question.Question
	if result.Question.Answerer != "" && (q.AnsweredBy == nil || *q.AnsweredBy != wire.AnswererResponder || !validators.NativeProgram(result.Question.Answerer)) {
		return errors.New("asks: malformed permission attribution")
	}
	if result.Permission != nil && (q.Outcome != wire.OperatorQuestionOutcomeFound || q.Record.Key != catalogue.FirstUseKey || !ValidPermissionRequest(*result.Permission, validators)) ||
		q.Outcome == wire.OperatorQuestionOutcomeFound && q.Record.Key == catalogue.FirstUseKey && result.Permission == nil {
		return errors.New("asks: malformed permission details")
	}
	return nil
}

func CheckQuestionDetails(v wire.QuestionDetails, id string, validators Validators) error {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return errors.New("asks: missing native presenter owner")
	}
	return validateDetails(v, id, validators)
}
func CheckAskerProgram(v wire.QuestionDetails, program string, validators Validators) error {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil || validators.PackagedProgramPrefix == "" {
		return errors.New("asks: missing native presenter owner")
	}
	return validateAskerProgram(v, program, validators)
}
func CheckPermissionDetails(v wire.QuestionDetails, account, id string, validators Validators) error {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return errors.New("asks: missing native presenter owner")
	}
	return validatePermissionDetails(v, account, id, validators)
}

func validateContent(result wire.QuestionContent, validators Validators) error {
	if result.Content == nil {
		return nil
	}
	question := result.Question.Details.Question.Question
	if question.Outcome != wire.OperatorQuestionOutcomeFound || question.Record == nil || !ValidCatalogueContent(question.Record.Key, question.Record.About, *result.Content, validators) {
		return errors.New("asks: invalid catalogue content")
	}
	if question.Record.Key == catalogue.FirstUseKey {
		permission := result.Question.Details.Permission
		if permission == nil || !slices.Equal(permission.Targets, result.Content.Permission.Targets) {
			return errors.New("asks: mismatched catalogue permission")
		}
	}
	return nil
}

func ReadContentContext(ctx context.Context, reader DetailsReader, account, id string, validators Validators) (wire.QuestionContent, error) {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil || validators.PackagedProgramPrefix == "" {
		return wire.QuestionContent{}, errors.New("asks: missing native presenter owner")
	}
	if contentReader, ok := reader.(interface {
		ReadQuestionContentContext(context.Context, string) (wire.QuestionContent, error)
	}); ok {
		result, err := contentReader.ReadQuestionContentContext(ctx, id)
		if err == nil {
			if err = validatePermissionDetails(result.Question.Details, account, id, validators); err != nil {
				return wire.QuestionContent{}, err
			}
			if err = validateAskerProgram(result.Question.Details, result.Question.Program, validators); err != nil {
				return wire.QuestionContent{}, err
			}
			if err = validateContent(result, validators); err != nil {
				return wire.QuestionContent{}, err
			}
			return result, nil
		}
		var service *wire.ServiceError
		if !errors.As(err, &service) || service.Code != wire.ServiceErrorCodeUnknownMethod {
			return wire.QuestionContent{}, err
		}
	}
	question, err := ReadAskerContext(ctx, reader, account, id, validators)
	return wire.QuestionContent{Question: question}, err
}

func CheckContent(v wire.QuestionContent, validators Validators) error {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return errors.New("asks: missing native presenter owner")
	}
	return validateContent(v, validators)
}
