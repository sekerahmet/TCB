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
		`TR_path("a/./b/../c")`:              "a/c",
		`TR_path("veri", "eğitim", "a.npy")`: "veri/eğitim/a.npy",
		`TR_path("veri/", "./a.npy")`:        "veri/a.npy",
		`TR_path(".")`:                       ".",
		`TR_path("a", "..")`:                 ".",
		`TR_path("..", "a")`:                 "../a",
		`TR_path(TR_path("veri"), "a")`:      "veri/a",
		`TR_path("a//b")`:                    "a/b",
	}
	if runtime.GOOS == "windows" {
		cases[`TR_path("veri\\alt", "a.txt")`] = "veri/alt/a.txt"
		cases[`TR_path("C:\\veri\\..\\x")`] = "C:/x"
	}
	for code, wants := range cases {
		if got := mustRunTR(t, code); got != wants {
			t.Errorf("%s = %q, %q bekleniyordu", code, got, wants)
		}
	}
}

func TestPathMembers(t *testing.T) {
	cases := map[string]string{
		`TR_path("veri/derlem.tar.gz").TR_name`:      "derlem.tar.gz",
		`TR_path("veri/derlem.tar.gz").TR_extension`: ".gz",
		`TR_path("veri/derlem.tar.gz").TR_stem`:      "derlem.tar",
		`TR_path(".gitignore").TR_extension`:         "",
		`TR_path(".gitignore").TR_stem`:              ".gitignore",
		`TR_path("a.").TR_extension`:                 "",
		`TR_path("Makefile").TR_extension`:           "",
		`TR_path(".").TR_name`:                       "",
		`TR_path("a.txt").TR_parent`:                 ".",
		`TR_path("veri/eğitim/a.npy").TR_parent`:     "veri/eğitim",
		`TR_type(TR_path("a").TR_parent)`:            "Yol",
		`TR_path("a") == TR_path("a/./")`:            "doğru",
		`TR_path("a") == "a"`:                        "yanlış",
		`"\(TR_path("x", "y"))"`:                     "x/y",
		`TR_json_string({"y": TR_path("a/b")})`:      `{"y":"a/b"}`,
	}
	for code, wants := range cases {
		if got := mustRunTR(t, code); got != wants {
			t.Errorf("%s = %q, %q bekleniyordu", code, got, wants)
		}
	}
	errContains(t, `TR_path("a").TR_embed_dim`, "özelliği ya da metodu bulunamadı")
	errContains(t, `TR_path("a").dosya_adi`, "dosya_adı")
}

func TestPathErrors(t *testing.T) {
	errContains(t, `TR_path("a", "/etc")`, `TR_path() 2. parça "/etc" TR_abs bir TR_path`)
	errHas(t, `TR_path("a", "")`, messages.H7138)
	errContains(t, `TR_path("")`, "1. parça boş TR_str")
	errContains(t, `TR_path()`, "TR_path()")
	errContains(t, `TR_path(5)`, "1. argüman olarak Metin ya da Yol bekler, Tam (5) verildi")
	errContains(t, `TR_path("C:\tokenizer.json")`, "sekme karakteri var", "ham metin")
	if runtime.GOOS == "windows" {
		errHas(t, `TR_path("a", "C:\\x")`, messages.H7139)
		errHas(t, `TR_path("a", "C:x")`, messages.H7139)
		errHas(t, `TR_path("a", "\\\\sunucu\\pay")`, messages.H7139)
	}
}

// Diske bakan özellikler yolu kullanıldığı anda 13.1'in tek kuralıyla çözer: önce çalışma
// klasörü, sonra programın klasörü.
func TestPathDiskQueries(t *testing.T) {
	work := tmpWork(t)
	program := t.TempDir()
	write(t, filepath.Join(program, "veri", "a.txt"), "merhaba")
	write(t, filepath.Join(work, "b.txt"), "xy")
	s := runTR(t, program, `[TR_path("veri/a.txt").TR_exists, TR_path("veri/a.txt").TR_is_file, TR_path("veri").TR_is_dir,
		TR_path("veri").TR_is_file, TR_path("yok").TR_exists, TR_path("yok").TR_is_dir, TR_path("veri/a.txt").TR_size, TR_path("b.txt").TR_size]`)
	if s.failure != nil || s.value.Inspect() != "[doğru, doğru, doğru, yanlış, yanlış, yanlış, 7, 2]" {
		t.Fatalf("%v %v", s.value, s.failure)
	}
	s = runTR(t, program, `TR_path("veri/a.txt").TR_absolute`)
	if s.failure != nil || !strings.EqualFold(s.value.Inspect(), forwardPath(filepath.Join(program, "veri", "a.txt"))) {
		t.Errorf("mutlak: %v %v", s.value, s.failure)
	}
	s = runTR(t, program, `TR_path("veri").TR_size`)
	if s.failure == nil || !strings.Contains(s.failure.Error(), `yol.bayt: "veri" bir klasör`) {
		t.Errorf("klasörde bayt: %v", s.failure)
	}
	s = runTR(t, program, `TR_path("yok.txt").TR_size`)
	if s.failure == nil || !strings.Contains(s.failure.Error(), "böyle bir dosya ya da klasör yok") || !strings.Contains(s.failure.Error(), "bakılan yerler") {
		t.Errorf("olmayan dosyada bayt: %v", s.failure)
	}
}

// Yol alan yerleşikler Yol da kabul eder (interp.PathValuer kancası).
func TestPathTakingBuiltins(t *testing.T) {
	tmpWork(t)
	got := mustRunTR(t, `TR_write_file(TR_path("a.txt"), "1,2\n3,4")
		TR_var t1 = TR_read_file(TR_path(".", "a.txt"))
		TR_var t2 = TR_read_csv(TR_path("a.txt"))
		TR_save_tensors(TR_path("m.trt"), {"w": TR_tensor([1.0, 2.0])})
		[t1, t2, TR_load_tensors(TR_path("m.trt"))["w"].TR_tolist]`)
	if got != `["1,2\n3,4", [[1, 2], [3, 4]], [1.0, 2.0]]` {
		t.Fatalf("%s", got)
	}
	// Yol verilen csv_oku içeriği metin sanmaz: dosya yoksa hata verir, csv_çöz önermez.
	errContains(t, `TR_read_csv(TR_path("ad,yaş"))`, "csv_oku", "okunamadı")
	if s := runTR(t, "", `TR_read_csv(TR_path("ad,yaş"))`); messages.Has(s.failure, messages.H7007) {
		t.Errorf("Yol ile csv_çöz önerilmemeli: %v", s.failure)
	}
	// csv_çöz yalnızca metin alır; Yol bir tip hatasıdır.
	if s := runTR(t, "", `TR_parse_csv(TR_path("a.csv"))`); !messages.Has(s.failure, messages.H2423) {
		t.Errorf("csv_çöz(Yol): tip hatası bekleniyordu: %v", s.failure)
	}
	errContains(t, `TR_read_file(5)`, "TR_read_file() 1. argüman olarak TR_String ya da TR_Path bekler")
}

// .gz saydamlığı: dosya_yaz sıkıştırır, dosya_oku, csv_oku ve dosya_satırları açar.
func TestGzipTransparent(t *testing.T) {
	k := tmpWork(t)
	got := mustRunTR(t, `TR_write_file("t.csv.gz", "a,b\n1,2"); [TR_read_file("t.csv.gz"), TR_read_csv("t.csv.gz")]`)
	if got != `["a,b\n1,2", [["a", "b"], [1, 2]]]` {
		t.Fatalf("%s", got)
	}
	b, _ := os.ReadFile(filepath.Join(k, "t.csv.gz"))
	if len(b) < 2 || b[0] != 0x1f || b[1] != 0x8b {
		t.Fatalf("dosya sıkıştırılmamış: %q", b)
	}
	write(t, filepath.Join(k, "duz.gz"), "sıkıştırılmamış")
	errContains(t, `TR_read_file("duz.gz")`, `dosya_oku(): "duz.gz" okunamadı: uzantısı .gz ama içerik gzip değil`)
	errContains(t, `TR_read_lines("duz.gz")`, "uzantısı .gz ama içerik gzip değil")
	write(t, filepath.Join(k, "yarim.gz"), string(b[:len(b)-6]))
	errContains(t, `TR_read_file("yarim.gz")`, "gzip içeriği yarıda kesilmiş")
}
