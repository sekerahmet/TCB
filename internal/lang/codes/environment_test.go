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

// Kural (dil tanımı 17.8): bir ortam değişkeni her gömülü yazım dilinin adıyla okunur; hiçbir dilin adı
// öncelikli değildir: iki ad aynı değerle verilebilir, ayrı değerle verilirse hata (H2527).
func TestEnvValueReadsEveryLanguagesName(t *testing.T) {
	unsetEnv(t, "TR_GPU_ESZAMANLI")
	unsetEnv(t, "TR_GPU_SYNC")
	if _, ok := EnvValue(EnvGpuSync); ok {
		t.Fatal("unset variable reported as set")
	}
	for _, name := range []string{"TR_GPU_SYNC", "TR_GPU_ESZAMANLI"} {
		t.Run(name, func(t *testing.T) {
			unsetEnv(t, "TR_GPU_ESZAMANLI")
			unsetEnv(t, "TR_GPU_SYNC")
			t.Setenv(name, "1")
			if v, ok := EnvValue(EnvGpuSync); !ok || v != "1" {
				t.Errorf("%s=1 not read: %q %v", name, v, ok)
			}
		})
	}
	t.Setenv("TR_GPU_SYNC", "1")
	t.Setenv("TR_GPU_ESZAMANLI", "1")
	if _, _, err := envLookup(EnvGpuSync); err != nil {
		t.Errorf("same value under both names: %v", err)
	}
	t.Setenv("TR_GPU_ESZAMANLI", "0")
	if _, _, err := envLookup(EnvGpuSync); !messages.Has(err, messages.H2527) {
		t.Errorf("different values under two names: want H2527, got %v", err)
	}
	if err := ExportEnvironment(); !messages.Has(err, messages.H2527) {
		t.Errorf("ExportEnvironment: want H2527, got %v", err)
	}
}

// Kural: tr komutu başlarken her ortam değişkenine kimlik adı verilir (ad kodlarını göremeyen hesap
// çekirdeği onu okur: TR_env_GPU_SYNC); hiçbir yazım dilinde verilmeyen değişkenin kalıtılmış kimlik adı
// silinir. Ortak adlı değişken (TR_CPU_AVX2) ortak adıyla da okunur.
func TestExportEnvironmentGivesTheIdentityName(t *testing.T) {
	for _, name := range []string{"TR_GPU_SYNC", "TR_GPU_ESZAMANLI", "TR_GPU_SYNC", "TR_CPU_AVX2", "TR_CPU_AVX2"} {
		unsetEnv(t, name)
	}
	for _, name := range []string{"TR_GPU_SYNC", "TR_GPU_ESZAMANLI"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "1")
			t.Setenv("TR_CPU_AVX2", "0")
			if err := ExportEnvironment(); err != nil {
				t.Fatal(err)
			}
			if v := os.Getenv("TR_GPU_SYNC"); v != "1" {
				t.Errorf("TR_GPU_SYNC = %q, want 1", v)
			}
			if v := os.Getenv("TR_CPU_AVX2"); v != "0" {
				t.Errorf("TR_CPU_AVX2 = %q, want 0", v)
			}
			os.Unsetenv(name)
			if err := ExportEnvironment(); err != nil {
				t.Fatal(err)
			}
			if v, set := os.LookupEnv("TR_GPU_SYNC"); set {
				t.Errorf("TR_GPU_SYNC = %q left after its name was removed", v)
			}
		})
	}
}

// messages yazım dilini dil listelerinden önce okuduğu için TR_env_LANG'ün adlarını kendisi tutar; liste
// TR_env_LANG'ün bütün gömülü dillerdeki adlarıyla aynı olmalı (yeni dil eklenince burada yakalanır).
func TestLanguageEnvVarsAreK400(t *testing.T) {
	got, want := slices.Sorted(slices.Values(messages.LanguageEnvVars)), WritingNames(EnvLang)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("messages.LanguageEnvVars = %v, TR_LANG names = %v", got, want)
	}
	if !slices.Contains(want, messages.LanguageEnvVar) {
		t.Errorf("messages.LanguageEnvVar = %q is not a name of TR_LANG %v", messages.LanguageEnvVar, want)
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
