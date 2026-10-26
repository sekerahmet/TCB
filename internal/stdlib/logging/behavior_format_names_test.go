package logging

import (
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Günlüğün ve iş durumunun (progress.json) anahtarları, kayıt türleri ve dosya adı dilden
// bağımsız İngilizce tanımlayıcılardır (dil tanımı 17.3, 19.5); Türkçe programın günlüğü de
// aynı adları taşır. Liste dondurulmuştur: değişirse formatVersion artar.
func TestLogFormatNamesAreLanguageIndependent(t *testing.T) {
	testkit.FormatNames(t, formatNames...)
	testkit.FormatNames(t, jobFile)
	want := []string{"kind", "time", "version", "program", "total", "settings", "step", "rate", "seconds", "message",
		"metrics", "remaining_seconds", "done", "start", "end", "resume"}
	if !slices.Equal(formatNames, want) || formatVersion != 2 {
		t.Errorf("günlük biçiminin adları %v (sürüm %d), beklenen %v (sürüm 2)", formatNames, formatVersion, want)
	}
}
