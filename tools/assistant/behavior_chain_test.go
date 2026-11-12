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
	for _, f := range []string{chainProgram, chainModule, chainTest, quantModule, screenModule, screenTest, fakeChain, recordModule, recordTest} {
		name := f[strings.LastIndex(f, "/")+1:]
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("%s does not name %s", readmePath, name)
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
// Ağırlık türünün denetimi (sürüm 9, DENETİM 549 N1) dil modelinden hemen sonra, Whisper'dan önce.
// Davranışı zincir_test.tr sınar (q8 çıkış katmanlı model; float ve q4_k ağırlıklı model).
func TestChainQuantChecksOrder(t *testing.T) {
	setup := body(t, chainProgram, readFile(t, chainProgram), "TR_func asistan_kur(")
	before(t, chainProgram+" asistan_kur", setup, "nicemli_hatası(", "TR_pretrained_gguf(")
	before(t, chainProgram+" asistan_kur", setup, "TR_pretrained_gguf(", "nicemli_ağırlık_hatası(")
	before(t, chainProgram+" asistan_kur", setup, "nicemli_ağırlık_hatası(", "TR_pretrained_whisper(")
	before(t, chainProgram+" asistan_kur", setup, "ısıt(", "nicemli_hatasını_at(")
	// Seçenek zincire geçmezse blok hiç açılmaz, açılış denetimleri yine geçer ve deneme başlığı "true" yazar (DENETİM 553 N1).
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
