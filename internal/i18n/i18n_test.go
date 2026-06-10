package i18n

import (
	"os"
	"strings"
	"testing"
)

// resetState clears the cached language so tests can re-detect from env.
func resetState(t *testing.T) {
	t.Helper()
	mu.Lock()
	current = ""
	resolved = false
	mu.Unlock()
}

func TestDetect_Spanish(t *testing.T) {
	resetState(t)
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "es_MX.UTF-8")
	if got := Current(); got != LangES {
		t.Errorf("Current() = %q; want %q", got, LangES)
	}
}

func TestDetect_English(t *testing.T) {
	resetState(t)
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "en_US.UTF-8")
	if got := Current(); got != LangEN {
		t.Errorf("Current() = %q; want %q", got, LangEN)
	}
}

func TestDetect_FallsBackToEnglish(t *testing.T) {
	resetState(t)
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "")
	os.Unsetenv("LC_ALL")
	os.Unsetenv("LANG")
	if got := Current(); got != LangEN {
		t.Errorf("Current() = %q; want fallback to %q", got, LangEN)
	}
}

func TestDetect_LCAllWinsOverLang(t *testing.T) {
	resetState(t)
	t.Setenv("LC_ALL", "es_ES.UTF-8")
	t.Setenv("LANG", "en_US.UTF-8")
	if got := Current(); got != LangES {
		t.Errorf("LC_ALL should win over LANG; got %q want %q", got, LangES)
	}
}

func TestT_ReturnsTranslation(t *testing.T) {
	Set(LangES)
	defer Set("")
	got := T("risk.safe")
	if !strings.Contains(got, "seguro") {
		t.Errorf("expected Spanish translation, got %q", got)
	}
}

func TestT_FallsBackToEnglish(t *testing.T) {
	// Inject a key that exists in english only by adding it temporarily.
	english["test.only.english"] = "english only"
	defer delete(english, "test.only.english")

	Set(LangES)
	defer Set("")
	got := T("test.only.english")
	if got != "english only" {
		t.Errorf("expected english fallback, got %q", got)
	}
}

func TestT_ReturnsKeyWhenMissingEverywhere(t *testing.T) {
	Set(LangEN)
	defer Set("")
	got := T("does.not.exist.anywhere")
	if got != "does.not.exist.anywhere" {
		t.Errorf("expected key as fallback, got %q", got)
	}
}

func TestT_FormatsArguments(t *testing.T) {
	Set(LangEN)
	defer Set("")
	got := T("ui.days-ago", 7)
	if got != "7 days ago" {
		t.Errorf("got %q", got)
	}
}

// TestCatalogParity ensures every English key has a Spanish translation.
// Missing entries are visible immediately when this test fails.
func TestCatalogParity(t *testing.T) {
	missing := []string{}
	for k := range english {
		if !Has(LangES, k) {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		t.Errorf("Spanish catalog missing %d keys:\n%s",
			len(missing), strings.Join(missing, "\n"))
	}
}

// TestNoExtraSpanishKeys ensures Spanish doesn't drift with strings that
// English doesn't know about (would be unreachable on en_US users).
func TestNoExtraSpanishKeys(t *testing.T) {
	extra := []string{}
	for k := range spanish {
		if !Has(LangEN, k) {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		t.Errorf("Spanish catalog has %d keys not in English:\n%s",
			len(extra), strings.Join(extra, "\n"))
	}
}
