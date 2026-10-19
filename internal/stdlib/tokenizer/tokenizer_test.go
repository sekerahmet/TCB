package tokenizer_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tokenize"
	"github.com/sekerahmet/TCB/internal/devtools/debugger"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
)

// Test parçalayıcıları; yollar TR kaynağına da yazıldığı için bölü çizgisiyle.
var (
	bpePath  = filepath.ToSlash(reporoot.Path("internal/compute/tokenize/testdata/p_bayt_bpe.json"))
	bertPath = filepath.ToSlash(reporoot.Path("internal/compute/tokenize/testdata/p_wordpiece.json"))
)

func run(t *testing.T, code string) (*interp.VM, error) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	m.Out = &bytes.Buffer{}
	_, err := m.Run(prog)
	return m, err
}

func mustRun(t *testing.T, code string) *interp.VM {
	t.Helper()
	m, err := run(t, code)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v\nkod:\n%s", err, code)
	}
	return m
}

func al(t *testing.T, m *interp.VM, name string) interp.Value {
	t.Helper()
	v, ok := m.GlobalEnv.Get(name)
	if !ok {
		t.Fatalf("%s yok", name)
	}
	return v
}

func ints(t *testing.T, v interp.Value) []int {
	t.Helper()
	l, ok := v.(*interp.ListVal)
	if !ok {
		t.Fatalf("liste bekleniyordu: %s", v.Inspect())
	}
	out := []int{}
	for _, e := range l.Elements {
		out = append(out, int(e.(*interp.IntVal).Val))
	}
	return out
}

func goTokenizer(t *testing.T, path string) *tokenize.Tokenizer {
	t.Helper()
	p, err := tokenize.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TR'deki kodla/çöz sonuçları Go kütüphanesiyle birebir aynıdır.
func TestEncodeDecodeSameAsGo(t *testing.T) {
	g := goTokenizer(t, bpePath)
	texts := []string{"Merhaba dünya!", "İstanbul'da hava güzel <|endoftext|>", ""}
	m := mustRun(t, `
değişken p = parçalayıcı_yükle("`+bpePath+`")
değişken tek = p.kodla("Merhaba dünya!")
değişken toplu = p.kodla(["Merhaba dünya!", "İstanbul'da hava güzel <|endoftext|>", ""])
değişken çözülen = p.çöz(toplu)
değişken özelli = p.çöz(toplu[1], özel_parçalar: doğru)
değişken özelsiz = p.çöz(toplu[1])
değişken boyut = p.dağarcık_boyutu
değişken eot = p.parça_kimliği("<|endoftext|>")
değişken yok_token = p.parça_kimliği("böyle-bir-token-yok")
`)
	if got := ints(t, al(t, m, "tek")); !reflect.DeepEqual(got, g.Encode(texts[0])) {
		t.Fatalf("kodla: %v, Go: %v", got, g.Encode(texts[0]))
	}
	batch := al(t, m, "toplu").(*interp.ListVal)
	for i, s := range texts {
		if got := ints(t, batch.Elements[i]); !reflect.DeepEqual(got, g.Encode(s)) {
			t.Fatalf("toplu kodla %d: %v, Go: %v", i, got, g.Encode(s))
		}
	}
	decoded := al(t, m, "çözülen").(*interp.ListVal)
	for i, s := range texts {
		if got := decoded.Elements[i].Inspect(); got != g.Decode(g.Encode(s), true) {
			t.Fatalf("toplu çöz %d: %q", i, got)
		}
	}
	if got := al(t, m, "özelli").Inspect(); got != texts[1] {
		t.Fatalf("özel_parçalar: doğru ile çöz: %q", got)
	}
	if got := al(t, m, "özelsiz").Inspect(); strings.Contains(got, "<|endoftext|>") {
		t.Fatalf("varsayılan çöz özel tokenları atlamalı: %q", got)
	}
	if got := al(t, m, "boyut").(*interp.IntVal).Val; int(got) != g.VocabSize() {
		t.Fatalf("sözlük boyutu %d", got)
	}
	id, _ := g.TokenToID("<|endoftext|>")
	if got := al(t, m, "eot").(*interp.IntVal).Val; int(got) != id {
		t.Fatalf("token_kimliği %d", got)
	}
	if al(t, m, "yok_token") != interp.NULL {
		t.Fatal("olmayan token yok dönmeli")
	}
}

// özel_parçalar: yanlış son işleyicinin eklemelerini kapatır; ikinci: çift kodlar.
func TestSpecialTokensAndPair(t *testing.T) {
	g := goTokenizer(t, bertPath)
	m := mustRun(t, `
değişken p = parçalayıcı_yükle("`+bertPath+`")
değişken a = p.kodla("merhaba dünya")
değişken b = p.kodla("merhaba dünya", özel_parçalar: yanlış)
değişken c = p.kodla("soru", ikinci: "bağlam")
değişken d = p.kodla(["soru", "bir"], ikinci: ["bağlam", "iki"], özel_parçalar: yanlış)
`)
	if got := ints(t, al(t, m, "a")); !reflect.DeepEqual(got, g.Encode("merhaba dünya")) {
		t.Fatalf("a: %v", got)
	}
	if got, b := ints(t, al(t, m, "b")), g.EncodeWithOptions("merhaba dünya", tokenize.Options{DontAddSpecial: true}); !reflect.DeepEqual(got, b) {
		t.Fatalf("b: %v, Go: %v", got, b)
	}
	ik := "bağlam"
	k := g.EncodePair("soru", &ik, tokenize.Options{})
	want := []int{}
	for _, x := range k.IDs {
		want = append(want, int(x))
	}
	if got := ints(t, al(t, m, "c")); !reflect.DeepEqual(got, want) {
		t.Fatalf("c: %v, Go: %v", got, want)
	}
	d := al(t, m, "d").(*interp.ListVal)
	ik2 := "iki"
	k2 := g.EncodePair("bir", &ik2, tokenize.Options{DontAddSpecial: true})
	want = []int{}
	for _, x := range k2.IDs {
		want = append(want, int(x))
	}
	if got := ints(t, d.Elements[1]); !reflect.DeepEqual(got, want) {
		t.Fatalf("d[1]: %v, Go: %v", got, want)
	}
}

// Tensör (ör. maks_indeks sonucu ya da okuyucunun girdi tensörü) doğrudan çözülebilir.
func TestDecodeTensor(t *testing.T) {
	m := mustRun(t, `
değişken p = parçalayıcı_yükle("`+bpePath+`")
değişken k = p.kodla("bir iki")
değişken tek = p.çöz(tensör(k))
değişken iki = p.çöz(tensör([k, k]))
`)
	if got := al(t, m, "tek").Inspect(); got != "bir iki" {
		t.Fatalf("tek: %q", got)
	}
	if got := al(t, m, "iki").Inspect(); got != `["bir iki", "bir iki"]` {
		t.Fatalf("iki: %s", got)
	}
}

// riskyTokenizer bayt düzeyi BPE dosyasının ön bölme desenini, kabul edilen ama bazı
// girdilerde aşırı geri izleyen bir desenle değiştirip geçici bir dosyaya yazar.
func riskyTokenizer(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(bpePath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	pt := raw["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
	pt[0].(map[string]any)["pattern"] = map[string]any{"Regex": `(a|a)*b|\s+|\S`}
	fresh, _ := json.Marshal(raw)
	path := filepath.Join(t.TempDir(), "riskli.json")
	if err := os.WriteFile(path, fresh, 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(path)
}

// captureStderr f çalışırken standart hataya yazılanları döndürür.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	f()
	os.Stderr = old
	w.Close()
	return <-done
}

// Düzenli ifade adım sınırı aşılırsa yarım sonuç yerine yol gösterici bir hata oluşur;
// riskli desen uyarısı standart hataya dosya başına bir kez yazılır.
func TestStepLimitAndWarning(t *testing.T) {
	path := riskyTokenizer(t)
	bad := strings.Repeat("a", 40)
	var m *interp.VM
	warning := captureStderr(t, func() {
		m = mustRun(t, `
değişken p = parçalayıcı_yükle("`+path+`")
değişken p2 = parçalayıcı_yükle("`+path+`")
değişken iyi = p.kodla("merhaba dünya")
değişken h1 = ""
dene { p.kodla("`+bad+`") } yakala e { h1 = e }
değişken h2 = ""
dene { p.kodla(["iyi", "`+bad+`"]) } yakala e { h2 = e }
`)
	})
	if strings.Count(warning, "Uyarı: parçalayıcı") != 1 || !strings.Contains(warning, "riskli desen") {
		t.Fatalf("uyarı bir kez yazılmalıydı:\n%s", warning)
	}
	for _, name := range []string{"h1", "h2"} {
		h := al(t, m, name).Inspect()
		if !strings.Contains(h, "adım sınırını") || !strings.Contains(h, "daha küçük parçalara") {
			t.Fatalf("%s: adım sınırı hatası bekleniyordu: %q", name, h)
		}
	}
	if !strings.Contains(al(t, m, "h2").Inspect(), "listenin 2. metni") {
		t.Fatalf("toplu hatada metnin sırası belirtilmeli: %s", al(t, m, "h2").Inspect())
	}
}

func TestLoadErrors(t *testing.T) {
	d := t.TempDir()
	write := func(name, content string) string {
		y := filepath.Join(d, name)
		if err := os.WriteFile(y, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(y)
	}
	unsupported := write("desteksiz.json", `{"model": {"type": "Bilinmeyen", "vocab": {}}}`)
	risky := write("riskli.json", `{"pre_tokenizer": {"type": "Split", "pattern": {"Regex": "(a+)+"}, "behavior": "Isolated", "invert": false},
		"model": {"type": "WordLevel", "vocab": {"a": 0, "[UNK]": 1}, "unk_token": "[UNK]"}}`)
	broken := write("bozuk.json", `{"model": `)
	cases := []struct{ code, want string }{
		{`parçalayıcı_yükle("yok_boyle_bir_dosya.json")`, "yüklenemedi: böyle bir dosya ya da klasör yok"},
		{`parçalayıcı_yükle("` + filepath.ToSlash(d) + `")`, "bir klasör"},
		{`parçalayıcı_yükle("` + unsupported + `")`, "parçalayıcı türü desteklenmiyor"},
		{`parçalayıcı_yükle("` + risky + `")`, "güvenlik gereği kabul edilmiyor"},
		{`parçalayıcı_yükle("` + broken + `")`, "geçerli bir tokenizer.json dosyası değil"},
		{`parçalayıcı_yükle(5)`, "yol bir metin olmalıdır"},
		{`parçalayıcı_yükle("` + bpePath + `").kodla(5)`, "metin ya da metin listesi"},
		{`parçalayıcı_yükle("` + bpePath + `").kodla(["a", 1])`, "2. eleman Tam"},
		{`parçalayıcı_yükle("` + bpePath + `").kodla("a", özel_parçlar: doğru)`, "Bunu mu demek istediniz: 'özel_parçalar'"},
		{`parçalayıcı_yükle("` + bpePath + `").kodla("a", özel_parçalar: 1)`, "doğru ya da yanlış"},
		{`parçalayıcı_yükle("` + bpePath + `").kodla(["a"], ikinci: ["b", "c"])`, "eşit uzunlukta"},
		{`parçalayıcı_yükle("` + bpePath + `").çöz([1, "a"])`, "tam sayı olmalıdır"},
		{`parçalayıcı_yükle("` + bpePath + `").çöz(tensör([[[1]]]))`, "1 boyutlu"},
		{`parçalayıcı_yükle("` + bpePath + `").çözz([1])`, "Bunu mu demek istediniz: 'çöz'"},
	}
	for _, dd := range cases {
		_, err := run(t, dd.code)
		if err == nil || !strings.Contains(err.Error(), dd.want) {
			t.Errorf("%s\n hata: %v\n beklenen: %q", dd.code, err, dd.want)
		}
	}
}

func TestWatchPanel(t *testing.T) {
	m := mustRun(t, `değişken p = parçalayıcı_yükle("`+bpePath+`")`)
	iz, ok := al(t, m, "p").(debugger.WatchValue)
	if !ok {
		t.Fatal("WatchValue değil")
	}
	if iz.WatchLabel() != "Parçalayıcı (dağarcık 1005)" {
		t.Fatalf("etiket %q", iz.WatchLabel())
	}
	cols, rows, err := iz.TableData()
	if err != nil || len(cols) != 3 || len(rows) == 0 || rows[0][1] != "<|endoftext|>" {
		t.Fatalf("tablo %v %v %v", cols, rows, err)
	}
}
