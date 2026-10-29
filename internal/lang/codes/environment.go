package codes

import (
	"os"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Ortam değişkenleri (env sınıfı) komutlar gibi ad kodudur (dil tanımı 17.8): her gömülü yazım dilinin
// adıyla okunur (TR_GPU_SYNC ile TR_GPU_SYNC aynı değişkendir), hiçbir dilin adı öncelikli değildir:
// iki ad ayrı değerle verilirse tr komutu başlamadan durur (H2527). İletiler ve belgeler kişinin dilindeki
// adı yazar. Karşılaştırma: Go (GODEBUG, GOMAXPROCS) ve PyTorch (CUDA_LAUNCH_BLOCKING, PYTORCH_*) ortam
// adlarını yalnızca İngilizce tutar; TR'de komut satırı adları her dilde kabul edildiği için ortam adları da
// aynı kuralı izler, betik hangi dilde yazılırsa yazılsın çalışır.
// Yazım dilini seçen TR_env_LANG'ı dil listeleri kurulmadan önce messages okur (messages.LanguageEnvVars).

// EnvValue returns the value of the environment variable k (env class) and whether it is set, reading
// it under its name in every embedded writing language (ExportEnvironment rejects two names with
// different values).
func EnvValue(k Code) (string, bool) {
	v, set, _ := envLookup(k)
	return v, set
}

// ExportEnvironment gives every environment variable its identity name (TR_env_GPU_SYNC), which the
// packages that cannot see name codes read (compute); a variable set under no writing-language name
// loses an inherited identity name. Two names of one variable with different values are an error
// (H2527). The tr command calls it before anything else runs.
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

// envLookup k'yi her yazım dilinin adıyla okur; iki ad ayrı değerle verilmişse H2527 döner.
func envLookup(k Code) (value string, set bool, err error) {
	var first string
	for _, name := range WritingNames(k) {
		v, ok := os.LookupEnv(name)
		switch {
		case !ok:
		case !set:
			value, set, first = v, true, name
		case v != value:
			return "", false, messages.New(messages.H2527, first, value, name, v)
		}
	}
	return value, set, nil
}
