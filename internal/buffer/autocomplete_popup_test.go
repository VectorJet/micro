package buffer

import "testing"

func TestAutocompletePopupKeepsSourceTextUntouched(t *testing.T) {
	b := NewBufferFromString("pri", "", BTDefault)
	b.GetActiveCursor().GotoLoc(Loc{X: 3, Y: 0})

	completer := func(*Buffer) ([]string, []string) {
		return []string{"ntf", "vate"}, []string{"printf", "private"}
	}
	if !b.AutocompletePopup(completer) {
		t.Fatal("AutocompletePopup() returned false")
	}
	if got := string(b.Bytes()); got != "pri" {
		t.Fatalf("opening popup changed source text: got %q, want %q", got, "pri")
	}
	if !b.CompletionPopup || !b.HasSuggestions {
		t.Fatal("popup state was not activated")
	}
	if b.CurSuggestion != 0 {
		t.Fatalf("initial selection = %d, want 0", b.CurSuggestion)
	}

	b.CycleAutocomplete(true)
	if got := string(b.Bytes()); got != "pri" {
		t.Fatalf("cycling popup changed source text: got %q, want %q", got, "pri")
	}
	if b.CurSuggestion != 1 {
		t.Fatalf("cycled selection = %d, want 1", b.CurSuggestion)
	}
}

func TestAutocompleteKeepsLegacyPreviewBehavior(t *testing.T) {
	b := NewBufferFromString("pri", "", BTDefault)
	b.GetActiveCursor().GotoLoc(Loc{X: 3, Y: 0})

	completer := func(*Buffer) ([]string, []string) {
		return []string{"ntf", "vate"}, []string{"printf", "private"}
	}
	if !b.Autocomplete(completer) {
		t.Fatal("Autocomplete() returned false")
	}
	if got := string(b.Bytes()); got != "printf" {
		t.Fatalf("legacy autocomplete preview = %q, want %q", got, "printf")
	}
	if b.CompletionPopup {
		t.Fatal("legacy autocomplete unexpectedly enabled floating popup mode")
	}
}
