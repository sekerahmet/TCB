package files

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Silme korumaları (sil, taşı'nın kaynağı ve üzerine_yaz ile yerine konan klasör): kök ve
// sürücü kökleri, ev klasörü ve üstleri (başka kullanıcıların ev klasörlerini içerir),
// işletim sisteminin bilinen klasörleri ve üstleri, çalışma klasörü ve üstleri, kodun
// modülünün ve programın klasörü ve üstleri silinmez. Bir depoda (proje klasöründe) çalışan programın depo
// kökü, programın klasörünün üstü olduğu için bu kurala girer. Karşılaştırma bağlantıları
// çözülmüş mutlak yollarla yapılır (yalnızca silinecek yolun üst klasörleri çözülür; yolun
// kendisi bir bağlantıysa bağlantının kendisi silinir, gösterdiği yer değil). Windows ve
// macOS'ta harf büyüklüğü fark etmez.

// isLink girdi bir bağlantı mı (sembolik bağlantı, Windows dizin bağlantısı ya da başka
// bir yeniden ayrıştırma noktası)? Bağlantılar hiçbir işte izlenmez.
func isLink(mode fs.FileMode) bool { return mode&(fs.ModeSymlink|fs.ModeIrregular) != 0 }

// realPath yolun bağlantıları çözülmüş mutlak biçimidir; yolun kendisi değil, yalnızca bulunduğu
// klasör çözülür (bağlantının kendisi korunur).
func realPath(fpath string) string {
	m, err := filepath.Abs(fpath)
	if err != nil {
		return fpath
	}
	parent, name := filepath.Dir(m), filepath.Base(m)
	if name == string(filepath.Separator) || parent == m {
		return m
	}
	if g, err := filepath.EvalSymlinks(parent); err == nil {
		return filepath.Join(g, name)
	}
	return m
}

// resolvedDir bir klasörün bağlantıları çözülmüş mutlak yoludur (çözülemezse mutlak yolu).
func resolvedDir(fpath string) string {
	m, err := filepath.Abs(fpath)
	if err != nil {
		return fpath
	}
	if g, err := filepath.EvalSymlinks(m); err == nil {
		return g
	}
	return m
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// isInside a, b'nin kendisi ya da altında mı?
func isInside(a, b string) bool {
	if samePath(a, b) {
		return true
	}
	extra := b
	if !strings.HasSuffix(extra, string(filepath.Separator)) {
		extra += string(filepath.Separator)
	}
	return len(a) > len(extra) && samePath(a[:len(extra)], extra)
}

// systemDirs işletim sisteminin bilinen klasörleridir (ev klasörünün altında
// olmayanlar); kendileri ve üstleri silinmez. Windows'ta ortam değişkenlerinden okunur
// (tanımsız olanlar atlanır).
func systemDirs() []string {
	if runtime.GOOS != "windows" {
		return []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib64", "/proc", "/sbin", "/sys", "/usr", "/var",
			"/System", "/Library", "/Applications"}
	}
	var out []string
	for _, name := range []string{"SystemRoot", "ProgramFiles", "ProgramFiles(x86)", "ProgramData"} {
		if k := os.Getenv(name); k != "" && filepath.IsAbs(k) {
			out = append(out, k)
		}
	}
	return out
}

// deleteProtection mutlak (realPath ile bulunmuş) yol silinmemesi gereken bir yerse nedenini
// verir; silinebilirse boş döner.
func deleteProtection(m *interp.VM, fpath string) string {
	if filepath.Dir(fpath) == fpath {
		return "bir kök klasör (sürücünün ya da dosya sisteminin kökü)"
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && isInside(resolvedDir(home), fpath) {
		if samePath(fpath, resolvedDir(home)) {
			return "ev klasörünüzün kendisi"
		}
		return "ev klasörünüzün bir üst klasörü (başka kullanıcıların ev klasörlerini de içerebilir)"
	}
	for _, k := range systemDirs() {
		if isInside(resolvedDir(k), fpath) {
			return "işletim sisteminin bir klasörü ya da onun bir üst klasörü"
		}
	}
	if cw, err := os.Getwd(); err == nil && isInside(resolvedDir(cw), fpath) {
		return "çalışma klasörü ya da onun bir üst klasörü"
	}
	for _, k := range m.CodeDirs() {
		if isInside(resolvedDir(k), fpath) {
			return "programın (ya da modülün) klasörü ya da onun bir üst klasörü"
		}
	}
	return ""
}
