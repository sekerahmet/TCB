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

// Kural (dil tanımı 17.9): bir ortam değişkeni yalnız kimliğin adıyla okunur (TR_GPU_SYNC, TR_LANG);
// gösterim dilindeki ad (TR_GPU_ESZAMANLI, TR_DIL) kabul edilmez ve kimliğin adını önerir (H8875).
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
	os.Unsetenv("TR_GPU_SYNC")
	t.Setenv("TR_GPU_ESZAMANLI", "1")
	if v, ok := EnvValue(EnvGpuSync); ok {
		t.Errorf("display-language name TR_GPU_ESZAMANLI must not be read: %q", v)
	}
	if _, _, err := envLookup(EnvGpuSync); !messages.Has(err, messages.H8875) || !strings.Contains(err.Error(), "TR_GPU_SYNC") {
		t.Errorf("display-language name TR_GPU_ESZAMANLI: want H8875 suggesting TR_GPU_SYNC, got %v", err)
	}
	if err := ExportEnvironment(); !messages.Has(err, messages.H8875) || !strings.Contains(err.Error(), "TR_GPU_SYNC") {
		t.Errorf("ExportEnvironment: want H8875 suggesting TR_GPU_SYNC, got %v", err)
	}
}

// Kural (17.9): tr komutu başlarken her ortam değişkenine kimlik adı verilir (ad kodlarını göremeyen hesap
// çekirdeği onu okur: TR_env_GPU_SYNC); verilmeyen değişkenin kalıtılmış kimlik adı silinir.
func TestExportEnvironmentGivesTheIdentityName(t *testing.T) {
	for _, name := range []string{"TR_GPU_SYNC", "TR_GPU_ESZAMANLI", "TR_env_GPU_SYNC", "TR_CPU_AVX2", "TR_env_CPU_AVX2"} {
		unsetEnv(t, name)
	}
	t.Setenv("TR_GPU_SYNC", "1")
	t.Setenv("TR_CPU_AVX2", "0")
	if err := ExportEnvironment(); err != nil {
		t.Fatal(err)
	}
	if v := os.Getenv("TR_env_GPU_SYNC"); v != "1" {
		t.Errorf("TR_env_GPU_SYNC = %q, want 1", v)
	}
	if v := os.Getenv("TR_env_CPU_AVX2"); v != "0" {
		t.Errorf("TR_env_CPU_AVX2 = %q, want 0", v)
	}
	os.Unsetenv("TR_GPU_SYNC")
	if err := ExportEnvironment(); err != nil {
		t.Fatal(err)
	}
	if v, set := os.LookupEnv("TR_env_GPU_SYNC"); set {
		t.Errorf("TR_env_GPU_SYNC = %q left after its name was removed", v)
	}
}

// messages gösterim dilini dil listelerinden önce okuduğu için TR_env_LANG'in komut satırı adını
// kendisi tutar; yalnız kimlik adını (TR_LANG) okur (17.8, 17.9).
func TestLanguageEnvVarsAreK400(t *testing.T) {
	want := []string{CommandLineName(EnvLang)}
	if !slices.Equal(messages.LanguageEnvVars, want) {
		t.Errorf("messages.LanguageEnvVars = %v, want %v", messages.LanguageEnvVars, want)
	}
	if messages.LanguageEnvVar != CommandLineName(EnvLang) {
		t.Errorf("messages.LanguageEnvVar = %q, want %q", messages.LanguageEnvVar, CommandLineName(EnvLang))
	}
}

// Hesap çekirdeğinin iletileri ortam adını ad kodundan alamaz (katman kuralı): ad her dilin
// şablonunda o dilin adıyla yazılır. Şablon ile ad kodu ayrışırsa burada yakalanır.
func TestComputeMessagesNameTheirLanguagesVariable(t *testing.T) {
	uses := map[string]Code{"H4009": EnvGpuSync, "H4026": EnvGpuCublaslt, "H4027": EnvGpuCublaslt}
	for _, lc := range Languages() {
		if _, err := LoadLanguage(lc); err != nil {
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
			want := CommandLineName(k)
			if !strings.Contains(texts[code], want) {
				t.Errorf("%s.json %s: %q does not name %s", lc, code, texts[code], want)
			}
		}
	}
}
