package e2e

import (
	"github.com/sekerahmet/TCB/internal/lang/source"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kitaplık kapsamı: lib/*.tr'nin kullanıcıya açık adları (üst düzey fonksiyon, yapı ve sabitler ile yapıların
// metotları; `_` ile başlayanlar ve kurucu hariç) examples/tr altındaki bir örnekte geçer. Ad kapsamı testinin
// (behavior_examples_coverage_test.go) kitaplık karşılığıdır: oradaki adlar dilin kodlarıdır, buradakiler TR ile
// yazılmış kitaplığın adlarıdır. Üst düzey ad "modül.ad", metot "modül.ad()" anahtarıyla yazılır.

// uncoveredLibraryNames, örneği henüz olmayan ya da örnek gerektirmeyen kitaplık adlarıdır ve gerekçesiyle durur.
// Liste yalnızca küçülür: örneği olan bir ad listede kalırsa test düşer.
var uncoveredLibraryNames = map[string]string{
	"layers.göm()":                     "iç yardımcı metot: GPT.gizli_temsil ve tek_adım ortak gövdesi",
	"layers.blokları_işle()":           "iç yardımcı metot: GPT.gizli_temsil ve tek_adım ortak gövdesi",
	"layers.tampon_ekle()":             "iç yardımcı metot: KVÖnbellek.ekle'nin anahtar ve değer tamponları için ortak gövdesi",
	"layers.ağırlık_başlat":            "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.Doğrusal":                  "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.GömmeKatmanı":              "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.RMSNormKatmanı":            "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.döner_konum_tablosu":       "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.döner_konum_uygula":        "iç yardımcı: lib/README.md'de belgelenmemiş",
	"layers.NedenselDikkat":            "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.swiglu_ara_boyutu":         "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.swiglu_kapısı":             "iç yardımcı: lib/README.md'de belgelenmemiş",
	"layers.swiglu_birleşik_kapısı":    "iç yardımcı: lib/README.md'de belgelenmemiş",
	"layers.SwiGLU":                    "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.ple_gelu":                  "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.ple_karışımı":              "iç yardımcı: lib/README.md'de belgelenmemiş",
	"layers.PLEBağlamı":                "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.PLEBloğu":                  "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.TransformerBloğu":          "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.katman_pencereleri":        "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.KVÖnbellek":                "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"layers.cihaz_adımı()":             "iç yardımcı: lib/README.md'de belgelenmemiş",
	"layers.maske()":                   "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"optimizers.NS_A":                  "iç yardımcı: lib/README.md'de belgelenmemiş",
	"optimizers.NS_B":                  "iç yardımcı: lib/README.md'de belgelenmemiş",
	"optimizers.NS_C":                  "iç yardımcı: lib/README.md'de belgelenmemiş",
	"optimizers.dikleştir":             "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"optimizers.şekil_grupları":        "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"optimizers.yığın_dikleştir":       "iç yardımcı: lib/README.md'de belgelenmemiş",
	"optimizers.durum_tensörü":         "iç yardımcı: lib/README.md'de belgelenmemiş",
	"optimizers.gradyanları_sıfırla":   "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"optimizers.yalnızca_2b":           "iç yardımcı: lib/README.md'de belgelenmemiş",
	"optimizers.ad_uyar_mı":            "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"optimizers.bir_desen_uyar_mı":     "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"optimizers.parametre_grupları":    "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"optimizers.parametre_kümeleri":    "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"optimizers.SatırRMS":              "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"training.EN_AZ_GÖRELİ_SAPMA":      "iç yardımcı: lib/README.md'de belgelenmemiş",
	"training.aşıyor()":                "iç yardımcı: lib/README.md'de belgelenmemiş",
	"training.geçmiş_uzunluğu()":       "belgeli ad; yazılacak: sıçrama bekçisinin ayrıntıları (44 yalnız TAMAM ile karşılaştırır)",
	"pretrained.DESTEKLENEN_MİMARİLER": "iç yardımcı: lib/README.md'de belgelenmemiş",
	"pretrained.ayar_değeri":           "iç yardımcı: lib/README.md'de belgelenmemiş",
	"pretrained.hazır_model_kur":       "belgeli ad; yazılacak: hazır modelin iç yapısı (hazır_model_kur, eşleme) örneği",
	"pretrained.rope_tabanı":           "belgeli ad; yazılacak: hazır modelin iç yapısı (hazır_model_kur, eşleme) örneği",
	"pretrained.rope_ölçekleme_var_mı": "iç yardımcı: lib/README.md'de belgelenmemiş",
	"pretrained.BLOK_ADLARI":           "iç yardımcı: lib/README.md'de belgelenmemiş",
	"pretrained.hugging_face_eşle":     "belgeli ad; yazılacak: hazır modelin iç yapısı (hazır_model_kur, eşleme) örneği",
}

var (
	libTopLevel = regexp.MustCompile(`(?m)^(?:fonksiyon|yapı|sabit)\s+([\p{L}\p{N}_]+)`)
	libMethod   = regexp.MustCompile(`(?m)^    fonksiyon\s+([\p{L}\p{N}_]+)`)
	lineComment = regexp.MustCompile(`//[^\n]*`)
)

// libraryNames kitaplık modüllerinin kullanıcıya açık adlarını anahtar biçimiyle verir.
func libraryNames(t *testing.T) []string {
	t.Helper()
	files, err := reporoot.Glob(reporoot.Path("lib", "*.tr"))
	if err != nil || len(files) == 0 {
		t.Fatalf("lib/*.tr bulunamadı: %v", err)
	}
	var names []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.tr") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		module := strings.TrimSuffix(filepath.Base(f), ".tr")
		view, err := source.Show(string(data), source.Turkish()) // kodlu kitaplık Türkçe görünümüyle okunur
		if err != nil {
			t.Fatal(err)
		}
		text := lineComment.ReplaceAllString(view, "")
		for _, m := range libTopLevel.FindAllStringSubmatch(text, -1) {
			if !strings.HasPrefix(m[1], "_") {
				names = append(names, module+"."+m[1])
			}
		}
		seen := map[string]bool{}
		for _, m := range libMethod.FindAllStringSubmatch(text, -1) {
			if m[1] != "kurucu" && !strings.HasPrefix(m[1], "_") && !seen[m[1]] {
				seen[m[1]] = true
				names = append(names, module+"."+m[1]+"()")
			}
		}
	}
	return names
}

// exampleCode örneklerin yorum satırları atılmış metnidir.
func exampleCode(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for name, text := range exampleTexts(t) {
		if strings.Contains(name, "/") {
			continue // hata örnekleri ve yardımcı modüller
		}
		b.WriteString(lineComment.ReplaceAllString(text, ""))
		b.WriteString("\n")
	}
	return b.String()
}

func TestExamplesCoverLibraryNames(t *testing.T) {
	code := exampleCode(t)
	live := map[string]bool{}
	var missing, stale []string
	for _, key := range libraryNames(t) {
		live[key] = true
		name := key[strings.Index(key, ".")+1:]
		// Go'nun \b'si ASCII'dir; Türkçe harfli adlar için sınır açıkça yazılır.
		const edge = `[^\p{L}\p{N}_]`
		pattern := `(^|` + edge + `)` + regexp.QuoteMeta(name) + `($|` + edge + `)`
		if strings.HasSuffix(name, "()") {
			pattern = `\.` + regexp.QuoteMeta(strings.TrimSuffix(name, "()")) + `($|` + edge + `)`
		}
		used := regexp.MustCompile(pattern).MatchString(code)
		_, excepted := uncoveredLibraryNames[key]
		switch {
		case !used && !excepted:
			missing = append(missing, key)
		case used && excepted:
			stale = append(stale, key)
		}
	}
	for key := range uncoveredLibraryNames {
		if !live[key] {
			stale = append(stale, key+" (kitaplıkta yok)")
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("hiçbir örnekte geçmeyen %d kitaplık adı (örnek yazın ya da gerekçesiyle uncoveredLibraryNames'e ekleyin): %s", len(missing), strings.Join(missing, ", "))
	}
	if len(stale) > 0 {
		t.Errorf("uncoveredLibraryNames'ten çıkarılmalı (artık örnekte geçiyor ya da kitaplıkta yok): %s", strings.Join(stale, ", "))
	}
}
