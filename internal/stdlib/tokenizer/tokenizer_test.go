package tokenizer_test

import (
	"encoding/json"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tokenize"
	"github.com/sekerahmet/TCB/internal/devtools/debugger"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Test parçalayıcıları; yollar TR kaynağına da yazıldığı için bölü çizgisiyle.
var (
	bpePath  = filepath.ToSlash(reporoot.Path("internal/compute/tokenize/testdata/tokenizer_byte_bpe.json"))
	bertPath = filepath.ToSlash(reporoot.Path("internal/compute/tokenize/testdata/tokenizer_wordpiece.json"))
)

func run(t *testing.T, code string) (*interp.VM, error) {
	t.Helper()
	o := trrun.Exec(t, code)
	return o.VM, o.Err
}

func mustRun(t *testing.T, code string) *interp.VM {
	t.Helper()
	return trrun.MustExec(t, code).VM
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
TR_var p = TR_load_tokenizer("`+bpePath+`")
TR_var tek = p.TR_encode("Merhaba dünya!")
TR_var toplu = p.TR_encode(["Merhaba dünya!", "İstanbul'da hava güzel <|endoftext|>", ""])
TR_var çözülen = p.TR_decode(toplu)
TR_var özelli = p.TR_decode(toplu[1], TR_special_tokens: TR_true)
TR_var özelsiz = p.TR_decode(toplu[1])
TR_var boyut = p.TR_vocab_size
TR_var eot = p.TR_token_id("<|endoftext|>")
TR_var yok_token = p.TR_token_id("böyle-bir-token-yok")
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
	if al(t, m, "yok_token") != interp.Null {
		t.Fatal("olmayan token yok dönmeli")
	}
}

// özel_parçalar: yanlış son işleyicinin eklemelerini kapatır; ikinci: çift kodlar.
func TestSpecialTokensAndPair(t *testing.T) {
	g := goTokenizer(t, bertPath)
	m := mustRun(t, `
TR_var p = TR_load_tokenizer("`+bertPath+`")
TR_var a = p.TR_encode("merhaba dünya")
TR_var b = p.TR_encode("merhaba dünya", TR_special_tokens: TR_false)
TR_var c = p.TR_encode("soru", TR_pair: "bağlam")
TR_var d = p.TR_encode(["soru", "bir"], TR_pair: ["bağlam", "iki"], TR_special_tokens: TR_false)
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

// Tensör (ör. argmax sonucu ya da okuyucunun girdi tensörü) doğrudan çözülebilir.
func TestDecodeTensor(t *testing.T) {
	m := mustRun(t, `
TR_var p = TR_load_tokenizer("`+bpePath+`")
TR_var k = p.TR_encode("bir iki")
TR_var tek = p.TR_decode(TR_Tensor(k))
TR_var iki = p.TR_decode(TR_Tensor([k, k]))
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
TR_var p = TR_load_tokenizer("`+path+`")
TR_var p2 = TR_load_tokenizer("`+path+`")
TR_var iyi = p.TR_encode("merhaba dünya")
TR_var h1 = ""
TR_try { p.TR_encode("`+bad+`") } TR_catch e { h1 = e }
TR_var h2 = ""
TR_try { p.TR_encode(["iyi", "`+bad+`"]) } TR_catch e { h2 = e }
`)
	})
	if strings.Count(warning, "Uyarı: parçalayıcı") != 1 || !strings.Contains(warning, "riskli desen") {
		t.Fatalf("uyarı bir kez yazılmalıydı:\n%s", warning)
	}
	for _, name := range []string{"h1", "h2"} {
		h := al(t, m, name).Inspect()
		if !strings.Contains(h, "exceeded the step limit") || !strings.Contains(h, "smaller pieces") {
			t.Fatalf("%s: adım sınırı hatası bekleniyordu: %q", name, h)
		}
	}
	if !strings.Contains(al(t, m, "h2").Inspect(), "string 2 of the list") {
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
	cases := []struct {
		code string
		want messages.Code
	}{
		{`TR_load_tokenizer("yok_boyle_bir_dosya.json")`, messages.H2132},
		{`TR_load_tokenizer("` + filepath.ToSlash(d) + `")`, messages.H5610},
		{`TR_load_tokenizer("` + unsupported + `")`, messages.H5614},
		{`TR_load_tokenizer("` + risky + `")`, messages.H5615},
		{`TR_load_tokenizer("` + broken + `")`, messages.H5617},
		{`TR_load_tokenizer(5)`, messages.H5609},
		{`TR_load_tokenizer("` + bpePath + `").TR_encode(5)`, messages.H5630},
		{`TR_load_tokenizer("` + bpePath + `").TR_encode(["a", 1])`, messages.H5631},
		{`TR_load_tokenizer("` + bpePath + `").TR_encode("a", özel_parçlar: TR_true)`, messages.H7900},
		{`TR_load_tokenizer("` + bpePath + `").TR_encode("a", TR_special_tokens: 1)`, messages.H7905},
		{`TR_load_tokenizer("` + bpePath + `").TR_encode(["a"], TR_pair: ["b", "c"])`, messages.H5628},
		{`TR_load_tokenizer("` + bpePath + `").TR_decode([1, "a"])`, messages.H5639},
		{`TR_load_tokenizer("` + bpePath + `").TR_decode(TR_Tensor([[[1]]]))`, messages.H5636},
		{`TR_load_tokenizer("` + bpePath + `").çözz([1])`, messages.H7903},
	}
	for _, dd := range cases {
		_, err := run(t, dd.code)
		if !messages.Has(err, dd.want) {
			t.Errorf("%s\n hata: %v\n beklenen: %s", dd.code, err, dd.want)
		}
	}
}

// Kural: tam sayı olmayan kimliğin hangi listede olduğu iletiye ileti parçası ya da Türkçe sözcük
// olarak değil satır numarası olarak gider (0: düz liste); sözcükleri şablon seçer (H5639).
func TestDecodeNonIntegerIDNamesTheRow(t *testing.T) {
	for _, c := range []struct {
		code string
		row  int
		text string
	}{
		{`TR_load_tokenizer("` + bpePath + `").TR_decode([1, "a"])`, 0, "listenin 2. elemanı"},
		{`TR_load_tokenizer("` + bpePath + `").TR_decode([[1], [2, "a"]])`, 2, "2. listenin 2. elemanı"},
	} {
		_, err := run(t, c.code)
		e, ok := messages.As(err)
		if !ok || e.Code != messages.H5639 || len(e.Args) < 2 || e.Args[1] != c.row {
			t.Errorf("%s: %#v, satır %d bekleniyordu", c.code, e, c.row)
			continue
		}
		if !strings.Contains(err.Error(), c.text) {
			t.Errorf("%s: %q, %q içermeli", c.code, err.Error(), c.text)
		}
	}
}

func TestWatchPanel(t *testing.T) {
	m := mustRun(t, `TR_var p = TR_load_tokenizer("`+bpePath+`")`)
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

// Uzun iş yorumlayıcıyı kilitlemez: parçalayıcı dosyasının yüklenmesi ve kodlama/çözme
// çağrıları (tokenize.Load, Safe*) Blocking içinde (ya da uzun metinde
// maybeRelease ile) çalışır; böylece başka görevler bu sırada ilerler. Çağrılar, bu
// ikisine verilen işlev gövdelerinin (doğrudan ya da bir değişkenle verilen) içinde olmalıdır.
func TestHeavyWorkReleasesLock(t *testing.T) {
	file, err := goparser.ParseFile(token.NewFileSet(), reporoot.Path("internal", "stdlib", "tokenizer", "tokenizer.go"), nil, goparser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	isRelease := func(c *ast.CallExpr) bool {
		switch f := c.Fun.(type) {
		case *ast.SelectorExpr:
			// Abandonable ve AwaitNew de işi kilidi bırakarak koşar (1.5).
			return f.Sel.Name == "Blocking" || f.Sel.Name == "maybeRelease" || f.Sel.Name == "Abandonable" || f.Sel.Name == "AwaitNew"
		case *ast.Ident:
			return f.Name == "maybeRelease"
		}
		return false
	}
	// 1. geçiş: bırakan çağrılara verilen işlev gövdeleri.
	released := map[*ast.FuncLit]bool{}
	releasedNames := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok && isRelease(c) {
			for _, a := range c.Args {
				switch x := a.(type) {
				case *ast.FuncLit:
					released[x] = true
				case *ast.Ident:
					releasedNames[x.Name] = true
				}
			}
		}
		return true
	})
	ast.Inspect(file, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && len(a.Rhs) == 1 {
			id, isIdent := a.Lhs[0].(*ast.Ident)
			lit, isLit := a.Rhs[0].(*ast.FuncLit)
			if isIdent && isLit && releasedNames[id.Name] {
				released[lit] = true
			}
		}
		return true
	})
	// 2. geçiş: ağır çağrıların bir atası bırakan işlev gövdesi olmalı.
	heavy := func(c *ast.CallExpr) string {
		switch f := c.Fun.(type) {
		case *ast.SelectorExpr:
			if x, ok := f.X.(*ast.Ident); ok && x.Name == "tokenize" && f.Sel.Name == "Load" {
				return "tokenize.Load"
			}
			if strings.HasPrefix(f.Sel.Name, "Safe") && f.Sel.Name != "Safe" && f.Sel.Name != "SafeBuiltin" { // bind.Safe ve SafeBuiltin panik sarmalayıcıdır, ağır iş değil
				return f.Sel.Name
			}
		}
		return ""
	}
	var stack []ast.Node
	found := 0
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		c, ok := n.(*ast.CallExpr)
		if !ok || heavy(c) == "" {
			return true
		}
		found++
		for _, a := range stack {
			if lit, ok := a.(*ast.FuncLit); ok && released[lit] {
				return true
			}
		}
		t.Errorf("tokenizer.go: %s çağrısı Blocking/maybeRelease dışında: yorumlayıcı kilidi tutulurken uzun iş yapılır", heavy(c))
		return true
	})
	if found < 6 {
		t.Errorf("ağır çağrılar taranamadı (%d bulundu)", found)
	}
}
