package layoutrules

import (
	"slices"
	"strings"
	"testing"
)

// Kural: .tr dosyasının boyu son satır sonu olan dosyada satır sonu sayısıdır (FileLines), olmayanda bir fazlası; CRLF
// tek satır sonudur; boş dosya 1 satırdır.
func TestTRLinesCountsTheLastLineWithoutANewline(t *testing.T) {
	for _, c := range []struct {
		data  string
		lines int
	}{{"", 1}, {"a", 1}, {"a\n", 1}, {"a\nb", 2}, {"a\nb\n", 2}, {"a\r\nb\r\n", 2}, {"a\n\n", 2}} {
		if got := len(TRLines([]byte(c.data))); got != c.lines {
			t.Errorf("TRLines(%q) = %d lines, want %d", c.data, got, c.lines)
		}
	}
	if got, fl := len(TRLines([]byte("a\nb\n"))), FileLines([]byte("a\nb\n")); got != fl {
		t.Errorf("with a final newline TRLines %d, FileLines %d", got, fl)
	}
}

// Kural: TR fonksiyonu TR_func satırından gövdesini kapatan '}'a kadardır; metindeki, ham metindeki ve yorumdaki
// parantez sayılmaz; iç içe fonksiyon kendi boyuyla ayrıca sayılır; gövdesiz satır 1'dir.
func TestTRFuncsMeasureEachFunctionFromItsLineToItsBrace(t *testing.T) {
	src := strings.Join([]string{
		"TR_func TR_dış(TR_x: TR_Int) -> TR_Int {", // 1
		`    TR_var a = "{"`,                       // 2
		"    TR_var b = `}`",                       // 3
		"    # } yorum",                            // 4
		"    TR_func _iç[T](y: T) {",               // 5
		"        TR_return y",                      // 6
		"    }",                                    // 7
		`    TR_return "\"}"`,                      // 8
		"}",                                        // 9
		"TR_func TR_bildirim()",                    // 10
	}, "\n")
	got := TRFuncs(TRLines([]byte(src)))
	want := []TRFunc{{"TR_dış", 1, 9}, {"_iç", 5, 3}, {"TR_bildirim", 10, 1}}
	if !slices.Equal(got, want) {
		t.Errorf("TRFuncs = %v, want %v", got, want)
	}
}

// Kural: .tr düzeni bilerek hatalı fikstürleri (tests/negative), codegraph'ın fikstürlerini ve arşivi ölçmez; öbür .tr
// dosyalarını (testdata altındakiler dahil) ölçer; .tr olmayan dosya onun işi değildir.
func TestTRMeasuredLeavesOutNegativesFixturesAndTheArchive(t *testing.T) {
	for rel, want := range map[string]bool{
		"lib/audio.tr": true, "tests/vad_test.tr": true, "internal/devtools/codeindex/testdata/proj/lib/geo.tr": true,
		"tests/negative/alan_tipi.tr": false, "tools/codegraph/testdata/x.tr": false, "docs/archive/a.tr": false, "lib/audio.go": false,
	} {
		if got := TRMeasured(rel); got != want {
			t.Errorf("TRMeasured(%s) = %v, want %v", rel, got, want)
		}
	}
}

// Kural: .tr istisnasının sınırı Go'nunki gibi tavanı + düzeltme payıdır; listede olmayanın sınırı FileLimit; her
// kayıt ölçülen bir .tr dosyasıdır ve tavanı FileLimit'in üstündedir (gerekçeleri internal/layout_exceptions_test.go'da, TestLayoutTRFileSize denetler).
func TestATRExceptionsLimitIsItsCeilingPlusTheMargin(t *testing.T) {
	for file, ceiling := range TRBigFiles {
		if max, exc := TRFileMax(file); !exc || max != ceiling+FixMargin(ceiling) || !TRMeasured(file) || ceiling <= FileLimit {
			t.Errorf("TRFileMax(%s) = %d, %v (ceiling %d)", file, max, exc, ceiling)
		}
	}
	if max, exc := TRFileMax("lib/a.tr"); exc || max != FileLimit {
		t.Errorf("TRFileMax of a file off the list = %d, %v", max, exc)
	}
	if max, _ := TRFileMax("lib/pretrained.tr"); max != 787+78 {
		t.Errorf("lib/pretrained.tr's limit %d, want 865 (787 + 78)", max)
	}
}
