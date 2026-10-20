package asks

import "testing"

// The self-registration question renders the registering program's path as
// its About slot, and SelfRegistered reads program and title back from it.
func TestSelfRegisteredReadsProgramAndTitle(t *testing.T) {
	q, ok := Find(SelfRegisteredKey)
	if !ok {
		t.Fatal("no self-registration question")
	}
	program := `C:\Apps\Editor\editor.exe`
	text, err := q.Render(Ask{Key: SelfRegisteredKey, Slots: map[string]string{"program": program, "title": "Editor. Keep it? Pro"}})
	if err != nil {
		t.Fatal(err)
	}
	got, title, ok := SelfRegistered(SelfRegisteredKey, program, text)
	if !ok || got != program || title != "Editor. Keep it? Pro" {
		t.Fatalf("SelfRegistered(%q) = %q %q %t", text, got, title, ok)
	}
	for _, c := range []struct{ key, about, text string }{
		{FirstUseKey, program, text},
		{SelfRegisteredKey, "", text},
		{SelfRegisteredKey, `C:\Other\editor.exe`, text},
		{SelfRegisteredKey, program, program + " added itself to OpenAbstractions as . Keep it?"},
	} {
		if _, _, ok := SelfRegistered(c.key, c.about, c.text); ok {
			t.Errorf("SelfRegistered(%q, %q, %q) read a question", c.key, c.about, c.text)
		}
	}
}
