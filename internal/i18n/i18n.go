// Package i18n holds kiln's user-facing prose. Every translatable message is
// written once, in Chinese, at the point that prints it; the catalog below maps
// that exact source string to English. Missing entries fall back to the source,
// so a half-translated build stays readable instead of printing keys at the
// caller.
//
// The machine-readable surface (--json, rule codes, the action/relation
// vocabulary, trailers) is deliberately outside this package: those words are
// data with fixed spellings, and translating them would make one repository
// answer differently depending on the reader's locale.
package i18n

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Lang is a supported output language.
type Lang string

const (
	Zh Lang = "zh"
	En Lang = "en"
)

// current is a package-level setting on purpose. Output happens from a single
// goroutine per process (the CLI never prints concurrently), and threading a
// language through every rendering helper would add a parameter to code that
// has no other reason to care. Tests set the language explicitly.
var current = Zh

// Set fixes the language for this process.
func Set(l Lang) { current = l }

// Current reports the language in effect.
func Current() Lang { return current }

// Detect maps a POSIX locale name to a language. English locales start with
// "en" (en, en_US, en_GB.UTF-8); everything else falls back to Chinese, which
// is kiln's source language and default.
func Detect(locale string) Lang {
	if strings.HasPrefix(strings.ToLower(locale), "en") {
		return En
	}
	return Zh
}

// Parse validates an explicit language name.
func Parse(s string) (Lang, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "zh", "zh-cn", "zh_cn", "zh-hans", "chinese", "中文":
		return Zh, true
	case "en", "en-us", "en_us", "en-gb", "en_gb", "english":
		return En, true
	}
	return "", false
}

// ResolveFromEnv picks the language from the environment: an explicit
// KILN_LANG wins, then the POSIX locale variables in their documented order,
// then Chinese.
func ResolveFromEnv() Lang {
	if l, ok := Parse(os.Getenv("KILN_LANG")); ok {
		return l
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return Detect(v)
		}
	}
	return Zh
}

// T translates a source message and formats its arguments. With no arguments it
// returns the message untouched, so a caller that concatenates fragments still
// gets the source text in Chinese.
func T(source string, args ...any) string {
	msg := lookup(source)
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// lookup returns the translated text for source, falling back to source itself
// so a string added after the catalog still prints something readable.
func lookup(source string) string {
	if current == En {
		if en, ok := enCatalog[source]; ok {
			return en
		}
	}
	return source
}

// E is T for error values. It goes through fmt.Errorf, not fmt.Sprintf, so a %w
// in the translated format keeps its unwrap chain intact.
func E(source string, args ...any) error {
	if len(args) == 0 {
		return errors.New(lookup(source))
	}
	return fmt.Errorf(lookup(source), args...)
}
