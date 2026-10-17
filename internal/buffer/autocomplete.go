package buffer

import (
	"bytes"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/zyedidia/micro/v2/internal/util"
)

// A Completer is a function that takes a buffer and returns info
// describing what autocompletions should be inserted at the current
// cursor location
// It returns a list of string suggestions which will be inserted at
// the current cursor location if selected as well as a list of
// suggestion names which can be displayed in an autocomplete box or
// other UI element
type Completer func(*Buffer) ([]string, []string)

func (b *Buffer) GetSuggestions() {

}

// Autocomplete starts Micro's legacy autocomplete process. The initial
// suggestion is preview-inserted, as expected by existing command/file prompts.
func (b *Buffer) Autocomplete(c Completer) bool {
	b.CompletionPopup = false
	b.Completions, b.Suggestions = c(b)
	if len(b.Completions) != len(b.Suggestions) || len(b.Completions) == 0 {
		b.HasSuggestions = false
		return false
	}
	b.CurSuggestion = -1
	b.CycleAutocomplete(true)
	return true
}

// AutocompletePopup populates the selection list without modifying source text.
// It is intended for language-server completions shown in the editor viewport.
func (b *Buffer) AutocompletePopup(c Completer) bool {
	b.Completions, b.Suggestions = c(b)
	if len(b.Completions) != len(b.Suggestions) || len(b.Completions) == 0 {
		b.HasSuggestions = false
		b.CompletionPopup = false
		return false
	}
	b.CurSuggestion = 0
	b.CompletionLoc = b.GetActiveCursor().Loc
	b.CompletionPopup = true
	b.HasSuggestions = true
	return true
}

// CycleAutocomplete cycles suggestions. Floating completions change only the
// selection; legacy completions continue preview-inserting the chosen text.
func (b *Buffer) CycleAutocomplete(forward bool) {
	if b.CompletionPopup {
		if len(b.Suggestions) == 0 {
			b.HasSuggestions = false
			b.CompletionPopup = false
			return
		}
		if forward {
			b.CurSuggestion++
		} else {
			b.CurSuggestion--
		}
		if b.CurSuggestion >= len(b.Suggestions) {
			b.CurSuggestion = 0
		} else if b.CurSuggestion < 0 {
			b.CurSuggestion = len(b.Suggestions) - 1
		}
		b.HasSuggestions = true
		return
	}

	prevSuggestion := b.CurSuggestion
	if forward {
		b.CurSuggestion++
	} else {
		b.CurSuggestion--
	}
	if b.CurSuggestion >= len(b.Suggestions) {
		b.CurSuggestion = 0
	} else if b.CurSuggestion < 0 {
		b.CurSuggestion = len(b.Suggestions) - 1
	}

	c := b.GetActiveCursor()
	start := c.Loc
	end := c.Loc
	if prevSuggestion < len(b.Suggestions) && prevSuggestion >= 0 {
		start = end.Move(-util.CharacterCountInString(b.Completions[prevSuggestion]), b)
	}
	b.Replace(start, end, b.Completions[b.CurSuggestion])
	if len(b.Suggestions) > 1 {
		b.HasSuggestions = true
	}
}

// GetWord gets the most recent word separated by any separator
// (whitespace, punctuation, any non alphanumeric character)
func (b *Buffer) GetWord() ([]byte, int) {
	c := b.GetActiveCursor()
	l := b.LineBytes(c.Y)
	l = util.SliceStart(l, c.X)

	if c.X == 0 || util.IsWhitespace(b.RuneAt(c.Loc.Move(-1, b))) {
		return []byte{}, -1
	}

	if util.IsNonWordChar(b.RuneAt(c.Loc.Move(-1, b))) {
		return []byte{}, c.X
	}

	args := bytes.FieldsFunc(l, util.IsNonWordChar)
	input := args[len(args)-1]
	return input, c.X - util.CharacterCount(input)
}

// GetArg gets the most recent word (separated by ' ' only)
func (b *Buffer) GetArg() (string, int) {
	c := b.GetActiveCursor()
	l := b.LineBytes(c.Y)
	l = util.SliceStart(l, c.X)

	args := bytes.Split(l, []byte{' '})
	input := string(args[len(args)-1])
	argstart := 0
	for i, a := range args {
		if i == len(args)-1 {
			break
		}
		argstart += util.CharacterCount(a) + 1
	}

	return input, argstart
}

// FileComplete autocompletes filenames
func FileComplete(b *Buffer) ([]string, []string) {
	c := b.GetActiveCursor()
	input, argstart := b.GetArg()

	sep := string(os.PathSeparator)
	dirs := strings.Split(input, sep)

	var files []os.FileInfo
	var err error
	if len(dirs) > 1 {
		directories := strings.Join(dirs[:len(dirs)-1], sep) + sep

		directories, _ = util.ReplaceHome(directories)
		files, err = ioutil.ReadDir(directories)
	} else {
		files, err = ioutil.ReadDir(".")
	}

	if err != nil {
		return nil, nil
	}

	var suggestions []string
	for _, f := range files {
		name := f.Name()
		if f.IsDir() {
			name += sep
		}
		if strings.HasPrefix(name, dirs[len(dirs)-1]) {
			suggestions = append(suggestions, name)
		}
	}

	sort.Strings(suggestions)
	completions := make([]string, len(suggestions))
	for i := range suggestions {
		var complete string
		if len(dirs) > 1 {
			complete = strings.Join(dirs[:len(dirs)-1], sep) + sep + suggestions[i]
		} else {
			complete = suggestions[i]
		}
		completions[i] = util.SliceEndStr(complete, c.X-argstart)
	}

	return completions, suggestions
}

// BufferComplete autocompletes based on previous words in the buffer
func BufferComplete(b *Buffer) ([]string, []string) {
	c := b.GetActiveCursor()
	input, argstart := b.GetWord()

	if argstart == -1 {
		return []string{}, []string{}
	}

	inputLen := util.CharacterCount(input)

	suggestionsSet := make(map[string]struct{})

	var suggestions []string
	for i := c.Y; i >= 0; i-- {
		l := b.LineBytes(i)
		words := bytes.FieldsFunc(l, util.IsNonWordChar)
		for _, w := range words {
			if bytes.HasPrefix(w, input) && util.CharacterCount(w) > inputLen {
				strw := string(w)
				if _, ok := suggestionsSet[strw]; !ok {
					suggestionsSet[strw] = struct{}{}
					suggestions = append(suggestions, strw)
				}
			}
		}
	}
	for i := c.Y + 1; i < b.LinesNum(); i++ {
		l := b.LineBytes(i)
		words := bytes.FieldsFunc(l, util.IsNonWordChar)
		for _, w := range words {
			if bytes.HasPrefix(w, input) && util.CharacterCount(w) > inputLen {
				strw := string(w)
				if _, ok := suggestionsSet[strw]; !ok {
					suggestionsSet[strw] = struct{}{}
					suggestions = append(suggestions, strw)
				}
			}
		}
	}
	if len(suggestions) > 1 {
		suggestions = append(suggestions, string(input))
	}

	completions := make([]string, len(suggestions))
	for i := range suggestions {
		completions[i] = util.SliceEndStr(suggestions[i], c.X-argstart)
	}

	return completions, suggestions
}
