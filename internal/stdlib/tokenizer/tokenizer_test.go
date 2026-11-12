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

	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
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
	bpePath    = filepath.ToSlash(reporoot.Path("internal/compute/tokenize/testdata/tokenizer_byte_bpe.json"))
	sparsePath = filepath.ToSlash(reporoot.Path("tests/data/tokenizer_sparse.json"))
	bertPath   = filepath.ToSlash(reporoot.Path("internal/compute/tokenize/testdata/tokenizer_wordpiece.json"))
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
TR_var çözülen = p.TR_batch_decode(toplu)
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
TR_var iki = p.TR_batch_decode(TR_Tensor([k, k]))
`)
	if got := al(t, m, "tek").Inspect(); got != "bir iki" {
		t.Fatalf("tek: %q", got)
	}
	if got := al(t, m, "iki").Inspect(); got != `["bir iki", "bir iki"]` {
		t.Fatalf("iki: %s", got)
	}
}

// Kural (16.3, 13.8a): toplu_çöz her diziyi çöz ile aynı metne çevirir, özel_parçalar: ile de;
// boş liste ve 0 satırlı tensör boş liste, boş dizi boş metin verir.
func TestBatchDecodeIsDecodePerSequence(t *testing.T) {
	m := mustRun(t, `
TR_var p = TR_load_tokenizer("`+bpePath+`")
TR_var d = p.TR_encode(["Merhaba dünya!", "hava güzel <|endoftext|>", ""])
TR_var tek = [p.TR_decode(x) TR_for x TR_in d]
TR_var tek_özel = [p.TR_decode(x, TR_special_tokens: TR_true) TR_for x TR_in d]
TR_var toplu = p.TR_batch_decode(d)
TR_var toplu_özel = p.TR_batch_decode(d, TR_special_tokens: TR_true)
TR_var boş = p.TR_batch_decode([])
TR_var boş_tensör = p.TR_batch_decode(TR_zeros([0, 3]))
`)
	for _, c := range [][2]string{{"tek", "toplu"}, {"tek_özel", "toplu_özel"}} {
		if a, b := al(t, m, c[0]).Inspect(), al(t, m, c[1]).Inspect(); a != b {
			t.Errorf("%s %s, %s %s", c[0], a, c[1], b)
		}
	}
	if got := al(t, m, "toplu_özel").Inspect(); !strings.Contains(got, "<|endoftext|>") || strings.Contains(al(t, m, "toplu").Inspect(), "<|endoftext|>") {
		t.Errorf("özel_parçalar: %s", got)
	}
	for _, v := range []string{"boş", "boş_tensör"} {
		if got := al(t, m, v).Inspect(); got != "[]" {
			t.Errorf("%s: %s", v, got)
		}
	}
}

// Kural (16.3): dağarcıkta olmayan kimlik çalışma hatasıdır; ileti kimliği ve dağarcık boyunu,
// toplu_çöz'de dizinin sırasını da söyler (H5647, H5633).
func TestDecodeOutOfVocabularyIDIsAnError(t *testing.T) {
	for _, c := range []struct{ code, text string }{
		{`TR_load_tokenizer("` + bpePath + `").TR_decode([1, 1005])`, "1005 kimliği dağarcıkta yok (dağarcık boyutu 1005)"},
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode([[1], [2, 5000]])`, "(2. dizi): 5000 kimliği dağarcıkta yok (dağarcık boyutu 1005)"},
	} {
		_, err := run(t, c.code)
		if !messages.Has(err, messages.H5647) || !strings.Contains(err.Error(), c.text) {
			t.Errorf("%s: %v, %q içermeli", c.code, err, c.text)
		}
		if !strings.Contains(err.Error(), "TR_vocab_size: p.TR_vocab_size") {
			t.Errorf("%s: %v, üretimin sınırlamasını önermeli", c.code, err)
		}
	}
}

// Kural (16.3): sınırın içindeki boş kimlikte ileti kimliğin boş olduğunu söyler; üretimi
// sınırlamak bu kimliği engellemez, o yüzden öneri yoktur (H5647).
func TestDecodeGapIDSaysItIsEmpty(t *testing.T) {
	for _, code := range []string{
		`TR_load_tokenizer("` + sparsePath + `").TR_decode([1, 3])`,
		`TR_load_tokenizer("` + sparsePath + `").TR_batch_decode([[1], [3]])`,
	} {
		_, err := run(t, code)
		if !messages.Has(err, messages.H5647) || !strings.Contains(err.Error(), "3 kimliği sözlükte boş") {
			t.Errorf("%s: %v, kimliğin boş olduğunu söylemeli", code, err)
		}
		if err != nil && strings.Contains(err.Error(), "TR_vocab_size") {
			t.Errorf("%s: %v, üretimin sınırlamasını önermemeli", code, err)
		}
	}
	_, err := run(t, `TR_load_tokenizer("`+sparsePath+`").TR_decode([6])`)
	if !messages.Has(err, messages.H5647) || !strings.Contains(err.Error(), "TR_vocab_size: p.TR_vocab_size") {
		t.Errorf("sınırın dışı üretimin sınırlamasını önermeli: %v", err)
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
// TestLoadTokenizerFromGGUF: TR_load_tokenizer GGUF dosyasını içerikten tanır (16.1); etiket
// bilinmiyorsa ileti etiketi söyler.
func TestLoadTokenizerFromGGUF(t *testing.T) {
	tokens := make([]string, 0, 94)
	for c := '!'; c <= '~'; c++ {
		tokens = append(tokens, string(c))
	}
	file := func(pre string) string {
		b := (&gguftest.Builder{}).KV("general.architecture", "llama").KV("tokenizer.ggml.model", "gpt2").
			KV("tokenizer.ggml.pre", pre).KV("tokenizer.ggml.tokens", tokens).KV("tokenizer.ggml.merges", []string{}).
			KV("tokenizer.ggml.token_type", make([]int32, 94))
		path := filepath.Join(t.TempDir(), "m.gguf")
		if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(path)
	}
	m, err := run(t, `TR_var p = TR_load_tokenizer("`+file("smollm")+`")
TR_var k = p.TR_encode("ab")
TR_var n = p.TR_vocab_size
TR_var etiket = p.TR_pretokenizer_tag
TR_var durma = p.TR_declared_eos_tokens`)
	if err != nil {
		t.Fatal(err)
	}
	// 16.1: GGUF'ta etiket dosyanınkidir; dosya durma kimliği bildirmiyorsa üye yoktur (boş liste değil).
	if got := al(t, m, "etiket").Inspect(); got != "smollm" {
		t.Errorf("etiket %s", got)
	}
	if got := al(t, m, "durma"); got != interp.Null {
		t.Errorf("bildirilen durma kimlikleri %s; yok olmalı", got.Inspect())
	}
	if got := al(t, m, "n").Inspect(); got != "94" {
		t.Errorf("dağarcık %s", got)
	}
	if got := al(t, m, "k").Inspect(); got != "[64, 65]" {
		t.Errorf("kimlikler %s", got)
	}
	// Tanınmayan etiket ve parçalayıcısız GGUF H5620'nin içindedir: ileti işlevin adını ve dosyanın
	// yolunu yazar (DENETİM 453 N2). Yalnız dosya okuma hatası ayrıdır: H5620 değil, işlevin adıyla
	// (H2000) H2107; içinde H5620 yoktur (15.55).
	bare := filepath.Join(t.TempDir(), "yalın.gguf")
	if err := os.WriteFile(bare, (&gguftest.Builder{}).KV("general.architecture", "llama").Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "yok.gguf")
	for _, c := range []struct {
		name, path string
		top, inner messages.Code
	}{
		{"bilinmeyen etiket", file("gpt-4o"), messages.H5620, messages.H6679},
		{"parçalayıcısız", filepath.ToSlash(bare), messages.H5620, messages.H5650},
		{"okunamayan dosya", filepath.ToSlash(missing), messages.H2000, messages.H2107},
	} {
		_, err := run(t, `TR_load_tokenizer("`+c.path+`")`)
		top, ok := messages.As(err)
		if !ok || top.Code != c.top || !messages.Has(err, c.inner) || (c.top == messages.H2000 && messages.Has(err, messages.H5620)) {
			t.Errorf("%s: üst kod %v, içinde %v beklendi: %v", c.name, c.top, c.inner, err)
			continue
		}
		text := err.Error()
		named := strings.Contains(text, "parçalayıcı_yükle") || strings.Contains(text, "load_tokenizer")
		if c.top == messages.H5620 && (!named || !strings.Contains(text, filepath.Base(c.path))) {
			t.Errorf("%s: ileti işlevin adını ve yolu yazmıyor: %v", c.name, err)
		}
	}
}

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
TR_try { p.TR_encode("`+bad+`") } TR_catch e { h1 = TR_String(e) }
TR_var h2 = ""
TR_try { p.TR_encode(["iyi", "`+bad+`"]) } TR_catch e { h2 = TR_String(e) }
`)
	})
	if strings.Count(warning, "Uyarı: parçalayıcı") != 1 || !strings.Contains(warning, "riskli desen") {
		t.Fatalf("uyarı bir kez yazılmalıydı:\n%s", warning)
	}
	// Yakalanan hatanın metni iletiyi kuran çağrıdır (8.3): adım sınırının kodu (H5623) ve toplu
	// kodlamada metnin sırası (H5629, 2. metin).
	for _, name := range []string{"h1", "h2"} {
		if h := al(t, m, name).Inspect(); !strings.HasPrefix(h, `Message("H5623", `) {
			t.Fatalf("%s: adım sınırı hatası bekleniyordu: %q", name, h)
		}
	}
	if h := al(t, m, "h2").Inspect(); !strings.Contains(h, `Message("H5629", "TR_encode", 2)`) {
		t.Fatalf("toplu hatada metnin sırası belirtilmeli: %s", h)
	}
	if text := messages.ErrorIn(messages.New(messages.H5623, "f", "x", 1, 0), "en"); !strings.Contains(text, "exceeded the step limit") || !strings.Contains(text, "smaller pieces") {
		t.Fatalf("gösterilen ileti yol göstermeli: %q", text)
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
		{`TR_load_tokenizer("` + bpePath + `").TR_decode(TR_Tensor([[[1.0]]]))`, messages.H5636},
		{`TR_load_tokenizer("` + bpePath + `").TR_decode(TR_Tensor(1))`, messages.H5636},
		{`TR_load_tokenizer("` + bpePath + `").TR_decode([[1], [2]])`, messages.H5642},
		{`TR_load_tokenizer("` + bpePath + `").TR_decode(TR_Tensor([[1], [2]]))`, messages.H5642},
		{`TR_load_tokenizer("` + bpePath + `").TR_decode()`, messages.H5632},
		{`TR_load_tokenizer("` + bpePath + `").TR_decode("a")`, messages.H5638},
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode([1, 2])`, messages.H5643},
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode(TR_Tensor([1, 2]))`, messages.H5643},
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode([[1], 2])`, messages.H5634},
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode(["a"])`, messages.H5634},
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode([[1]], [[2]])`, messages.H5644},
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode("a")`, messages.H5645},
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode(TR_Tensor([[[1]]]))`, messages.H5646},
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode(TR_Tensor([[1.5]]))`, messages.H5637},
		{`TR_load_tokenizer("` + bpePath + `").çözz([1])`, messages.H7903},
	}
	for _, dd := range cases {
		_, err := run(t, dd.code)
		if !messages.Has(err, dd.want) {
			t.Errorf("%s\n hata: %v\n beklenen: %s", dd.code, err, dd.want)
		}
	}
}

// Kural (16.1, klasörle açma): klasörün tokenizer.json'u yüklenir; aile config.json'un model_type'ı ve tokenizer_config.json'un
// işaretiyle seçilir (sınıf Fast'sız, genel sınıfta bos_token). config.json'u olmayan ya da tabloda olmayan klasör ailesiz.
func TestLoadTokenizerFolderPicksTheFamily(t *testing.T) {
	for _, c := range []struct{ data, config, family string }{{"qwen3_chat", `{"model_type": "qwen3"}`, "qwen3"},
		{"llama3_chat", `{"model_type": "llama"}`, "llama3"}, {"qwen3_chat", "", ""}, {"llama3_chat", `{"model_type": "qwen3"}`, ""}} {
		d := t.TempDir()
		for _, f := range []string{"tokenizer.json", "tokenizer_config.json"} {
			b, err := os.ReadFile(reporoot.Path("tests", "data", c.data, f))
			if err != nil || os.WriteFile(filepath.Join(d, f), b, 0o644) != nil {
				t.Fatal(f, err)
			}
		}
		if c.config != "" && os.WriteFile(filepath.Join(d, "config.json"), []byte(c.config), 0o644) != nil {
			t.Fatal("config.json")
		}
		p := al(t, mustRun(t, `TR_var p = TR_load_tokenizer("`+filepath.ToSlash(d)+`")`), "p").(interface{ Tokenizer() *tokenize.Tokenizer })
		if got := p.Tokenizer().Family(); got != c.family {
			t.Errorf("%s, %q: aile %q, beklenen %q", c.data, c.config, got, c.family)
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
		{`TR_load_tokenizer("` + bpePath + `").TR_batch_decode([[1], [2, "a"]])`, 2, "2. listenin 2. elemanı"},
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
	found := 0
	for _, name := range []string{"tokenizer.go", "decode.go"} {
		found += heavyCallsReleased(t, name)
	}
	if found < 6 {
		t.Errorf("ağır çağrılar taranamadı (%d bulundu)", found)
	}
}

// heavyCallsReleased paketin bir dosyasındaki ağır çağrıları sayar; bırakan bir gövdenin
// dışında olanı hatadır.
func heavyCallsReleased(t *testing.T, name string) int {
	file, err := goparser.ParseFile(token.NewFileSet(), reporoot.Path("internal", "stdlib", "tokenizer", name), nil, goparser.SkipObjectResolution)
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
		t.Errorf("%s: %s çağrısı Blocking/maybeRelease dışında: yorumlayıcı kilidi tutulurken uzun iş yapılır", name, heavy(c))
		return true
	})
	return found
}

// Kural (16.1): dolu kimlik yazan dosya yüklenmez; yükleme hatası iki parçanın adını söyler (H5291).
func TestLoadRejectsAnAddedTokenOnATakenID(t *testing.T) {
	path := filepath.ToSlash(reporoot.Path("tests/data/broken_tokenizer_added_taken.json"))
	_, err := run(t, `TR_load_tokenizer("`+path+`")`)
	if !messages.Has(err, messages.H5291) || !strings.Contains(err.Error(), `"<s>"`) || !strings.Contains(err.Error(), `"b"`) {
		t.Fatalf("H5291 ve iki parçanın adı bekleniyordu: %v", err)
	}
}
