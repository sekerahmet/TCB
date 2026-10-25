package codes

import "os"

// Ortam değişkenleri (env sınıfı) komutlar gibi ad kodudur (dil tanımı 17.8): her gömülü yazım dilinin
// adıyla okunur (TR_GPU_ESZAMANLI ile TR_GPU_SYNC aynı değişkendir), iletiler ve belgeler kişinin
// dilindeki adı yazar. Karşılaştırma: Go (GODEBUG, GOMAXPROCS) ve PyTorch (CUDA_LAUNCH_BLOCKING,
// PYTORCH_*) ortam adlarını yalnızca İngilizce tutar; TR'de komut satırı adları her dilde kabul
// edildiği için ortam adları da aynı kuralı izler, betik hangi dilde yazılırsa yazılsın çalışır.
// Yazım dilini seçen TR_env_LANG'ı dil listeleri kurulmadan önce messages okur (messages.LanguageEnvVars).

// EnvValue returns the value of the environment variable k (env class) and whether it is set, reading
// it under its name in every embedded writing language; the source-language name comes first, so
// it wins when both are set.
func EnvValue(k Code) (string, bool) {
	for _, name := range envNames(k) {
		if v, ok := os.LookupEnv(name); ok {
			return v, true
		}
	}
	return "", false
}

// ExportEnvironment gives every environment variable set only under another language's name its
// source-language name too, so the packages that read the source name and cannot see name codes
// (compute: TR_GPU_ESZAMANLI, TR_CPU_AVX2 ...) see it. The tr command calls it before anything
// else runs.
func ExportEnvironment() {
	for _, k := range active.Codes() {
		if k.Namespace() != NamespaceEnvironment {
			continue
		}
		source := active.Name(k)
		if _, set := os.LookupEnv(source); set {
			continue
		}
		if v, ok := EnvValue(k); ok {
			os.Setenv(source, v)
		}
	}
}

// envNames k'nin bütün gömülü dillerdeki adlarıdır, kaynak dil önce, yinelenmeden.
func envNames(k Code) []string {
	names := []string{active.Name(k)}
	for _, lc := range Languages() {
		if lc == active.Code {
			continue
		}
		if l, err := LoadLanguage(lc); err == nil {
			if g, ok := l.Entry(k); ok && g.Name != names[0] {
				names = append(names, g.Name)
			}
		}
	}
	return names
}
