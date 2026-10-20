package catalogue

import "testing"

func TestQuestionPresentationCatalogueComplete(t *testing.T) {
	for _, question := range Questions {
		family, declared := questionPresentations[question.Key]
		if !declared || family == PresentationInvalid {
			t.Fatalf("%s has no explicit presentation", question.Key)
		}
		if consent := family == PresentationConsent || family == PresentationPairingConsent; consent != RuntimeOnlyKey(question.Key) {
			t.Fatalf("%s presentation does not match service-only admission", question.Key)
		}
	}
	for key, family := range questionPresentations {
		if _, found := Find(key); !found || family == PresentationInvalid {
			t.Fatalf("invalid presentation metadata for %s", key)
		}
	}
	if PresentationForQuestion(TypedPermissionKey) != PresentationConsent {
		t.Fatal("typed permission missing")
	}
	if PresentationForQuestion("free.prose") != PresentationQuestion {
		t.Fatal("free prose promoted")
	}
	if AutomaticNativeQuestion("download.reach") || AutomaticNativeQuestion("free.prose") {
		t.Fatal("ordinary confirmation acquired security presentation")
	}
	old := questionPresentations["download.reach"]
	questionPresentations["download.reach"] = PresentationConsent
	if PresentationForQuestion("download.reach") != PresentationInvalid || AutomaticNativeQuestion("download.reach") {
		t.Fatal("metadata alone promoted application-callable key to security consent")
	}
	questionPresentations["download.reach"] = old
	Questions = append(Questions, Question{Key: "test.new.catalogue.entry"})
	defer func() { Questions = Questions[:len(Questions)-1] }()
	if PresentationForQuestion("test.new.catalogue.entry") != PresentationInvalid {
		t.Fatal("undeclared new catalogue route accepted")
	}
}
