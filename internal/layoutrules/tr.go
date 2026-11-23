package layoutrules

import "strings"

// El yazımı .tr dosyalarının boy kuralları: dosya en çok FileLimit, TR fonksiyonu en çok FuncLimit satırdır
// (internal/layout_tr_test.go: TestLayoutTRFileSize, TestLayoutTRFuncSize). Ölçü ve muafiyet burada durur; düzen testi
// de kod grafı (tools/codegraph, tests-for'un size: satırı) da buradan okur, iki okuyucu aynı sayıyı görür.

// TRBigFiles FileLimit'i aşan el yazımı .tr dosyalarıdır; gerekçeleri internal/layout_tr_test.go'nun trBigFileReasons'ında
// (Go'nun istisnaları gibi: gerekçe geliştiricinin notudur, metin sabiti olarak test dosyasında durur). Liste yalnız
// muafiyettir, tavan koymaz (TCB, DENETİM 738 Ş2 (a)): listedeki dosya büyüyebilir; sınırın altına inen dosya listeden
// çıkar (düzen testi düşer).
var TRBigFiles = map[string]bool{
	"lib/turkish.tr":            true,
	"tests/tensor_test.tr":      true,
	"tests/yerlesikler_test.tr": true,
	"lib/layers_test.tr":        true,
	"lib/layers.tr":             true,
	"tests/tipler_test.tr":      true,
	"lib/vits.tr":               true,
	"lib/whisper.tr":            true,
	"tests/vad_test.tr":         true,
	"lib/pretrained.tr":         true,
}

// trUnmeasured .tr düzeninin ölçmediği yolların önekleridir: bilerek hatalı fikstürler, codegraph'ın fikstürleri, arşiv.
var trUnmeasured = []string{"tests/negative/", "tools/codegraph/testdata/", "docs/archive/"}

// TRMeasured .tr düzeni bu dosyayı (depo köküne göre, eğik çizgili yol) ölçer mi: el yazımı .tr dosyası, trUnmeasured
// öneklerinin altında değil.
func TRMeasured(rel string) bool {
	if !strings.HasSuffix(rel, ".tr") {
		return false
	}
	for _, p := range trUnmeasured {
		if strings.HasPrefix(rel, p) {
			return false
		}
	}
	return true
}

// TRFileExempt dosya TRBigFiles'ta mı: muaf dosyanın boy sınırı yoktur, sınırın altına inince kaydı silinir.
func TRFileExempt(path string) bool {
	return TRBigFiles[path]
}

// TRLines düzenin saydığı .tr dosyasının satırlarıdır: CRLF LF olur, sondaki tek satır sonu kırpılır, kalan satır
// sonlarından bölünür. Dosyanın boyu len(TRLines(data))'dır: son satır sonu olan dosyada FileLines'a eşit, olmayanda
// bir fazla (satır sonu olmayan son satır da satırdır); boş dosya 1 satırdır.
func TRLines(data []byte) []string {
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), "\n")
}

// TRFunc bir TR fonksiyonunun düzendeki ölçüsüdür.
type TRFunc struct {
	Name  string // TR_func'tan sonraki ad, parantezine ya da köşeli parantezine kadar
	Line  int    // TR_func satırı (1'den)
	Lines int    // TR_func satırından gövdesini kapatan '}'a kadar
}

// TRFuncs dosyanın satırlarındaki (TRLines) bütün TR fonksiyonlarıdır: boşluk kırpılınca "TR_func " ile başlayan her
// satır bir fonksiyondur; iç içe fonksiyon kendi boyuyla ayrıca sayılır.
func TRFuncs(lines []string) []TRFunc {
	var out []TRFunc
	for i, line := range lines {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "TR_func ")
		if !ok {
			continue
		}
		name := strings.TrimSpace(rest)
		if j := strings.IndexAny(name, "([ "); j >= 0 {
			name = name[:j]
		}
		out = append(out, TRFunc{name, i + 1, trFuncLines(lines[i:])})
	}
	return out
}

// trFuncLines lines[0]'da başlayan fonksiyonun satır sayısıdır: ilk '{'dan onu kapatan '}'a kadar (metin
// değişmezlerinin ve yorumların içindeki parantezler sayılmaz). Gövdesi bulunmayan satır 1'dir.
func trFuncLines(lines []string) int {
	depth, opened := 0, false
	for i, line := range lines {
		inText, raw := false, false
		for j := 0; j < len(line); j++ {
			switch c := line[j]; {
			case raw:
				raw = c != '`'
			case inText:
				if c == '\\' {
					j++
				} else if c == '"' {
					inText = false
				}
			case c == '#':
				j = len(line)
			case c == '"':
				inText = true
			case c == '`':
				raw = true
			case c == '{':
				depth, opened = depth+1, true
			case c == '}':
				depth--
			}
		}
		if opened && depth <= 0 {
			return i + 1
		}
	}
	return 1
}
