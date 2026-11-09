package testkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// ValidLossMetrics bir eğitim günlüğünün (metrics.jsonl) geçerli ölçü kayıtlarını "step:loss"
// olarak verir (kayıp {kayıp: k} ile yazılır, anahtarı "loss"tur; 17.3). Günlük okuyucu kuralı
// (19.6): bir resume kaydının offset'i ile kendisi arasındaki her kayıt, adımı olsun olmasın,
// geçersizdir; resume'dan önce gelen ve adımı ondan büyük olan kayıtlar da geçersizdir. Kaybı
// olmayan kayıtlar ölçü değildir. skipBroken true ise ayrıştırılamayan satırlar atlanır
// (öldürülen sürecin yarım kalmış satırı); false ise test düşer.
func ValidLossMetrics(t testing.TB, path string, skipBroken bool) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type measure struct {
		start, step int64 // start: satırın dosyadaki bayt konumu
		loss        string
	}
	var measures []measure
	starts := map[int64]bool{}
	for start := int64(0); start < int64(len(data)); {
		line, _, _ := bytes.Cut(data[start:], []byte("\n"))
		lineStart := start
		start += int64(len(line)) + 1
		starts[lineStart] = true
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec map[string]json.RawMessage
		if err := json.Unmarshal(line, &rec); err != nil {
			if skipBroken {
				continue
			}
			t.Fatalf("invalid line %q", line)
		}
		var step int64
		var kind string
		if s := rec["step"]; s != nil { // başlangıç kaydında adım yoktur
			if err := json.Unmarshal(s, &step); err != nil {
				t.Fatalf("step is not a whole number in %q: %v", line, err)
			}
		}
		_ = json.Unmarshal(rec["kind"], &kind)
		switch {
		case kind == "resume":
			offset := resumeOffset(t, rec, line, lineStart, starts)
			measures = slices.DeleteFunc(measures, func(o measure) bool { return o.start >= offset || o.step > step })
		case kind == "" && rec["loss"] != nil:
			measures = append(measures, measure{lineStart, step, string(rec["loss"])})
		}
	}
	var out []string
	for _, o := range measures {
		out = append(out, fmt.Sprintf("%d:%s", o.step, o.loss))
	}
	return out
}

// resumeOffset devam kaydının offset'idir (19.6, 20.2: noktadaki günlüğün bayt uzunluğu).
// Kayıtsız ya da tam sayı olmayan offset ve satır ortasına düşen offset bozuk günlüktür, test
// düşer. Devam satırından ilerideki offset (günlük kesilmiş, 20.6) hiçbir kaydı geçersiz kılmaz.
func resumeOffset(t testing.TB, rec map[string]json.RawMessage, line []byte, lineStart int64, starts map[int64]bool) int64 {
	t.Helper()
	var offset int64
	if err := json.Unmarshal(rec["offset"], &offset); err != nil || offset < 0 {
		t.Fatalf("resume record without a whole, non-negative offset: %q", line)
	}
	if offset < lineStart && !starts[offset] {
		t.Fatalf("resume offset %d falls inside a line: %q", offset, line)
	}
	return offset
}

// AssertSameRunFiles iki koşu klasöründe files'ın her birinin bit düzeyinde aynı olduğunu ve
// metrics.jsonl'lerin geçerli kayıp dizilerinin eşit olduğunu denetler; skipBroken için bkz. ValidLossMetrics.
func AssertSameRunFiles(t testing.TB, dirA, dirB string, files []string, skipBroken bool) {
	t.Helper()
	for _, name := range files {
		x, err1 := os.ReadFile(filepath.Join(dirA, name))
		y, err2 := os.ReadFile(filepath.Join(dirB, name))
		if err1 != nil || err2 != nil {
			t.Errorf("%s unreadable: %v %v", name, err1, err2)
		} else if !bytes.Equal(x, y) {
			t.Errorf("%s differs bitwise", name)
		}
	}
	ka := ValidLossMetrics(t, filepath.Join(dirA, "metrics.jsonl"), skipBroken)
	kb := ValidLossMetrics(t, filepath.Join(dirB, "metrics.jsonl"), skipBroken)
	if !slices.Equal(ka, kb) {
		t.Errorf("valid loss series differ (%d, %d records):\n%v\n%v", len(ka), len(kb), ka, kb)
	}
}
