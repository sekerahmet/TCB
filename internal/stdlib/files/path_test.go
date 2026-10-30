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
		`TR_Path("a/./b/../c")`:              "a/c",
		`TR_Path("veri", "eğitim", "a.npy")`: "veri/eğitim/a.npy",
		`TR_Path("veri/", "./a.npy")`:        "veri/a.npy",
		`TR_Path(".")`:                       ".",
		`TR_Path("a", "..")`:                 ".",
		`TR_Path("..", "a")`:                 "../a",
		`TR_Path(TR_Path("veri"), "a")`:      "veri/a",
		`TR_Path("a//b")`:                    "a/b",
	}
	if runtime.GOOS == "windows" {
		cases[`TR_Path("veri\\alt", "a.txt")`] = "veri/alt/a.txt"
		cases[`TR_Path("C:\\veri\\..\\x")`] = "C:/x"
	}
	for code, wants := range cases {
		if got := mustRunTR(t, code); got != wants {
			t.Errorf("%s = %q, %q bekleniyordu", code, got, wants)
		}
	}
}

func TestPathMembers(t *testing.T) {
	cases := map[string]string{
		`TR_Path("veri/derlem.tar.gz").TR_name`:      "derlem.tar.gz",
		`TR_Path("veri/derlem.tar.gz").TR_extension`: ".gz",
		`TR_Path("veri/derlem.tar.gz").TR_stem`:      "derlem.tar",
		`TR_Path(".gitignore").TR_extension`:         "",
		`TR_Path(".gitignore").TR_stem`:              ".gitignore",
		`TR_Path("a.").TR_extension`:                 "",
		`TR_Path("Makefile").TR_extension`:           "",
		`TR_Path(".").TR_name`:                       "",
		`TR_Path("a.txt").TR_parent`:                 ".",
		`TR_Path("veri/eğitim/a.npy").TR_parent`:     "veri/eğitim",
		`TR_Type(TR_Path("a").TR_parent)`:            "Path",
		`TR_Path("a") == TR_Path("a/./")`:            "true",
		`TR_Path("a") == "a"`:                        "false",
		`"\(TR_Path("x", "y"))"`:                     "x/y",
		`TR_json_string({"y": TR_Path("a/b")})`:      `{"y":"a/b"}`,
	}
	for code, wants := range cases {
		if got := mustRunTR(t, code); got != wants {
			t.Errorf("%s = %q, %q bekleniyordu", code, got, wants)
		}
	}
	errContains(t, `TR_Path("a").TR_embed_dim`, "özelliği ya da metodu bulunamadı")
	errContains(t, `TR_Path("a").dosya_adi`, "TR_name")
}

func TestPathErrors(t *testing.T) {
	errContains(t, `TR_Path("a", "/etc")`, `Yol() 2. parça "/etc" mutlak bir yol`)
	errHas(t, `TR_Path("a", "")`, messages.H7138)
	errContains(t, `TR_Path("")`, "1. parça boş metin")
	errContains(t, `TR_Path()`, "Yol()")
	errContains(t, `TR_Path(5)`, "1. argüman olarak Metin ya da Yol bekler, Tam (5) verildi")
	errContains(t, `TR_Path("C:\tokenizer.json")`, "sekme karakteri var", "ham metin")
	if runtime.GOOS == "windows" {
		errHas(t, `TR_Path("a", "C:\\x")`, messages.H7139)
		errHas(t, `TR_Path("a", "C:x")`, messages.H7139)
		errHas(t, `TR_Path("a", "\\\\sunucu\\pay")`, messages.H7139)
	}
}

// Diske bakan özellikler yolu kullanıldığı anda 13.1'in tek kuralıyla çözer: önce çalışma
// klasörü, sonra programın klasörü.
func TestPathDiskQueries(t *testing.T) {
	work := tmpWork(t)
	program := t.TempDir()
	write(t, filepath.Join(program, "veri", "a.txt"), "merhaba")
	write(t, filepath.Join(work, "b.txt"), "xy")
	s := runTR(t, program, `[TR_Path("veri/a.txt").TR_exists, TR_Path("veri/a.txt").TR_is_file, TR_Path("veri").TR_is_dir,
		TR_Path("veri").TR_is_file, TR_Path("yok").TR_exists, TR_Path("yok").TR_is_dir, TR_Path("veri/a.txt").TR_size, TR_Path("b.txt").TR_size]`)
	if s.failure != nil || s.value.Inspect() != "[true, true, true, false, false, false, 7, 2]" {
		t.Fatalf("%v %v", s.value, s.failure)
	}
	s = runTR(t, program, `TR_Path("veri/a.txt").TR_absolute`)
	if s.failure != nil || !strings.EqualFold(s.value.Inspect(), forwardPath(filepath.Join(program, "veri", "a.txt"))) {
		t.Errorf("mutlak: %v %v", s.value, s.failure)
	}
	s = runTR(t, program, `TR_Path("veri").TR_size`)
	if s.failure == nil || !strings.Contains(s.failure.Error(), `yol.bayt: "veri" bir klasör`) {
		t.Errorf("klasörde bayt: %v", s.failure)
	}
	s = runTR(t, program, `TR_Path("yok.txt").TR_size`)
	if s.failure == nil || !strings.Contains(s.failure.Error(), "böyle bir dosya ya da klasör yok") || !strings.Contains(s.failure.Error(), "bakılan yerler") {
		t.Errorf("olmayan dosyada bayt: %v", s.failure)
	}
}

// Yol alan yerleşikler Yol da kabul eder (interp.PathValuer kancası).
func TestPathTakingBuiltins(t *testing.T) {
	tmpWork(t)
	got := mustRunTR(t, `TR_write_file(TR_Path("a.txt"), "1,2\n3,4")
		TR_var t1 = TR_read_file(TR_Path(".", "a.txt"))
		TR_var t2 = TR_read_csv(TR_Path("a.txt"))
		TR_save_tensors(TR_Path("m.trt"), {"w": TR_Tensor([1.0, 2.0])})
		[t1, t2, TR_load_tensors(TR_Path("m.trt"))["w"].TR_tolist]`)
	if got != `["1,2\n3,4", [[1, 2], [3, 4]], [1.0, 2.0]]` {
		t.Fatalf("%s", got)
	}
	// Yol verilen csv_oku içeriği metin sanmaz: dosya yoksa hata verir, csv_çöz önermez.
	errContains(t, `TR_read_csv(TR_Path("ad,yaş"))`, "csv_oku", "okunamadı")
	if s := runTR(t, "", `TR_read_csv(TR_Path("ad,yaş"))`); messages.Has(s.failure, messages.H7007) {
		t.Errorf("Yol ile csv_çöz önerilmemeli: %v", s.failure)
	}
	// csv_çöz yalnızca metin alır; Yol bir tip hatasıdır.
	if s := runTR(t, "", `TR_parse_csv(TR_Path("a.csv"))`); !messages.Has(s.failure, messages.H2423) {
		t.Errorf("csv_çöz(Yol): tip hatası bekleniyordu: %v", s.failure)
	}
	errContains(t, `TR_read_file(5)`, "dosya_oku() 1. argüman olarak Metin ya da Yol bekler")
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
