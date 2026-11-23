package layoutrules

import "strings"

// El yazımı .tr dosyalarının boy kuralları: dosya en çok FileLimit, TR fonksiyonu en çok FuncLimit satırdır
// (internal/layout_tr_test.go: TestLayoutTRFileSize, TestLayoutTRFuncSize). Ölçü ve istisnalar burada durur; düzen testi
// de kod grafı (tools/codegraph, tests-for'un size: satırı) da buradan okur, iki okuyucu aynı sayıyı görür.

// MaxTRBigFiles TRBigFiles'ın en fazla kayıt sayısıdır (MaxBigFiles gibi: liste büyümez).
const MaxTRBigFiles = 10

// TRBigFiles FileLimit'i aşan el yazımı .tr dosyalarının tavanıdır (2026-11-23'teki satır sayısı, TRLines'la).
// Gerekçeleri Go'nun istisnalarınınkilerle birlikte internal/layout_exceptions_test.go'nun trBigFileReasons'ındadır
// (gerekçe geliştiricinin notudur, metin sabiti olarak test dosyasında durur). Go'nun BigFiles'ı gibi YALNIZCA KÜÇÜLEBİLİR: dosya tavanı ve
// düzeltme payını (FixMargin) aşamaz; sınırın altına inen dosyanın kaydı silinir, yeni kayıt eklenmez, tavan yükseltilmez.
// Liste 2026-11-23'e kadar yalnız muafiyetti (DENETİM 738 Ş2 (a)); lib/pretrained.tr uyarısız 583'ten 787 satıra
// büyüyünce TCB tavanı koydu (tcb-cevaplar, 2026-11-23 18:56).
var TRBigFiles = map[string]int{
	"lib/turkish.tr":            3211,
	"tests/tensor_test.tr":      2105,
	"tests/yerlesikler_test.tr": 783,
	"lib/layers_test.tr":        810,
	"lib/layers.tr":             788,
	"tests/tipler_test.tr":      741,
	"lib/vits.tr":               1105,
	"lib/whisper.tr":            611,
	"tests/vad_test.tr":         610,
	"lib/pretrained.tr":         787,
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

// TRFileMax .tr dosyasının bugün aşamayacağı boydur: istisna listesinde değilse FileLimit, listedeyse tavan + düzeltme
// payı (FileMax'ın .tr eşi).
func TRFileMax(path string) (max int, exception bool) {
	return limitOf(TRBigFiles, path, FileLimit)
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
