package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.55, sohbet tanımının seçimi ve durma kimlikleri): hazır modelin durma listesi sohbet
// tanımındaki adlardan parçalayıcının kimliğiyle kurulur; m.TR_eos_tokens() onu verir. GGUF'ta aile
// (mimari, p.TR_pretokenizer_tag), klasörde (model_type, tokenizer_config.json'un işareti) ile
// seçilir. Adı dağarcıkta olmayan durma parçası H7162, dosyanın bildirdiği ama listede olmayan
// kimlik H7163. Ailesiz modelde (parçalayıcısı yüklenemeyen GGUF, tabloda olmayan etiket,
// tokenizer_config'siz ya da tokenizer.json'suz klasör) çağrı H7170'tir, dosyanın kimliği
// denetlenmez, TR_eos: [...] ile üretim sürer.

// familyGGUF küçük modelin GGUF'una parçalayıcı anahtarlarını ekler: tek karakterli parçalar,
// sonra özel adlar (kimlikleri 40'tan başlar). declared dosyanın bildirdiği durma kimlikleridir.
func familyGGUF(t *testing.T, dir string, c tinyConfig, pre string, special []string, declared map[string]uint32) string {
	t.Helper()
	return familyGGUFOf(t, dir, c, "gpt2", pre, special, declared)
}

// familyGGUFOf familyGGUF'tur, parçalayıcının türü (tokenizer.ggml.model) verilerek: "llama"
// SentencePiece'li GGUF'tur (Llama 2, Mistral), TR'nin parçalayıcısı onu yüklemez (H6678).
func familyGGUFOf(t *testing.T, dir string, c tinyConfig, kind, pre string, special []string, declared map[string]uint32) string {
	t.Helper()
	b := c.ggufFile(c.randomWeights(1))
	tokens := make([]string, 0, 40+len(special))
	for i := range 40 {
		tokens = append(tokens, string(rune('A'+i)))
	}
	tokens = append(tokens, special...)
	kinds := make([]int32, len(tokens))
	for i := range kinds {
		kinds[i] = 1
		if i >= 40 {
			kinds[i] = 3
		}
	}
	b.KV("tokenizer.ggml.model", kind).KV("tokenizer.ggml.pre", pre).KV("tokenizer.ggml.tokens", tokens).
		KV("tokenizer.ggml.token_type", kinds).KV("tokenizer.ggml.merges", []string{}).KV("tokenizer.ggml.bos_token_id", uint32(0))
	for _, key := range []string{"eos", "eot", "eom"} {
		if id, ok := declared[key]; ok {
			b.KV("tokenizer.ggml."+key+"_token_id", id)
		}
	}
	file := filepath.Join(dir, "m.gguf")
	writeBytes(t, file, b.Bytes())
	return filepath.ToSlash(file)
}

// familyImport betiklerin aktarmasıdır; sabit olarak durur ki tests-for lib/pretrained.tr bu dosyayı bulsun.
const familyImport = `TR_import "tr/pretrained"`

// runFamily m'yi yükler ve gövdeyi koşar; çıktıyı verir.
func runFamily(t *testing.T, dir, load, body string) string {
	t.Helper()
	writeTR(t, dir, "aile.tr", familyImport+"\nTR_var m = "+load+"\n"+body+"\n")
	s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "aile.tr"))
	return strings.TrimSpace(s.Stdout + s.Stderr)
}

var (
	llamaTiny = tinyConfig{arch: "llama", vocab: 48, dim: 64, ff: 96, layers: 1, heads: 4, kv: 2, headDim: 16, context: 32, theta: 10000}
	qwenTiny  = tinyConfig{arch: "qwen3", vocab: 48, dim: 64, ff: 96, layers: 1, heads: 4, kv: 2, headDim: 16, context: 32, theta: 10000}
	llama3    = []string{"<|begin_of_text|>", "<|end_of_text|>", "<|eot_id|>", "<|eom_id|>"}
)

func TestGGUFChatFamilyStopList(t *testing.T) {
	codeOf := "TR_print(TR_error_code { m.TR_eos_tokens() })"
	cases := []struct {
		name     string
		c        tinyConfig
		pre      string
		special  []string
		declared map[string]uint32
		body     string
		want     string
	}{
		// Llama 3'ün üç adı, sohbet tanımının sırasıyla: <|eot_id|> 42, <|eom_id|> 43, <|end_of_text|> 41.
		{"llama3", llamaTiny, "llama-bpe", llama3, map[string]uint32{"eos": 42, "eot": 42, "eom": 43}, "TR_print(m.TR_eos_tokens())", "[42, 43, 41]"},
		{"qwen3", qwenTiny, "qwen2", []string{"<|endoftext|>", "<|im_end|>"}, map[string]uint32{"eos": 41}, "TR_print(m.TR_eos_tokens())", "[41, 40]"},
		{"dosya bildirmiyor", llamaTiny, "llama-bpe", llama3, nil, "TR_print(m.TR_eos_tokens())", "[42, 43, 41]"},
		// Dağarcık dışı eom hata değildir (16.1) ama listede değildir: H7163.
		{"bildirilen listede yok", llamaTiny, "llama-bpe", llama3, map[string]uint32{"eos": 42, "eom": 7}, "", "H7163"},
		// Ailenin ilk adı zorunludur, öbürleri dağarcıkta yoksa atlanır (DENETİM 446 Ş1): <|eom_id|>'siz
		// llama-bpe dağarcığında liste iki öğedir; <|eot_id|>'siz dağarcık H7162'dir.
		{"eom yok", llamaTiny, "llama-bpe", llama3[:3], map[string]uint32{"eos": 42}, "TR_print(m.TR_eos_tokens())", "[42, 41]"},
		{"ilk ad yok", llamaTiny, "llama-bpe", []string{"<|begin_of_text|>", "<|end_of_text|>", "<|eom_id|>"}, nil, "", "H7162"},
		{"tabloda olmayan etiket", llamaTiny, "smollm", llama3, map[string]uint32{"eos": 7}, codeOf, "H7170"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			file := familyGGUF(t, dir, c.c, c.pre, c.special, c.declared)
			load := fmt.Sprintf("TR_pretrained_gguf(%q)", file)
			if c.body == "" {
				load = "TR_none"
				c.body = fmt.Sprintf("TR_print(TR_error_code { TR_pretrained_gguf(%q) })", file)
			}
			if got := runFamily(t, dir, load, c.body); got != c.want {
				t.Errorf("çıktı %q; %q olmalı", got, c.want)
			}
		})
	}
	// H7162 ve H7163 H7160 gibi dosyanın yolunu yazar (DENETİM 446 N11).
	for name, c := range map[string]struct {
		special  []string
		declared map[string]uint32
	}{
		"H7162": {[]string{"<|begin_of_text|>", "<|end_of_text|>"}, nil},
		"H7163": {llama3, map[string]uint32{"eos": 7}},
	} {
		dir := t.TempDir()
		file := familyGGUF(t, dir, llamaTiny, "llama-bpe", c.special, c.declared)
		if got := runFamily(t, dir, fmt.Sprintf("TR_pretrained_gguf(%q)", file), ""); !strings.Contains(got, file) {
			t.Errorf("%s iletisi dosyanın yolunu yazmıyor:\n%s", name, got)
		}
	}
}

// TestChatTableIsUnique: sohbet tanımı tablosunda (mimari, etiket) ve (mimari, işaret) ikilileri
// tekildir; seçim ilk eşleşmede durur (DENETİM 446 N5). Tablo kitaplığın özel fonksiyonudur, TR
// testinden görünmez: test kaynağın satırlarını okur.
func TestChatTableIsUnique(t *testing.T) {
	src, err := os.ReadFile(reporoot.Path("lib", "pretrained.tr"))
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile(`_Sohbet\("([^"]*)", "([^"]*)", "([^"]*)", "([^"]*)", \[`).FindAllStringSubmatch(string(src), -1)
	if len(rows) < 2 {
		t.Fatalf("tablonun satırları okunamadı: %d", len(rows))
	}
	seen := map[string]string{}
	for _, r := range rows {
		for _, key := range []string{"etiket " + r[2] + " " + r[3], "işaret " + r[2] + " " + r[4]} {
			if prev, dup := seen[key]; dup {
				t.Errorf("%s iki ailede: %s ve %s", key, prev, r[1])
			}
			seen[key] = r[1]
		}
	}
}

// TestFamilylessModelGeneratesWithExplicitStops: yedi ailesiz yolda (parçalayıcısız GGUF, tabloda
// olmayan etiket, parçalayıcının tanımadığı etiket, SentencePiece'li GGUF, etiketi mimarisiyle
// çelişen GGUF, tokenizer_config'siz klasör, tokenizer.json'suz klasör) model.TR_eos_tokens()
// H7170'tir, ileti kimliklerin açık yazımını söyler; TR_eos: [...] ile üretim sürer. GGUF'ta
// parçalayıcı yüklenemezse model ailesizdir; hatayı yalnız TR_load_tokenizer verir, işlevin adı ve
// dosyanın yoluyla (H5620'nin içinde). Okunamayan dosya yine hatadır (DENETİM 453 Ş1, N2).
func TestFamilylessModelGeneratesWithExplicitStops(t *testing.T) {
	body := "TR_print(TR_error_code { m.TR_eos_tokens() })\nTR_print(m.TR_generate([1, 2], 3, TR_temperature: 0, TR_eos: [5]).TR_length > 2)"
	dir := t.TempDir()
	bare := filepath.Join(dir, "yalın.gguf")
	writeBytes(t, bare, llamaTiny.ggufFile(llamaTiny.randomWeights(1)).Bytes())
	folder := filepath.Join(dir, "klasör")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	llamaTiny.writeHF(t, folder, llamaTiny.randomWeights(1))
	// Sohbet tanımı bulunan ama tokenizer.json'u olmayan klasör de ailesizdir (DENETİM 446 Ş2).
	noVocab := filepath.Join(dir, "sözlüksüz")
	if err := os.MkdirAll(noVocab, 0o755); err != nil {
		t.Fatal(err)
	}
	qwenTiny.writeHF(t, noVocab, qwenTiny.randomWeights(1))
	writeBytes(t, filepath.Join(noVocab, "tokenizer_config.json"), []byte(`{"tokenizer_class": "Qwen2TokenizerFast"}`))
	unknownTag := familyGGUF(t, t.TempDir(), llamaTiny, "smaug-bpe", llama3, nil)
	loads := map[string]string{
		"parçalayıcısız GGUF":        fmt.Sprintf("TR_pretrained_gguf(%q)", filepath.ToSlash(bare)),
		"smollm etiketi":             fmt.Sprintf("TR_pretrained_gguf(%q)", familyGGUF(t, t.TempDir(), llamaTiny, "smollm", llama3, nil)),
		"tanınmayan etiket":          fmt.Sprintf("TR_pretrained_gguf(%q)", unknownTag),
		"SentencePiece'li GGUF":      fmt.Sprintf("TR_pretrained_gguf(%q)", familyGGUFOf(t, t.TempDir(), llamaTiny, "llama", "llama-bpe", llama3, nil)),
		"mimarisiyle çelişen etiket": fmt.Sprintf("TR_pretrained_gguf(%q)", familyGGUF(t, t.TempDir(), llamaTiny, "qwen2", llama3, nil)),
		"klasör":                     fmt.Sprintf("TR_pretrained_model(%q)", filepath.ToSlash(folder)),
		"tokenizer.json'suz klasör":  fmt.Sprintf("TR_pretrained_model(%q)", filepath.ToSlash(noVocab)),
	}
	for name, load := range loads {
		t.Run(name, func(t *testing.T) {
			if got := runFamily(t, dir, load, body); got != "H7170\ntrue" {
				t.Errorf("çıktı %q", got)
			}
		})
	}
	got := runFamily(t, dir, loads["klasör"], "m.TR_eos_tokens()")
	if !strings.Contains(got, "`TR_eos: [...]`") || !strings.Contains(got, "llama3, qwen3") {
		t.Errorf("H7170'in iletisi aileleri ve açık yazımı söylemiyor:\n%s", got)
	}
	// Hatayı TR_load_tokenizer verir: H5620, işlevin adı, dosyanın yolu ve etiket.
	got = runFamily(t, dir, loads["tanınmayan etiket"], fmt.Sprintf("TR_try { TR_load_tokenizer(%q) } TR_catch e {\n    TR_print(e.TR_code)\n    TR_print(e.TR_text(\"tr\"))\n}", unknownTag))
	if lines := strings.Split(got, "\n"); len(lines) != 2 || lines[0] != "H5620" || !strings.Contains(lines[1], "parçalayıcı_yükle") ||
		!strings.Contains(lines[1], unknownTag) || !strings.Contains(lines[1], "smaug-bpe") {
		t.Errorf("tanınmayan etiketin parçalayıcısı:\n%s", got)
	}
	// Okunamayan dosya ailesiz sayılmaz: yükleme hatadır (H2107; TR_error_code en içteki nedeni,
	// "dosya yok"u, H2132'yi verir).
	missing := filepath.ToSlash(filepath.Join(dir, "yok.gguf"))
	if got := runFamily(t, dir, fmt.Sprintf("TR_error_code { TR_pretrained_gguf(%q) }", missing), "TR_print(m)"); got != "H2132" {
		t.Errorf("okunamayan dosya: %q", got)
	}
}

// TestFolderChatFamily: klasörde aile (model_type, tokenizer_class'ın Fast'sız adı) ile seçilir;
// kimlikler klasörün tokenizer.json'undan, bildirilen kimlik generation_config.json'dan, o yoksa
// config.json'un eos_token_id'sinden (DENETİM 446 N4).
func TestFolderChatFamily(t *testing.T) {
	tok, err := os.ReadFile(reporoot.Path("tests", "data", "qwen3_chat", "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, eos, want string }{
		{"liste", "[573, 571]", "[573, 571]"},
		{"tek sayı", "573", "[573, 571]"},
		{"listede yok", "[573, 7]", "H7163"},
		{"config.json'dan", "config:573", "[573, 571]"},
		{"config.json'da listede yok", "config:7", "H7163"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			qwenTiny.writeHF(t, dir, qwenTiny.randomWeights(1))
			writeBytes(t, filepath.Join(dir, "tokenizer.json"), tok)
			writeBytes(t, filepath.Join(dir, "tokenizer_config.json"), []byte(`{"tokenizer_class": "Qwen2TokenizerFast"}`))
			if id, inConfig := strings.CutPrefix(c.eos, "config:"); inConfig {
				addConfigKey(t, filepath.Join(dir, "config.json"), "eos_token_id", id)
			} else {
				writeBytes(t, filepath.Join(dir, "generation_config.json"), []byte(`{"eos_token_id": `+c.eos+`}`))
			}
			load := fmt.Sprintf("TR_pretrained_model(%q)", filepath.ToSlash(dir))
			body := "TR_print(m.TR_eos_tokens())"
			if c.want == "H7163" {
				load, body = "TR_none", fmt.Sprintf("TR_print(TR_error_code { TR_pretrained_model(%q) })", filepath.ToSlash(dir))
			}
			if got := runFamily(t, dir, load, body); got != c.want {
				t.Errorf("çıktı %q; %q olmalı", got, c.want)
			}
		})
	}
}

// addConfigKey config.json'a bir anahtar ekler (değer JSON metni olarak).
func addConfigKey(t *testing.T, path, key, value string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.TrimRight(string(data), " \n")
	if !strings.HasSuffix(s, "}") {
		t.Fatalf("config.json bir nesneyle bitmiyor: %q", s)
	}
	writeBytes(t, path, []byte(s[:len(s)-1]+fmt.Sprintf(", %q: %s}", key, value)))
}

// TestGenerateEosMethodWithoutParentheses: TR_eos:'a parantezsiz metot (fonksiyon değeri) verilince
// ileti parantezli yazımı önerir (H7173, `model.`: DENETİM 446 N1); önerilen yazım ailesi olan
// modelde çalışır ve modelin durma listesini verir (DENETİM 440 N1).
func TestGenerateEosMethodWithoutParentheses(t *testing.T) {
	dir := t.TempDir()
	file := familyGGUF(t, dir, llamaTiny, "llama-bpe", llama3, nil)
	load := fmt.Sprintf("TR_pretrained_gguf(%q)", file)
	got := runFamily(t, dir, load, "TR_var model = m\nmodel.TR_generate([1], 2, TR_temperature: 0, TR_eos: model.TR_eos_tokens)")
	if !strings.Contains(got, "`TR_eos: model.TR_eos_tokens()`") {
		t.Fatalf("ileti parantezli yazımı önermiyor:\n%s", got)
	}
	if got := runFamily(t, dir, load, "TR_var model = m\nTR_print(model.TR_eos_tokens())\nTR_print(model.TR_generate([1], 2, TR_temperature: 0, TR_eos: model.TR_eos_tokens()).TR_length > 1)"); got != "[42, 43, 41]\ntrue" {
		t.Errorf("önerilen yazım: %q", got)
	}
}

// TestGenerateEosItemsAreInts: TR_eos:'un her öğesi Tam'dır; Tam olmayan öğe (dağarcıkta olmayan
// adın p.TR_token_id'sinin TR_none'ı, metin) ve liste olmayan TR_eos: TR_none H7174'tür; ileti
// öğenin sırasını ve türünü, kimliğin p.TR_token_id(...) ile alındığını söyler (DENETİM 446 N2).
// Tür iletiye kimliğiyle girer, her dilde kendi görünüm adıyla yazılır: Türkçede YokTipi ve Metin,
// İngilizcede NoneType ve String; düz metin olarak sızmaz (TCB 2026-11-11, kapanış öncesi). Yokluk
// değeri kimlik yuvasıdır (`TR_none`, her dilde kimlik yazımı); ileti modelin listesini de önerir.
func TestGenerateEosItemsAreInts(t *testing.T) {
	dir := t.TempDir()
	load := "TR_GPT(6, TR_context: 8, TR_embed_dim: 8, TR_num_layers: 1, TR_num_heads: 2, TR_seed: 2)"
	for _, c := range []struct{ eos, tr, en string }{
		{"[1, TR_none]", "2. öğe YokTipi, Tam değil", "item 2 is a NoneType, not an Int"},
		{`["a"]`, "1. öğe Metin, Tam değil", "item 1 is a String, not an Int"},
		{"TR_none", "liste değil, YokTipi verildi", "a NoneType was given, not a list"},
	} {
		got := runFamily(t, dir, load, "TR_try { m.TR_generate([1], 2, TR_temperature: 0, TR_eos: "+c.eos+") } TR_catch e {\n"+
			"    TR_print(e.TR_code)\n    TR_print(e.TR_text(\"tr\"))\n    TR_print(e.TR_text(\"en\"))\n}")
		lines := strings.Split(got, "\n")
		if len(lines) != 3 || lines[0] != "H7174" {
			t.Fatalf("TR_eos: %s:\n%s", c.eos, got)
		}
		// Yokluk değeri de kimlik yuvasıdır ({#none}): düz sözcük `none` yazılmaz (TCB 20:38).
		if !strings.Contains(lines[1], c.tr) || strings.Contains(lines[1], "NoneType") || strings.Contains(lines[1], "`none`") ||
			!strings.Contains(lines[1], "`TR_none`") || !strings.Contains(lines[1], "`p.TR_token_id(\"...\")`") {
			t.Errorf("TR_eos: %s, Türkçe ileti:\n%s", c.eos, lines[1])
		}
		if !strings.Contains(lines[2], c.en) || strings.Contains(lines[2], "`none`") || !strings.Contains(lines[2], "`TR_none`") ||
			!strings.Contains(lines[2], "`p.TR_token_id(\"...\")`") {
			t.Errorf("TR_eos: %s, İngilizce ileti:\n%s", c.eos, lines[2])
		}
		for _, l := range lines[1:] {
			if !strings.Contains(l, "`TR_eos: model.TR_eos_tokens()`") {
				t.Errorf("TR_eos: %s: ileti modelin listesini önermiyor:\n%s", c.eos, l)
			}
		}
	}
	// Önerilen yazım ailesi olan modelde koşar ve modelin listesiyle üretir (DENETİM 453 N1).
	fam := fmt.Sprintf("TR_pretrained_gguf(%q)", familyGGUF(t, dir, llamaTiny, "llama-bpe", llama3, nil))
	if got := runFamily(t, dir, fam, "TR_var model = m\nTR_print(TR_error_code { model.TR_generate([1], 2, TR_eos: [TR_none]) })\n"+
		"TR_print(model.TR_generate([1], 2, TR_temperature: 0, TR_eos: model.TR_eos_tokens()).TR_length > 1)"); got != "H7174\ntrue" {
		t.Errorf("önerilen yazım: %q", got)
	}
}
