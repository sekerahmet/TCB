package gpu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Matris çarpımı kitaplığının (cuBLASLt) arama sırası. Çalışma klasörü ve programın yanındaki
// klasör hiçbir zaman aranmaz (kitaplık kaçırma); adayların hepsi mutlak yoldur, Linux'ta
// ayrıca standart kitaplık arama yolu (yalnızca LD_LIBRARY_PATH mutlak klasörlerden
// oluşuyorsa) kullanılır:
//
//  1. TR_GPU_CUBLASLT (dosyanın tam yolu)
//  2. standart kitaplık arama yolu (yalnızca Linux)
//  3. bilinen CUDA kurulum klasörleri (/usr/local/cuda/lib64, CUDA_PATH, Windows'ta
//     Program Files\NVIDIA GPU Computing Toolkit\CUDA\v*)
//  4. Python'un pip ile kurduğu NVIDIA paketleri (nvidia/cublas/lib ya da bin); yalnızca
//     sabit, bilinen üst klasörlerde glob ile, Python çalıştırılmadan
//  5. TR'nin kendi önbellek klasörü (trOnbellekKlasoru; ileride kitaplığı oraya indirecek
//     bir komut için ayrılmıştır, şimdilik yalnızca aranır)

// carpimKitaplikAdlari kitaplığın sürüm sırasıyla dosya adları.
func carpimKitaplikAdlari() []string {
	if runtime.GOOS == "windows" {
		return []string{"cublasLt64_13.dll", "cublasLt64_12.dll"}
	}
	return []string{"libcublasLt.so.13", "libcublasLt.so.12"}
}

// trOnbellekKlasoru TR'nin GPU kitaplıkları için ayırdığı klasör (kullanıcı önbelleği
// altında tr/gpu/lib); kullanıcı önbelleği bilinmiyorsa boş.
func trOnbellekKlasoru() string {
	d, err := os.UserCacheDir()
	if err != nil || !filepath.IsAbs(d) {
		return ""
	}
	return filepath.Join(d, "tr", "gpu", "lib")
}

// globla deseni mutlak yollara açar; en yeni sürüm önce (ters sözlük sırası).
func globla(desen string) []string {
	if !filepath.IsAbs(desen) {
		return nil
	}
	r, _ := filepath.Glob(desen)
	sort.Sort(sort.Reverse(sort.StringSlice(r)))
	return r
}

// carpimKitaplikKlasorleri 3.–5. adımların klasörleri (sırayla).
func carpimKitaplikKlasorleri() []string {
	var k []string
	ekle := func(yollar ...string) {
		for _, y := range yollar {
			if filepath.IsAbs(y) {
				k = append(k, y)
			}
		}
	}
	cuda := os.Getenv("CUDA_PATH")
	if runtime.GOOS == "windows" {
		if filepath.IsAbs(cuda) {
			ekle(filepath.Join(cuda, "bin", "x64"), filepath.Join(cuda, "bin"))
		}
		if pf := os.Getenv("ProgramFiles"); filepath.IsAbs(pf) {
			for _, v := range globla(filepath.Join(pf, "NVIDIA GPU Computing Toolkit", "CUDA", "v*")) {
				ekle(filepath.Join(v, "bin", "x64"), filepath.Join(v, "bin"))
			}
		}
		for _, ust := range []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Python", "Python3*", "Lib", "site-packages"),
			filepath.Join(os.Getenv("APPDATA"), "Python", "Python3*", "site-packages"),
		} {
			for _, sp := range globla(ust) {
				ekle(filepath.Join(sp, "nvidia", "cublas", "bin"), filepath.Join(sp, "nvidia", "cu13", "bin"))
			}
		}
	} else {
		ekle("/usr/local/cuda/lib64", "/usr/local/cuda/targets/x86_64-linux/lib")
		if filepath.IsAbs(cuda) {
			ekle(filepath.Join(cuda, "lib64"))
		}
		ustler := []string{"/usr/local/lib/python3*/dist-packages", "/usr/local/lib/python3*/site-packages",
			"/usr/lib/python3/dist-packages", "/usr/lib/python3*/site-packages"}
		if h, err := os.UserHomeDir(); err == nil && filepath.IsAbs(h) {
			ustler = append(ustler, filepath.Join(h, ".local", "lib", "python3*", "site-packages"))
		}
		for _, ust := range ustler {
			for _, sp := range globla(ust) {
				ekle(filepath.Join(sp, "nvidia", "cublas", "lib"), filepath.Join(sp, "nvidia", "cu13", "lib"))
			}
		}
	}
	ekle(trOnbellekKlasoru())
	return k
}

// carpimKitapligiAc kitaplığı arama sırasıyla açar; bulunan yolu (Linux'ta standart arama
// yolundan bulunduysa dosya adını) da verir.
func carpimKitapligiAc() (kutuphane, string, error) {
	adlar := carpimKitaplikAdlari()
	var adaylar []string
	if p := os.Getenv("TR_GPU_CUBLASLT"); p != "" {
		if !filepath.IsAbs(p) {
			return nil, "", fmt.Errorf("TR_GPU_CUBLASLT tam yol olmalı: %q", p)
		}
		adaylar = append(adaylar, p)
	}
	if runtime.GOOS != "windows" && aramaYoluGuvenli(os.Getenv("LD_LIBRARY_PATH")) {
		adaylar = append(adaylar, adlar...) // standart arama yolu (dosya adıyla)
	}
	for _, k := range carpimKitaplikKlasorleri() {
		for _, ad := range adlar {
			adaylar = append(adaylar, filepath.Join(k, ad))
		}
	}
	var son error = errors.New("hiçbir arama yerinde yok")
	for _, a := range adaylar {
		if filepath.IsAbs(a) {
			if _, err := os.Stat(a); err != nil {
				continue
			}
		}
		k, err := kitaplikAc(a)
		if err == nil {
			return k, a, nil
		}
		son = err
	}
	return nil, "", son
}

// aramaYoluGuvenli LD_LIBRARY_PATH'in yalnızca mutlak klasörlerden oluştuğunu denetler
// (boş öğe ya da göreli yol çalışma klasörünü aratırdı).
func aramaYoluGuvenli(yol string) bool {
	if yol == "" {
		return true
	}
	for _, p := range strings.Split(yol, ":") {
		if !strings.HasPrefix(p, "/") { // Linux yolu (ayraç ':')
			return false
		}
	}
	return true
}
