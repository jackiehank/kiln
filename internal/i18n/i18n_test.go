package i18n

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := map[string]Lang{
		"en":          En,
		"en_US.UTF-8": En,
		"en_GB":       En,
		"EN":          En,
		"zh_CN.UTF-8": Zh,
		"zh":          Zh,
		"":            Zh,
		"C":           Zh,
		"ja_JP.UTF-8": Zh,
		"chinese":     Zh,
		"english":     En, // lenient: any "en*" locale name counts as English
	}
	for locale, want := range cases {
		if got := Detect(locale); got != want {
			t.Errorf("Detect(%q) = %q, want %q", locale, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	good := map[string]Lang{
		"zh": Zh, "zh-CN": Zh, "zh_cn": Zh, "zh-hans": Zh, "chinese": Zh, "中文": Zh,
		"en": En, "en-US": En, "en_us": En, "en-GB": En, "en_gb": En, "english": En,
		" EN ": En,
	}
	for in, want := range good {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "de", "fr_FR", "zh_Hant", "en-GB-oed"} {
		if l, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) = %q, accepted; want rejected", bad, l)
		}
	}
}

func TestResolveFromEnvPrefersKilnLang(t *testing.T) {
	t.Setenv("KILN_LANG", "en")
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	t.Setenv("LC_MESSAGES", "zh_CN.UTF-8")
	t.Setenv("LANG", "zh_CN.UTF-8")
	if got := ResolveFromEnv(); got != En {
		t.Fatalf("KILN_LANG should win: got %q", got)
	}
}

func TestResolveFromEnvUsesLocaleOrder(t *testing.T) {
	t.Setenv("KILN_LANG", "")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	t.Setenv("LC_MESSAGES", "zh_CN.UTF-8")
	t.Setenv("LANG", "zh_CN.UTF-8")
	if got := ResolveFromEnv(); got != En {
		t.Fatalf("LC_ALL should win over LC_MESSAGES: got %q", got)
	}
	t.Setenv("LC_ALL", "")
	if got := ResolveFromEnv(); got != Zh {
		t.Fatalf("LC_MESSAGES should win over LANG: got %q", got)
	}
	t.Setenv("LC_MESSAGES", "")
	if got := ResolveFromEnv(); got != Zh {
		t.Fatalf("LANG zh should resolve to zh: got %q", got)
	}
	t.Setenv("LANG", "")
	if got := ResolveFromEnv(); got != Zh {
		t.Fatalf("empty environment should default to zh: got %q", got)
	}
}

func TestTTranslatesAndFallsBack(t *testing.T) {
	defer Set(Zh)
	Set(En)
	if got := T("<文本>"); got != "<text>" {
		t.Fatalf("T(<文本>) = %q", got)
	}
	if got := T("缺少 %s：问题没有起点动作", "ask"); got != "missing ask: the issue has no start action" {
		t.Fatalf("formatted T = %q", got)
	}
	if got := T("这条消息没有翻译"); got != "这条消息没有翻译" {
		t.Fatalf("missing key should fall back to the source, got %q", got)
	}
	Set(Zh)
	if got := T("<文本>"); got != "<文本>" {
		t.Fatalf("zh should return the source verbatim, got %q", got)
	}
	if got := T("缺少 %s：问题没有起点动作", "ask"); got != "缺少 ask：问题没有起点动作" {
		t.Fatalf("zh formatting = %q", got)
	}
}

func TestEKeepsTheWrapChain(t *testing.T) {
	defer Set(Zh)
	base := errors.New("disk full")
	Set(En)
	err := E("读取 %s: %w", "config", base)
	if !errors.Is(err, base) {
		t.Fatalf("E dropped the %%w chain: %v", err)
	}
	if want := "read config: disk full"; err.Error() != want {
		t.Fatalf("E = %q, want %q", err.Error(), want)
	}
}

var (
	cjk    = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)
	verbRE = regexp.MustCompile(`%[a-zA-Z]`)
)

func verbSequence(s string) string {
	return strings.Join(verbRE.FindAllString(strings.ReplaceAll(s, "%%", ""), -1), " ")
}

// Every English value must be free of CJK and keep the key's formatting verbs in
// the same order; otherwise Sprintf prints %!s(MISSING)-style noise at the user.
func TestEnglishCatalogIsTranslatedAndFormatSafe(t *testing.T) {
	for src, en := range enCatalog {
		if cjk.MatchString(en) {
			t.Errorf("english value still has CJK: %q -> %q", src, en)
		}
		if got, want := verbSequence(en), verbSequence(src); got != want {
			t.Errorf("verb mismatch for %q: key %q, value %q", src, want, got)
		}
	}
}
