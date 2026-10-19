package gpu

import (
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

// gemmLibNames kitaplığın sürüm sırasıyla dosya adları.
func gemmLibNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"cublasLt64_13.dll", "cublasLt64_12.dll"}
	}
	return []string{"libcublasLt.so.13", "libcublasLt.so.12"}
}

// trCacheDir TR'nin GPU kitaplıkları için ayırdığı klasör (kullanıcı önbelleği
// altında tr/gpu/lib); kullanıcı önbelleği bilinmiyorsa boş.
func trCacheDir() string {
	d, err := os.UserCacheDir()
	if err != nil || !filepath.IsAbs(d) {
		return ""
	}
	return filepath.Join(d, "tr", "gpu", "lib")
}

// toGlobal deseni mutlak yollara açar; en yeni sürüm önce (ters sözlük sırası).
func toGlobal(pattern string) []string {
	if !filepath.IsAbs(pattern) {
		return nil
	}
	r, _ := filepath.Glob(pattern)
	sort.Sort(sort.Reverse(sort.StringSlice(r)))
	return r
}

// gemmLibDirs 3.–5. adımların klasörleri (sırayla).
func gemmLibDirs() []string {
	var k []string
	add := func(paths ...string) {
		for _, y := range paths {
			if filepath.IsAbs(y) {
				k = append(k, y)
			}
		}
	}
	cuda := os.Getenv("CUDA_PATH")
	if runtime.GOOS == "windows" {
		if filepath.IsAbs(cuda) {
			add(filepath.Join(cuda, "bin", "x64"), filepath.Join(cuda, "bin"))
		}
		if pf := os.Getenv("ProgramFiles"); filepath.IsAbs(pf) {
			for _, v := range toGlobal(filepath.Join(pf, "NVIDIA GPU Computing Toolkit", "CUDA", "v*")) {
				add(filepath.Join(v, "bin", "x64"), filepath.Join(v, "bin"))
			}
		}
		for _, upper := range []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Python", "Python3*", "Lib", "site-packages"),
			filepath.Join(os.Getenv("APPDATA"), "Python", "Python3*", "site-packages"),
		} {
			for _, sp := range toGlobal(upper) {
				add(filepath.Join(sp, "nvidia", "cublas", "bin"), filepath.Join(sp, "nvidia", "cu13", "bin"))
			}
		}
	} else {
		add("/usr/local/cuda/lib64", "/usr/local/cuda/targets/x86_64-linux/lib")
		if filepath.IsAbs(cuda) {
			add(filepath.Join(cuda, "lib64"))
		}
		exponents := []string{"/usr/local/lib/python3*/dist-packages", "/usr/local/lib/python3*/site-packages",
			"/usr/lib/python3/dist-packages", "/usr/lib/python3*/site-packages"}
		if h, err := os.UserHomeDir(); err == nil && filepath.IsAbs(h) {
			exponents = append(exponents, filepath.Join(h, ".local", "lib", "python3*", "site-packages"))
		}
		for _, upper := range exponents {
			for _, sp := range toGlobal(upper) {
				add(filepath.Join(sp, "nvidia", "cublas", "lib"), filepath.Join(sp, "nvidia", "cu13", "lib"))
			}
		}
	}
	add(trCacheDir())
	return k
}

// gemmLibCandidates aranacak yerleri verir: açıkça verilen yol (TR_GPU_CUBLASLT; tam yol
// değilse hata) ve öbür adaylar (Linux'ta standart arama yolundakiler dosya adıyla). Var
// olmayan mutlak yollar atlanır, aynı dosya (bağlar çözülerek) bir kez yer alır. Adaylar
// arasından seçim sürüme göre yapılır (bkz. ltAdaylariAc).
func gemmLibCandidates() (open string, candidates []string, err error) {
	if p := os.Getenv("TR_GPU_CUBLASLT"); p != "" {
		if !filepath.IsAbs(p) {
			return "", nil, fmt.Errorf("TR_GPU_CUBLASLT tam yol olmalı: %q", p)
		}
		open = p
	}
	names := gemmLibNames()
	if runtime.GOOS != "windows" && searchPathSafe(os.Getenv("LD_LIBRARY_PATH")) {
		candidates = append(candidates, names...) // standart arama yolu (dosya adıyla)
	}
	wasSeen := map[string]bool{}
	for _, k := range gemmLibDirs() {
		for _, ad := range names {
			path := filepath.Join(k, ad)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			if actual, err := filepath.EvalSymlinks(path); err == nil {
				if wasSeen[actual] {
					continue
				}
				wasSeen[actual] = true
			}
			candidates = append(candidates, path)
		}
	}
	return open, candidates, nil
}

// driverDirs NVIDIA sürücüsünün libcuda'yı koyduğu bilinen Linux klasörleridir (dağıtımlar,
// Colab ve kapsayıcı kalıpları, WSL).
var driverDirs = []string{
	"/usr/lib/x86_64-linux-gnu", "/usr/lib/aarch64-linux-gnu", "/usr/lib64", "/usr/lib",
	"/usr/lib64-nvidia", "/usr/local/nvidia/lib64", "/usr/lib/wsl/lib",
}

// driverLibCandidates sürücü kitaplığı için denenecek adları verir: LD_LIBRARY_PATH güvenliyse
// dosya adları (standart arama yolu), değilse yalnızca bilinen sürücü klasörlerindeki tam
// yollar (boş öğe ya da göreli yol çalışma klasöründeki sahte bir libcuda'yı yükletirdi).
func driverLibCandidates(ldPath string) []string {
	names := []string{"libcuda.so.1", "libcuda.so"}
	if searchPathSafe(ldPath) {
		return names
	}
	var paths []string
	for _, d := range driverDirs {
		for _, n := range names {
			paths = append(paths, d+"/"+n)
		}
	}
	return paths
}

// searchPathSafe LD_LIBRARY_PATH'in yalnızca mutlak klasörlerden oluştuğunu denetler
// (boş öğe ya da göreli yol çalışma klasörünü aratırdı).
func searchPathSafe(path string) bool {
	if path == "" {
		return true
	}
	// Yükleyici ':' ve ';' ayraçlarının ikisini de kabul eder.
	for _, p := range strings.Split(strings.ReplaceAll(path, ";", ":"), ":") {
		if !strings.HasPrefix(p, "/") { // Linux yolu
			return false
		}
	}
	return true
}
