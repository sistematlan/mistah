// Package i18n provides minimal localization without external dependencies.
//
// Design:
//   - Strings are looked up by stable keys (e.g. "caches.npm.name").
//   - Catalogs live as Go maps; one per language.
//   - Language is auto-detected from $LANG / $LC_ALL on first use.
//   - Fallback chain: requested lang → "en" → key itself.
//
// Why not go-i18n / nicholasjackson / similar?
//   - Adding a dependency for ~200 strings is overkill.
//   - We never need plurals/dates beyond formatting that fmt.Sprintf already handles.
//   - Go maps are zero-cost and trivially testable.
//
// Modes:
//   The same logical concept may have two phrasings:
//
//     "caches.npm.detail.advanced" → "downloaded packages cache (~/.npm/_cacache)"
//     "caches.npm.detail.simple"   → "Paquetes de Node.js descargados"
//
//   Callers pick which to use via Detail(key, simple bool).
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Lang is a 2-letter language code (ISO 639-1). We support "es" and "en".
type Lang string

const (
	LangES Lang = "es"
	LangEN Lang = "en"
)

// state holds the resolved language and overrides for tests.
var (
	mu        sync.RWMutex
	current   Lang
	resolved  bool
)

// Set forces the active language. Empty string falls back to autodetect.
// Mostly useful for tests and for the `--lang` global flag.
func Set(l Lang) {
	mu.Lock()
	defer mu.Unlock()
	current = l
	resolved = l != ""
}

// Current returns the active language, resolving once if needed.
func Current() Lang {
	mu.RLock()
	if resolved {
		l := current
		mu.RUnlock()
		return l
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if !resolved {
		current = detect()
		resolved = true
	}
	return current
}

// detect inspects environment variables in priority order.
//
// Priority: $LC_ALL > $LANG > "en". Anything that starts with "es" maps
// to Spanish; otherwise English. Variants like "es_MX.UTF-8" are handled.
func detect() Lang {
	for _, env := range []string{"LC_ALL", "LANG"} {
		if v := os.Getenv(env); v != "" {
			low := strings.ToLower(v)
			if strings.HasPrefix(low, "es") {
				return LangES
			}
			if strings.HasPrefix(low, "en") {
				return LangEN
			}
		}
	}
	return LangEN
}

// T returns the translation for key in the active language.
//
// Lookup order: current → English → key (so missing translations are visible
// during development without crashing). Optional fmt args are forwarded to
// fmt.Sprintf, so callers can write T("disk.usage", used, total).
func T(key string, args ...any) string {
	return TIn(Current(), key, args...)
}

// TIn is T but with an explicit language. Used by tests and by callers that
// already resolved the language elsewhere.
func TIn(l Lang, key string, args ...any) string {
	cat := catalog(l)
	if s, ok := cat[key]; ok {
		if len(args) > 0 {
			return fmt.Sprintf(s, args...)
		}
		return s
	}
	// Fallback to English.
	if l != LangEN {
		if s, ok := catalog(LangEN)[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(s, args...)
			}
			return s
		}
	}
	// Last resort: the key itself, so missing strings are visible.
	return key
}

// catalog returns the map for a given language, or English as fallback.
func catalog(l Lang) map[string]string {
	switch l {
	case LangES:
		return spanish
	case LangEN:
		return english
	default:
		return english
	}
}

// Has reports whether key exists in the language's catalog.
// Used by tests to verify coverage parity between es and en.
func Has(l Lang, key string) bool {
	_, ok := catalog(l)[key]
	return ok
}

// Keys returns all keys defined in a language's catalog.
// Order is not guaranteed.
func Keys(l Lang) []string {
	cat := catalog(l)
	keys := make([]string, 0, len(cat))
	for k := range cat {
		keys = append(keys, k)
	}
	return keys
}
