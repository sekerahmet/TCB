package dosya

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"tr_dili/internal/dil/yurut"
)

// Silme korumaları (sil, taşı'nın kaynağı ve üzerine_yaz ile yerine konan klasör): kök ve
// sürücü kökleri, ev klasörü ve üstleri (başka kullanıcıların ev klasörlerini içerir),
// işletim sisteminin bilinen klasörleri ve üstleri, çalışma klasörü ve üstleri, kodun
// modülünün ve programın klasörü ve üstleri silinmez. Bir depoda (proje klasöründe) çalışan programın depo
// kökü, programın klasörünün üstü olduğu için bu kurala girer. Karşılaştırma bağlantıları
// çözülmüş mutlak yollarla yapılır (yalnızca silinecek yolun üst klasörleri çözülür; yolun
// kendisi bir bağlantıysa bağlantının kendisi silinir, gösterdiği yer değil). Windows ve
// macOS'ta harf büyüklüğü fark etmez.

// baglantiMi girdi bir bağlantı mı (sembolik bağlantı, Windows dizin bağlantısı ya da başka
// bir yeniden ayrıştırma noktası)? Bağlantılar hiçbir işte izlenmez.
func baglantiMi(mod fs.FileMode) bool { return mod&(fs.ModeSymlink|fs.ModeIrregular) != 0 }

// gercekYol yolun bağlantıları çözülmüş mutlak biçimidir; yolun kendisi değil, yalnızca bulunduğu
// klasör çözülür (bağlantının kendisi korunur).
func gercekYol(yol string) string {
	m, err := filepath.Abs(yol)
	if err != nil {
		return yol
	}
	ust, ad := filepath.Dir(m), filepath.Base(m)
	if ad == string(filepath.Separator) || ust == m {
		return m
	}
	if g, err := filepath.EvalSymlinks(ust); err == nil {
		return filepath.Join(g, ad)
	}
	return m
}

// cozulmusKlasor bir klasörün bağlantıları çözülmüş mutlak yoludur (çözülemezse mutlak yolu).
func cozulmusKlasor(yol string) string {
	m, err := filepath.Abs(yol)
	if err != nil {
		return yol
	}
	if g, err := filepath.EvalSymlinks(m); err == nil {
		return g
	}
	return m
}

func ayniYol(a, b string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// icindeMi a, b'nin kendisi ya da altında mı?
func icindeMi(a, b string) bool {
	if ayniYol(a, b) {
		return true
	}
	ek := b
	if !strings.HasSuffix(ek, string(filepath.Separator)) {
		ek += string(filepath.Separator)
	}
	return len(a) > len(ek) && ayniYol(a[:len(ek)], ek)
}

// sistemKlasorleri işletim sisteminin bilinen klasörleridir (ev klasörünün altında
// olmayanlar); kendileri ve üstleri silinmez. Windows'ta ortam değişkenlerinden okunur
// (tanımsız olanlar atlanır).
func sistemKlasorleri() []string {
	if runtime.GOOS != "windows" {
		return []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib64", "/proc", "/sbin", "/sys", "/usr", "/var",
			"/System", "/Library", "/Applications"}
	}
	var out []string
	for _, ad := range []string{"SystemRoot", "ProgramFiles", "ProgramFiles(x86)", "ProgramData"} {
		if k := os.Getenv(ad); k != "" && filepath.IsAbs(k) {
			out = append(out, k)
		}
	}
	return out
}

// silmeKorumasi mutlak (gercekYol ile bulunmuş) yol silinmemesi gereken bir yerse nedenini
// verir; silinebilirse boş döner.
func silmeKorumasi(m *yurut.VM, yol string) string {
	if filepath.Dir(yol) == yol {
		return "bir kök klasör (sürücünün ya da dosya sisteminin kökü)"
	}
	if ev, err := os.UserHomeDir(); err == nil && ev != "" && icindeMi(cozulmusKlasor(ev), yol) {
		if ayniYol(yol, cozulmusKlasor(ev)) {
			return "ev klasörünüzün kendisi"
		}
		return "ev klasörünüzün bir üst klasörü (başka kullanıcıların ev klasörlerini de içerebilir)"
	}
	for _, k := range sistemKlasorleri() {
		if icindeMi(cozulmusKlasor(k), yol) {
			return "işletim sisteminin bir klasörü ya da onun bir üst klasörü"
		}
	}
	if cw, err := os.Getwd(); err == nil && icindeMi(cozulmusKlasor(cw), yol) {
		return "çalışma klasörü ya da onun bir üst klasörü"
	}
	for _, k := range m.KodKlasorleri() {
		if icindeMi(cozulmusKlasor(k), yol) {
			return "programın (ya da modülün) klasörü ya da onun bir üst klasörü"
		}
	}
	return ""
}
