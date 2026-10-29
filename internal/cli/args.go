package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jackiehank/kiln/internal/i18n"
)

// flags holds repeatable --key value pairs parsed from a command's arguments.
type flags map[string][]string

// missingValue records a value flag that found no value: either the command
// ended or the next token starts with "--". The parser cannot tell a missing
// value from "the next flag follows", so it reports the fact and lets the
// command decide — known flags are refused, unknown ones stay warnings so an
// older binary still tolerates a newer flag (see rejectMissingValues).
type missingValue struct{ flag, next string }

// allowedFlags builds a set from names.
func allowedFlags(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// warnUnknownFlags prints a warning for every supplied flag not in allowed.
// Warnings go to stderr so they never pollute a `do` message destined for a
// commit; kiln stays honest about what it accepted and what it ignored.
func warnUnknownFlags(errw io.Writer, f flags, allowed map[string]bool) {
	var unknown []string
	for k := range f {
		if !allowed[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		fmt.Fprint(errw, i18n.T("警告: 未知 flag --%s（可用 flag 见 kiln -h）\n", k))
	}
}

func (f flags) get(key string) string {
	if v := f[key]; len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
}

// has reports whether a (possibly value-less) flag was supplied at all.
func (f flags) has(key string) bool { return len(f[key]) > 0 }

// list returns the values for a list-valued key, flattening comma-separated
// strings: each collected value is split on commas, each item trimmed, and
// blank items dropped. This makes `--refs a,b` and `--refs a --refs b` produce
// the same canonical output.
func (f flags) list(key string) []string {
	var out []string
	for _, v := range f[key] {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// parse splits args into positionals and flags. `--key=value`, `--key value` and
// repeated `--key` all accumulate. A flag listed in switches never consumes the
// following token, so `--json <id>` still leaves <id> a positional; giving a
// switch an explicit value (--json=x) is refused by rejectSwitchValues. A value
// flag with no value (end of args, or the next token starts with --) is reported
// in missing instead of being guessed at; a literal value that starts with --
// is written as --key=value, which this parser always takes verbatim.
func parse(args []string, switches ...string) ([]string, flags, []missingValue) {
	pos := make([]string, 0, len(args))
	f := flags{}
	var missing []missingValue
	isSwitch := make(map[string]bool, len(switches))
	for _, name := range switches {
		isSwitch[name] = true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			pos = append(pos, a)
			continue
		}
		body := a[2:]
		key, val := body, ""
		if j := strings.Index(body, "="); j >= 0 {
			key, val = body[:j], body[j+1:]
		} else if !isSwitch[key] {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				val = args[i+1]
				i++
			} else {
				next := ""
				if i+1 < len(args) {
					next = args[i+1]
				}
				missing = append(missing, missingValue{flag: key, next: next})
			}
		}
		f[key] = append(f[key], val)
	}
	return pos, f, missing
}

// rejectMissingValues refuses the first value flag that was given without a
// value, but only when the command knows that flag. An unknown name is left to
// warnUnknownFlags: refusing it would break the forward-compatibility rule that
// lets an older binary warn about, rather than reject, a flag a newer caller
// sends. The fix for a literal value that starts with "--" is spelled out in
// the message: --flag=value, which parse takes verbatim.
func rejectMissingValues(errw io.Writer, missing []missingValue, allowed map[string]bool) int {
	for _, m := range missing {
		if !allowed[m.flag] {
			continue
		}
		if m.next == "" {
			fmt.Fprint(errw, i18n.T("flag --%s 需要一个值（命令末尾）\n", m.flag))
			return 2
		}
		fmt.Fprint(errw, i18n.T("flag --%s 需要一个值（下一个 token 是 %s，以 -- 开头；若它就是字面值，请写 --%s=%s）\n", m.flag, m.next, m.flag, m.next))
		return 2
	}
	return 0
}

// rejectSwitchValues refuses an explicit value on a switch flag (--json=x).
// parse leaves a switch value-less when it stands alone, so a non-empty value
// can only come from `=`, which no command means.
func rejectSwitchValues(errw io.Writer, f flags, names ...string) int {
	for _, name := range names {
		if v := f.get(name); v != "" {
			fmt.Fprint(errw, i18n.T("flag --%s 不接受值: %s\n", name, v))
			return 2
		}
	}
	return 0
}

// rejectRepeats refuses a value flag given more than once: keeping the last one
// silently hides a mistyped second value. List-valued flags (tags, refs,
// relations, --only) accumulate on purpose and are not passed here.
func rejectRepeats(errw io.Writer, f flags, names ...string) int {
	for _, name := range names {
		if n := len(f[name]); n > 1 {
			fmt.Fprint(errw, i18n.T("flag --%s 只能给一次（当前给了 %d 次）\n", name, n))
			return 2
		}
	}
	return 0
}

// rejectExtra refuses positionals beyond max and prints the command's shape.
func rejectExtra(errw io.Writer, pos []string, max int, usage string) int {
	if len(pos) <= max {
		return 0
	}
	fmt.Fprint(errw, i18n.T("多余的参数: %s\n", strings.Join(pos[max:], " ")))
	fmt.Fprint(errw, i18n.T("用法: %s\n", usage))
	return 2
}

// dedupeStrings drops repeats while keeping the first occurrence's order.
func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
