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
	"katman.gömülü()":                  "iç yardımcı metot: GPT.gizli_durum ve tek_adım ortak gövdesi",
	"katman.blokları_işle()":           "iç yardımcı metot: GPT.gizli_durum ve tek_adım ortak gövdesi",
	"katman.tampon_ekle()":             "iç yardımcı metot: KVÖnbellek.ekle'nin anahtar ve değer tamponları için ortak gövdesi",
	"katman.ağırlık_başlat":            "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.Doğrusal":                  "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.GömmeKatmanı":              "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.RMSNormKatmanı":            "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.döner_konum_tablosu":       "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.döner_konum_hesapla":       "iç yardımcı: lib/README.md'de belgelenmemiş",
	"katman.NedenselDikkat":            "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.swiglu_ara_boyutu":         "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.swiglu_kapısı":             "iç yardımcı: lib/README.md'de belgelenmemiş",
	"katman.swiglu_birleşik_kapısı":    "iç yardımcı: lib/README.md'de belgelenmemiş",
	"katman.SwiGLU":                    "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.ple_gelu":                  "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.ple_karışımı":              "iç yardımcı: lib/README.md'de belgelenmemiş",
	"katman.PLEBağlamı":                "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.PLEBloğu":                  "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.TransformerBloğu":          "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.katman_pencereleri":        "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.KVÖnbellek":                "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"katman.cihaz_adımı()":             "iç yardımcı: lib/README.md'de belgelenmemiş",
	"katman.maske()":                   "belgeli ad; yazılacak: katman yapı taşlarıyla elle model kurma örneği",
	"iyilestirici.NS_A":                "iç yardımcı: lib/README.md'de belgelenmemiş",
	"iyilestirici.NS_B":                "iç yardımcı: lib/README.md'de belgelenmemiş",
	"iyilestirici.NS_C":                "iç yardımcı: lib/README.md'de belgelenmemiş",
	"iyilestirici.dikleştir":           "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"iyilestirici.şekil_grupları":      "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"iyilestirici.yığ_dikleştir":       "iç yardımcı: lib/README.md'de belgelenmemiş",
	"iyilestirici.durum_tensörü":       "iç yardımcı: lib/README.md'de belgelenmemiş",
	"iyilestirici.gradyanları_sıfırla": "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"iyilestirici.yalnızca_2b":         "iç yardımcı: lib/README.md'de belgelenmemiş",
	"iyilestirici.ad_uyar_mı":          "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"iyilestirici.bir_desen_uyar_mı":   "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"iyilestirici.parametre_grupları":  "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"iyilestirici.parametre_kümeleri":  "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"iyilestirici.SatırRMS":            "belgeli ad; yazılacak: Muon parametre grupları örneği",
	"egitim.SONLU_DEĞİL":               "belgeli ad; yazılacak: sıçrama bekçisinin ayrıntıları (44 yalnız TAMAM ile karşılaştırır)",
	"egitim.SIÇRAMA":                   "belgeli ad; yazılacak: sıçrama bekçisinin ayrıntıları (44 yalnız TAMAM ile karşılaştırır)",
	"egitim.EN_KÜÇÜK_GÖRELİ_SAPMA":     "iç yardımcı: lib/README.md'de belgelenmemiş",
	"egitim.aşıyor()":                  "iç yardımcı: lib/README.md'de belgelenmemiş",
	"egitim.geçmiş_uzunluğu()":         "belgeli ad; yazılacak: sıçrama bekçisinin ayrıntıları (44 yalnız TAMAM ile karşılaştırır)",
	"hazir.DESTEKLENEN_MİMARİLER":      "iç yardımcı: lib/README.md'de belgelenmemiş",
	"hazir.ayar_değeri":                "iç yardımcı: lib/README.md'de belgelenmemiş",
	"hazir.hazır_model_kur":            "belgeli ad; yazılacak: hazır modelin iç yapısı (hazır_model_kur, eşleme) örneği",
	"hazir.rope_tabanı":                "belgeli ad; yazılacak: hazır modelin iç yapısı (hazır_model_kur, eşleme) örneği",
	"hazir.rope_ölçekleme_var_mı":      "iç yardımcı: lib/README.md'de belgelenmemiş",
	"hazir.BLOK_ADLARI":                "iç yardımcı: lib/README.md'de belgelenmemiş",
	"hazir.hugging_face_eşle":          "belgeli ad; yazılacak: hazır modelin iç yapısı (hazır_model_kur, eşleme) örneği",
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
