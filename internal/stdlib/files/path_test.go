package files

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestPathCleaned(t *testing.T) {
	cases := map[string]string{
		`yol("a/./b/../c")`:              "a/c",
		`yol("veri", "eğitim", "a.npy")`: "veri/eğitim/a.npy",
		`yol("veri/", "./a.npy")`:        "veri/a.npy",
		`yol(".")`:                       ".",
		`yol("a", "..")`:                 ".",
		`yol("..", "a")`:                 "../a",
		`yol(yol("veri"), "a")`:          "veri/a",
		`yol("a//b")`:                    "a/b",
	}
	if runtime.GOOS == "windows" {
		cases[`yol("veri\\alt", "a.txt")`] = "veri/alt/a.txt"
		cases[`yol("C:\\veri\\..\\x")`] = "C:/x"
	}
	for code, wants := range cases {
		if got := mustRunTR(t, code); got != wants {
			t.Errorf("%s = %q, %q bekleniyordu", code, got, wants)
		}
	}
}

func TestPathMembers(t *testing.T) {
	cases := map[string]string{
		`yol("veri/derlem.tar.gz").dosya_adı`:    "derlem.tar.gz",
		`yol("veri/derlem.tar.gz").uzantı`:       ".gz",
		`yol("veri/derlem.tar.gz").uzantısız_ad`: "derlem.tar",
		`yol(".gitignore").uzantı`:               "",
		`yol(".gitignore").uzantısız_ad`:         ".gitignore",
		`yol("a.").uzantı`:                       "",
		`yol("Makefile").uzantı`:                 "",
		`yol(".").dosya_adı`:                     "",
		`yol("a.txt").üst_klasör`:                ".",
		`yol("veri/eğitim/a.npy").üst_klasör`:    "veri/eğitim",
		`tip(yol("a").üst_klasör)`:               "Yol",
		`yol("a") == yol("a/./")`:                "doğru",
		`yol("a") == "a"`:                        "yanlış",
		`"{yol("x", "y")}"`:                      "x/y",
		`json_metin({"y": yol("a/b")})`:          `{"y":"a/b"}`,
	}
	for code, wants := range cases {
		if got := mustRunTR(t, code); got != wants {
			t.Errorf("%s = %q, %q bekleniyordu", code, got, wants)
		}
	}
	errContains(t, `yol("a").boyut`, "özelliği ya da metodu bulunamadı")
	errContains(t, `yol("a").dosya_adi`, "dosya_adı")
}

func TestPathErrors(t *testing.T) {
	errContains(t, `yol("a", "/etc")`, `yol() 2. parça "/etc" mutlak bir yol`)
	errHas(t, `yol("a", "")`, messages.H7138)
	errContains(t, `yol("")`, "1. parça boş metin")
	errContains(t, `yol()`, "yol()")
	errContains(t, `yol(5)`, "1. argüman olarak Metin ya da Yol bekler, Tam (5) verildi")
	errContains(t, `yol("C:\tokenizer.json")`, "sekme karakteri var", "ham metin")
	if runtime.GOOS == "windows" {
		errHas(t, `yol("a", "C:\\x")`, messages.H7139)
		errHas(t, `yol("a", "C:x")`, messages.H7139)
		errHas(t, `yol("a", "\\\\sunucu\\pay")`, messages.H7139)
	}
}

// Diske bakan özellikler yolu kullanıldığı anda 13.1'in tek kuralıyla çözer: önce çalışma
// klasörü, sonra programın klasörü.
func TestPathDiskQueries(t *testing.T) {
	work := tmpWork(t)
	program := t.TempDir()
	write(t, filepath.Join(program, "veri", "a.txt"), "merhaba")
	write(t, filepath.Join(work, "b.txt"), "xy")
	s := runTR(t, program, `[yol("veri/a.txt").var_mı, yol("veri/a.txt").dosya_mı, yol("veri").klasör_mü,
		yol("veri").dosya_mı, yol("yok").var_mı, yol("yok").klasör_mü, yol("veri/a.txt").bayt, yol("b.txt").bayt]`)
	if s.failure != nil || s.value.Inspect() != "[doğru, doğru, doğru, yanlış, yanlış, yanlış, 7, 2]" {
		t.Fatalf("%v %v", s.value, s.failure)
	}
	s = runTR(t, program, `yol("veri/a.txt").mutlak`)
	if s.failure != nil || !strings.EqualFold(s.value.Inspect(), forwardPath(filepath.Join(program, "veri", "a.txt"))) {
		t.Errorf("mutlak: %v %v", s.value, s.failure)
	}
	s = runTR(t, program, `yol("veri").bayt`)
	if s.failure == nil || !strings.Contains(s.failure.Error(), `yol.bayt: "veri" bir klasör`) {
		t.Errorf("klasörde bayt: %v", s.failure)
	}
	s = runTR(t, program, `yol("yok.txt").bayt`)
	if s.failure == nil || !strings.Contains(s.failure.Error(), "böyle bir dosya ya da klasör yok") || !strings.Contains(s.failure.Error(), "bakılan yerler") {
		t.Errorf("olmayan dosyada bayt: %v", s.failure)
	}
}

// Yol alan yerleşikler Yol da kabul eder (interp.PathValuer kancası).
func TestPathTakingBuiltins(t *testing.T) {
	tmpWork(t)
	got := mustRunTR(t, `dosya_yaz(yol("a.txt"), "1,2\n3,4")
		değişken t1 = dosya_oku(yol(".", "a.txt"))
		değişken t2 = csv_oku(yol("a.txt"))
		tensör_kaydet(yol("m.trt"), {"w": tensör([1.0, 2.0])})
		[t1, t2, tensör_yükle(yol("m.trt"))["w"].değer]`)
	if got != `["1,2\n3,4", [[1, 2], [3, 4]], [1.0, 2.0]]` {
		t.Fatalf("%s", got)
	}
	// Yol verilen csv_oku içeriği metin sanmaz: dosya yoksa hata verir.
	errContains(t, `csv_oku(yol("ad,yaş"))`, "csv_oku", "okunamadı")
	errContains(t, `dosya_oku(5)`, "dosya_oku() 1. argüman olarak Metin ya da Yol bekler")
}

// .gz saydamlığı: dosya_yaz sıkıştırır, dosya_oku, csv_oku ve dosya_satırları açar.
func TestGzipTransparent(t *testing.T) {
	k := tmpWork(t)
	got := mustRunTR(t, `dosya_yaz("t.csv.gz", "a,b\n1,2"); [dosya_oku("t.csv.gz"), csv_oku("t.csv.gz")]`)
	if got != `["a,b\n1,2", [["a", "b"], [1, 2]]]` {
		t.Fatalf("%s", got)
	}
	b, _ := os.ReadFile(filepath.Join(k, "t.csv.gz"))
	if len(b) < 2 || b[0] != 0x1f || b[1] != 0x8b {
		t.Fatalf("dosya sıkıştırılmamış: %q", b)
	}
	write(t, filepath.Join(k, "duz.gz"), "sıkıştırılmamış")
	errContains(t, `dosya_oku("duz.gz")`, `dosya_oku(): "duz.gz" okunamadı: uzantısı .gz ama içerik gzip değil`)
	errContains(t, `dosya_satırları("duz.gz")`, "uzantısı .gz ama içerik gzip değil")
	write(t, filepath.Join(k, "yarim.gz"), string(b[:len(b)-6]))
	errContains(t, `dosya_oku("yarim.gz")`, "gzip içeriği yarıda kesilmiş")
}
