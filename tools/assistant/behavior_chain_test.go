package assistant_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Zincir (asistan.tr) dizüstünde kişisel yollarla koşar; depoya yol ve aygıt adı girmez (S1 zincir yönergesi, B.1).
// Davranışı tarayıcı değil trlang sınar: modelsiz parçalar zincir_test.tr'de.

const (
	chainProgram = "tools/assistant/asistan.tr"
	chainModule  = "tools/assistant/zincir.tr"
	chainTest    = "tools/assistant/zincir_test.tr"
	familyModule = "tools/assistant/aile.tr"
	familyTest   = "tools/assistant/aile_test.tr"
	quantModule  = "tools/assistant/nicemli.tr"
	chainGoTest  = "behavior_chain_test.go"
	screenModule = "tools/assistant/ekran.tr"
	screenTest   = "tools/assistant/ekran_test.tr"
	fakeChain    = "tools/assistant/screen_demo.tr"
	recordModule = "tools/assistant/deneme.tr"
	recordTest   = "tools/assistant/deneme_test.tr"
)

// Programın varsayılan model yolu ya da aygıtı yoktur: verilmezse hata verir, başka bir makinenin yolunu sessizce açmaz.
func TestChainHasNoDefaultModelOrDevice(t *testing.T) {
	program := readFile(t, chainProgram)
	for _, option := range []string{"kip", "girdi", "çıktı", "dil_modeli", "whisper", "vad", "ses_modeli", "aygıt", "sahte_mikrofon"} {
		if !regexp.MustCompile(`"` + option + `": "",`).MatchString(program) {
			t.Errorf("%s: option %q must default to the empty text", chainProgram, option)
		}
	}
	if regexp.MustCompile(`(?i)[a-z]:[/\\]|/users/|/home/`).MatchString(program) {
		t.Errorf("%s holds a local path", chainProgram)
	}
}

// Zincirin, ekranın ve deneme kaydının TR dosyaları README'de anılır; test dosyaları modüllerini aktarır.
func TestChainFilesAreListed(t *testing.T) {
	readme := readFile(t, readmePath)
	for _, f := range []string{chainProgram, chainModule, chainTest, familyModule, familyTest, quantModule, screenModule, screenTest, fakeChain, recordModule, recordTest} {
		name := f[strings.LastIndex(f, "/")+1:]
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("%s does not name %s", readmePath, name)
		}
	}
	for _, f := range []string{chainModule, chainProgram, familyTest} {
		if !strings.Contains(readFile(t, f), `TR_import "aile"`) {
			t.Errorf("%s does not import the family module", f)
		}
	}
	if !strings.Contains(readme, "- `"+chainGoTest+"`: ") {
		t.Errorf("%s does not list %s", readmePath, chainGoTest)
	}
	if !strings.Contains(readFile(t, chainTest), `TR_import "zincir"`) {
		t.Errorf("%s does not import the chain module", chainTest)
	}
	if !strings.Contains(readFile(t, chainProgram), `TR_import "zincir"`) {
		t.Errorf("%s does not import the chain module", chainProgram)
	}
	for _, f := range []string{chainModule, chainTest, chainProgram} {
		if !strings.Contains(readFile(t, f), `TR_import "nicemli"`) {
			t.Errorf("%s does not import the quantized-option module", f)
		}
	}
	if !strings.Contains(readFile(t, screenTest), `TR_import "ekran"`) {
		t.Errorf("%s does not import the screen module", screenTest)
	}
	for _, f := range []string{recordTest, chainProgram} {
		if !strings.Contains(readFile(t, f), `TR_import "deneme"`) {
			t.Errorf("%s does not import the record module", f)
		}
	}
}

// Dil modelinin her çağrısı zincir.tr'nin model_çağrısı'ndan geçer (H1 nicemli zincir, K2): bloğun dışında kalan bir
// çağrı hata vermez, yalnız nicemli seçenekte yavaş kalır. Desen öneksizdir; TR_vendor_matmul bloğu yalnız geçittedir.
func TestChainModelCallsGoThroughOneGate(t *testing.T) {
	files, err := reporoot.Glob(reporoot.Path("tools/assistant", "*.tr"))
	if err != nil {
		t.Fatal(err)
	}
	gates := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.tr") {
			continue
		}
		rel := "tools/assistant/" + filepath.Base(f)
		inGate := false
		for i, line := range strings.Split(readFile(t, rel), "\n") {
			if strings.HasPrefix(line, "TR_func model_çağrısı(") {
				inGate = true
			} else if line == "}" {
				inGate = false
			}
			if (strings.Contains(line, "TR_hidden_states(") || strings.Contains(line, "TR_logits(")) && !strings.Contains(line, "model_çağrısı(") {
				t.Errorf("%s:%d: dil modelinin çağrısı model_çağrısı'ndan geçmiyor: %s", rel, i+1, strings.TrimSpace(line))
			}
			if strings.Contains(line, "TR_vendor_matmul {") {
				if !inGate {
					t.Errorf("%s:%d: TR_vendor_matmul bloğu model_çağrısı'nın dışında: %s", rel, i+1, strings.TrimSpace(line))
				}
				gates++
			}
		}
	}
	if gates == 0 {
		t.Errorf("%s: model_çağrısı'nda TR_vendor_matmul bloğu yok", chainModule)
	}
}

// body: metinde `head` ile başlayan satırdan sütun 0'daki ilk `}`'e kadarki gövde.
func body(t *testing.T, file, text, head string) string {
	t.Helper()
	i := strings.Index(text, "\n"+head)
	if i < 0 {
		t.Fatalf("%s: %q yok", file, head)
	}
	rest := text[i+1:]
	if j := strings.Index(rest, "\n}\n"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// before: `a` ve `b` metinde geçer, `a` önce.
func before(t *testing.T, where, text, a, b string) {
	t.Helper()
	i, j := strings.Index(text, a), strings.Index(text, b)
	if i < 0 || j < 0 || i > j {
		t.Errorf("%s: %q, %q'dan önce geçmeli (konumlar %d, %d)", where, a, b, i, j)
	}
}

// Nicemli seçeneğin kurulum denetimleri (H1 nicemli zincir, K3): cihaz modeller yüklenmeden önce; ısıtma üretimin
// istem okumasını türevsiz koşar ve nicemli hatayı yutmadan önce çevirir; önbelleksiz kipin sınaması ısıtmadan sonra.
// Ağırlık türünün denetimi (sürüm 9, DENETİM 549 N1) dil modelinden hemen sonra, Whisper'dan önce. Otomatik seçim
// (dilim N) dil modeli yüklenince, zincir kurulmadan çözülür; çözülen hâl NICEMLI satırında ve her turun Tur'unda.
// Davranışı zincir_test.tr sınar (q8 çıkış katmanlı model; float ve q4_k ağırlıklı model).
func TestChainQuantChecksOrder(t *testing.T) {
	setup := body(t, chainProgram, readFile(t, chainProgram), "TR_func asistan_kur(")
	before(t, chainProgram+" asistan_kur", setup, "nicemli_hatası(", "TR_pretrained_gguf(")
	before(t, chainProgram+" asistan_kur", setup, "TR_pretrained_gguf(", "nicemli_ağırlık_hatası(")
	before(t, chainProgram+" asistan_kur", setup, "nicemli_ağırlık_hatası(", "TR_pretrained_whisper(")
	before(t, chainProgram+" asistan_kur", setup, "ısıt(", "nicemli_hatasını_at(")
	before(t, chainProgram+" asistan_kur", setup, "TR_pretrained_gguf(", "nicemli_çöz(")
	before(t, chainProgram+" asistan_kur", setup, "nicemli_çöz(", "Zincir(")
	before(t, chainProgram+" asistan_kur", setup, "OLCU_YUKLEME", "nicemli_satırı(")
	// Ekranlı kipte model oturum içinde yeniden yüklenir: kaydın tur satırı o turun zincirinin hâlidir (DENETİM 774 Ş1).
	if open := body(t, chainProgram, readFile(t, chainProgram), "TR_func tur_aç("); !strings.Contains(open, "nicemli: z.nicemli") {
		t.Errorf("%s tur_aç: Tur'a nicemli: z.nicemli verilmeli", chainProgram)
	}
	// Seçenek zincire geçmezse blok hiç açılmaz, açılış denetimleri yine geçer ve NICEMLI satırı yanlış yazar (DENETİM 553 N1).
	if !strings.Contains(setup, "nicemli: nicemli") {
		t.Errorf("%s asistan_kur: Zincir'e nicemli: nicemli verilmeli", chainProgram)
	}
	chain := readFile(t, chainModule)
	warm := body(t, chainModule, chain, "TR_func ısıt(")
	if !strings.Contains(warm, "TR_no_grad { istem_oku(") {
		t.Errorf("%s ısıt: TR_try istem_oku'yu TR_no_grad'da çağırmalı", chainModule)
	}
	catch := warm[max(strings.Index(warm, "TR_catch"), 0):]
	before(t, chainModule+" ısıt'ın TR_catch'i", catch, "nicemli_hatasını_at(", "ISITMA HATASI")
	if !strings.Contains(body(t, chainModule, chain, "TR_func üret("), "istem_oku(") {
		t.Errorf("%s üret: istem istem_oku ile okunmalı", chainModule)
	}
}

// Sohbet biçimi modelin ailesinindir (dost-katalog tasarımı K-4, K-5, K-8): aileyi ve durma kimliklerini kitaplık seçer
// (15.55; ailesiz modelde TR_eos_tokens() H7170'tir), aile adımı dil modelinden hemen sonra, parçalayıcıdan ve
// zincirden önce gelir. Biçime bağlı adlar (biçim, cevap, etiket, başlık) yalnız aile.tr'dedir: programda ve zincirde
// kalan bir ad Gemma 3'te sessizce yanlış biçim olurdu. Davranışı aile_test.tr ve cmd/trlang/e2e'nin
// behavior_assistant_family_test.go'su sınar.
func TestChainFamilyComesFromTheLibrary(t *testing.T) {
	setup := body(t, chainProgram, readFile(t, chainProgram), "TR_func asistan_kur(")
	before(t, chainProgram+" asistan_kur", setup, "TR_pretrained_gguf(", "gguf_ailesi(")
	before(t, chainProgram+" asistan_kur", setup, "gguf_ailesi(", "TR_load_tokenizer(")
	before(t, chainProgram+" asistan_kur", setup, "gguf_ailesi(", "Zincir(")
	if !strings.Contains(setup, "aile: aile, durma: durma") {
		t.Errorf("%s asistan_kur: Zincir'e aile: aile, durma: durma verilmeli", chainProgram)
	}
	// Dost katalog K-9: önbellek yüklenen modelin sınırını aşmaz (aile.tr, önbellek_bağlamı; aile_test.tr sınar).
	if !strings.Contains(setup, "bağlam: önbellek_bağlamı(bağlam, model)") {
		t.Errorf("%s asistan_kur: Zincir'in bağlamı önbellek_bağlamı(bağlam, model) olmalı", chainProgram)
	}
	family := body(t, familyModule, readFile(t, familyModule), "TR_func gguf_ailesi(")
	before(t, familyModule+" gguf_ailesi", family, "model.TR_eos_tokens()", "TR_load_metadata(")
	if !strings.Contains(family, "aile_denetle(") {
		t.Errorf("%s gguf_ailesi: mimari aile_denetle'den geçmeli (H8170)", familyModule)
	}
	for _, f := range []string{chainProgram, chainModule} {
		text := readFile(t, f)
		for _, name := range []string{"TR_qwen3_", "TR_gemma3_", "<|im_end|>", "<|endoftext|>", "<end_of_turn>", "<start_of_turn>", "<think>", "<tool_call>", `"assistant\n"`, `"model\n"`} {
			if strings.Contains(text, name) {
				t.Errorf("%s: %q aile.tr'de olmalı", f, name)
			}
		}
	}
}

// Otomatik nicemlinin kararı nicemli.tr'nin elle tutulan katman listesine dayanır (dilim N, DENETİM 774 N3): modelin
// her TR_Linear alanı (lib/layers.tr'deki dikkat, ileri katman, PLE ve GPT yapıları) listede kendi yapısının
// değişkeniyle okunur: TR_gate_proj hem TR_SwiGLU'da hem TR_PLEBlock'ta, ikisi ayrı aranır (DENETİM 794 N1). Liste bir
// çarpımı kaçırırsa otomatik q8 bir izdüşümü görmez, açar ve ısıtmanın H6723'ü Dost'un hatası olarak çıkar (N-S3b).
func TestQuantListCoversEveryLinear(t *testing.T) {
	layers := readFile(t, "lib/layers.tr")
	list := body(t, quantModule, readFile(t, quantModule), "TR_func _çarpım_türleri(")
	field := regexp.MustCompile(`(?m)^    (TR_[a-z_]+): TR_Linear\??\s*(?:=|$|#)`)
	// Yapının listedeki değişkeni: d dikkat, f ileri katman, b.TR_ple bloğun PLE'si, m.TR_ple_context ve m modelin kendisi.
	owner := map[string]string{"TR_CausalAttention": "d.", "TR_SwiGLU": "f.", "TR_PLEContext": "m.TR_ple_context.",
		"TR_PLEBlock": "b.TR_ple.", "TR_TransformerBlock": "b.", "TR_GPT": "m."}
	seen := 0
	for _, name := range []string{"TR_CausalAttention", "TR_SwiGLU", "TR_PLEContext", "TR_PLEBlock", "TR_TransformerBlock", "TR_GPT"} {
		block := capture(t, `(?s)\nTR_struct `+name+` \{\n(.*?)\n    TR_func `, layers)
		for _, m := range field.FindAllStringSubmatch(block, -1) {
			seen++
			if !strings.Contains(list, owner[name]+m[1]) {
				t.Errorf("%s _çarpım_türleri: %s.%s (lib/layers.tr) listede %s%s olarak yok", quantModule, name, m[1], owner[name], m[1])
			}
		}
	}
	if seen < 12 {
		t.Errorf("lib/layers.tr'de %d TR_Linear alanı bulundu, en az 12 bekleniyordu: desen yapıları okuyamıyor", seen)
	}
}
