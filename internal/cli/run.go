package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/jackiehank/kiln/internal/config"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
)

// Run executes one command and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	// The language switch is global: pull --lang out before dispatch, then pick
	// the language the environment asks for. It must happen before any output so
	// even parse errors and the usage text follow the chosen language.
	args, langFlag, bad, ok := extractLang(args)
	if langFlag != "" {
		i18n.Set(langFlag)
	} else {
		i18n.Set(i18n.ResolveFromEnv())
	}
	if !ok {
		if bad == "" {
			fmt.Fprint(stderr, i18n.T("--lang 需要一个值（zh 或 en）\n"))
		} else {
			fmt.Fprint(stderr, i18n.T("--lang 只接受 zh 或 en（当前: %s）\n", bad))
		}
		return 2
	}
	// Load config before help so `-h` reflects the project's relation vocabulary.
	// Help must not fail on a broken config: fall back to the built-in default.
	cfg, cfgErr := config.Read(".")
	rel := model.DefaultRelationSet()
	if cfgErr == nil && cfg.Relations != nil {
		rel = cfg.Relations
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText(rel))
		return 2
	}
	cmd, rest := args[0], args[1:]
	// `<cmd> -h` is the shape a reader tries first; intercept it before dispatch
	// so no parser sees -h as a positional (it used to be read as an issue id).
	if len(rest) == 1 && (rest[0] == "-h" || rest[0] == "--help") {
		if text, ok := commandUsageText(rel, cmd); ok {
			fmt.Fprint(stdout, text)
			return 0
		}
	}
	switch cmd {
	case "help", "-h", "--help":
		if len(rest) == 1 {
			if text, ok := commandUsageText(rel, rest[0]); ok {
				fmt.Fprint(stdout, text)
				return 0
			}
		}
		fmt.Fprint(stdout, usageText(rel))
		return 0
	case "version", "--version", "-V":
		return printVersion(stdout)
	case "do", "commit", "path", "status", "check", "list", "graph", "show", "worktree", "config", "action", "cache":
		// handled below
	default:
		if now, ok := retiredCommands[cmd]; ok {
			fmt.Fprint(stderr, i18n.T("未知命令: %s（已改名为 `%s`）\n\n%s", cmd, now, usageText(rel)))
			return 2
		}
		fmt.Fprint(stderr, i18n.T("未知命令: %s\n\n%s", cmd, usageText(rel)))
		return 2
	}

	if cfgErr != nil {
		fmt.Fprint(stderr, i18n.T("配置错误: %v\n", cfgErr))
		return 1
	}
	e := &env{
		actions:      cfg.Actions,
		relations:    cfg.Relations,
		dir:          ".",
		vcsPref:      cfg.VCS,
		worktree:     cfg.Worktree,
		configPath:   cfg.Path,
		configSource: cfg.Source,
		out:          stdout,
		errw:         stderr,
	}
	switch cmd {
	case "do":
		return e.write(rest, false)
	case "commit":
		return e.write(rest, true)
	case "path":
		return e.path(rest)
	case "show":
		return e.show(rest)
	case "status":
		return e.status(rest)
	case "check":
		return e.check(rest)
	case "list":
		return e.list(rest)
	case "graph":
		return e.graph(rest)
	case "worktree":
		return e.workspace(rest)
	case "config":
		return e.config(rest)
	case "action":
		return e.action(rest)
	case "cache":
		return e.cacheCmd(rest)
	}
	return 2
}

// extractLang pulls the global --lang flag out of the arguments before dispatch,
// so every command shares one switch instead of teaching each parser about it.
// A bare --lang with no value, or a value kiln does not know, comes back with
// ok=false and the offending value so Run can print the error in the language
// the user asked for.
func extractLang(args []string) (rest []string, lang i18n.Lang, bad string, ok bool) {
	ok = true
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--lang" {
			// A following token that starts with "--" is not a language either;
			// report it as a missing value instead of "got --json", which reads
			// like a typo of the language rather than of the flag ordering.
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return nil, "", "", false
			}
			l, good := i18n.Parse(args[i+1])
			if !good {
				return nil, "", args[i+1], false
			}
			lang = l
			i++
			continue
		}
		if v, found := strings.CutPrefix(a, "--lang="); found {
			l, good := i18n.Parse(v)
			if !good {
				return nil, "", v, false
			}
			lang = l
			continue
		}
		rest = append(rest, a)
	}
	return rest, lang, "", true
}

// printVersion answers "which kiln am I running, and how old is it?". A stale
// binary once hid new flags behind a warning, discovered by accident; the facts
// belong in one command instead of a guess. Version comes from the build info
// (no hand-kept constant, no ldflags); the build time is the executable's mtime,
// which is what answers "is this file current?".
func printVersion(w io.Writer) int {
	version := "devel"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	fmt.Fprintf(w, "kiln %s\n", version)
	if exe, err := os.Executable(); err == nil {
		fmt.Fprint(w, i18n.T("可执行文件: %s\n", exe))
		if st, err := os.Stat(exe); err == nil {
			fmt.Fprint(w, i18n.T("构建时间: %s\n", st.ModTime().Format("2006-01-02 15:04:05")))
		}
	}
	fmt.Fprintf(w, "%s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return 0
}

// retiredCommands maps a name kiln no longer answers to its current spelling, so
// a hard cut still tells the caller where the command went.
var retiredCommands = map[string]string{"workspace": "worktree", "no-cache": "cache clear"}
