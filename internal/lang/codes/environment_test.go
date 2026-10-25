package codes

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// unsetEnv değişkeni test boyunca siler, sonunda eski hâline getirir.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	old, had := os.LookupEnv(name)
	os.Unsetenv(name)
	t.Cleanup(func() {
		if had {
			os.Setenv(name, old)
		} else {
			os.Unsetenv(name)
		}
	})
}

// Kural (dil tanımı 17.8): bir ortam değişkeni her gömülü yazım dilinin adıyla okunur; ikisi de
// verilirse kaynak dildeki ad kazanır.
func TestEnvValueReadsEveryLanguagesName(t *testing.T) {
	unsetEnv(t, "TR_GPU_ESZAMANLI")
	unsetEnv(t, "TR_GPU_SYNC")
	if _, ok := EnvValue(EnvGpuSync); ok {
		t.Fatal("unset variable reported as set")
	}
	t.Setenv("TR_GPU_SYNC", "1")
	if v, ok := EnvValue(EnvGpuSync); !ok || v != "1" {
		t.Errorf("TR_GPU_SYNC=1 not read: %q %v", v, ok)
	}
	t.Setenv("TR_GPU_ESZAMANLI", "0")
	if v, _ := EnvValue(EnvGpuSync); v != "0" {
		t.Errorf("source-language name should win, got %q", v)
	}
}

// Kural: tr komutu başlarken başka bir dilin adıyla verilen değişkene kaynak dildeki adı da verilir
// (ad kodlarını göremeyen hesap çekirdeği onu okur); kaynak adla verilen değer korunur.
func TestExportEnvironmentGivesTheSourceName(t *testing.T) {
	unsetEnv(t, "TR_GPU_ESZAMANLI")
	unsetEnv(t, "TR_CPU_AVX2")
	t.Setenv("TR_GPU_SYNC", "1")
	t.Setenv("TR_CPU_AVX2", "0")
	ExportEnvironment()
	if v := os.Getenv("TR_GPU_ESZAMANLI"); v != "1" {
		t.Errorf("TR_GPU_ESZAMANLI = %q, want 1", v)
	}
	if v := os.Getenv("TR_CPU_AVX2"); v != "0" {
		t.Errorf("TR_CPU_AVX2 = %q, want 0", v)
	}
}

// messages yazım dilini dil listelerinden önce okuduğu için TR_env_LANG'ün adlarını kendisi tutar; liste
// TR_env_LANG'ün bütün gömülü dillerdeki adlarıyla aynı olmalı (yeni dil eklenince burada yakalanır).
func TestLanguageEnvVarsAreK400(t *testing.T) {
	if got, want := messages.LanguageEnvVars, envNames(EnvLang); !slices.Equal(got, want) {
		t.Errorf("messages.LanguageEnvVars = %v, TR_env_LANG names = %v", got, want)
	}
	if messages.LanguageEnvVar != active.Name(EnvLang) {
		t.Errorf("messages.LanguageEnvVar = %q, TR_env_LANG = %q", messages.LanguageEnvVar, active.Name(EnvLang))
	}
}

// Hesap çekirdeğinin iletileri ortam adını ad kodundan alamaz (katman kuralı): ad her dilin
// şablonunda o dilin adıyla yazılır. Şablon ile ad kodu ayrışırsa burada yakalanır.
func TestComputeMessagesNameTheirLanguagesVariable(t *testing.T) {
	uses := map[string]Code{"H4009": EnvGpuSync, "H4026": EnvGpuCublaslt, "H4027": EnvGpuCublaslt}
	for _, lc := range Languages() {
		l, err := LoadLanguage(lc)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(reporoot.Path("internal/lang/messages/" + lc + ".json"))
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(data, &texts); err != nil {
			t.Fatal(err)
		}
		for code, k := range uses {
			g, _ := l.Entry(k)
			if !strings.Contains(texts[code], g.Name) {
				t.Errorf("%s.json %s: %q does not name %s", lc, code, texts[code], g.Name)
			}
		}
	}
}
