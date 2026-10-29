package codes

import (
	"os"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Ortam değişkenleri (env sınıfı) komutlar gibi ad kodudur ve yalnız kimliğin adıyla (TR_LANG,
// TR_GPU_SYNC, TR_GPU_PROFILE, TR_JOB_DIR) okunur (dil tanımı 17.9). Bir gösterim dilinin adı
// (TR_DIL, TR_GPU_ESZAMANLI) kabul edilmez: tr komutu başlamadan durur ve kimliğin adını önerir
// (H8875). İletiler ve belgeler yazılacak kimlik adını gösterir.
// Yazım dilini seçen TR_env_LANG'ı dil listeleri kurulmadan önce messages okur (messages.LanguageEnvVars).

// EnvValue returns the value of the environment variable k (env class) and whether it is set, reading
// it under its command-line identity name (17.9).
func EnvValue(k Code) (string, bool) {
	v, set, _ := envLookup(k)
	return v, set
}

// ExportEnvironment gives every environment variable its identity name (TR_env_GPU_SYNC), which the
// packages that cannot see name codes read (compute); a variable not set under its command-line
// identity name loses an inherited identity name. A display-language name is an error suggesting
// the identity name (H8875, 17.9). The tr command calls it before anything else runs.
func ExportEnvironment() error {
	for _, k := range active.Codes() {
		if k.Namespace() != NamespaceEnvironment {
			continue
		}
		v, set, err := envLookup(k)
		if err != nil {
			return err
		}
		if set {
			os.Setenv(active.Name(k), v)
		} else {
			os.Unsetenv(active.Name(k))
		}
	}
	return nil
}

// envLookup k'yi kimlik adıyla (CommandLineName) okur; başka bir gömülü dilin adıyla verilmişse H8875 döner.
func envLookup(k Code) (value string, set bool, err error) {
	want := CommandLineName(k)
	for _, name := range WritingNames(k) {
		if name == want {
			continue
		}
		if _, ok := os.LookupEnv(name); ok {
			return "", false, messages.New(messages.H8875, name, NameRef(k))
		}
	}
	v, ok := os.LookupEnv(want)
	return v, ok, nil
}
