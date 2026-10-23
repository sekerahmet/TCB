package testkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// ValidLossMetrics bir eğitim günlüğünün (metrics.jsonl) geçerli ölçü kayıtlarını "adım:kayıp"
// olarak verir: bir devam kaydından önce gelen ve adımı ondan büyük olan kayıtlar geçersizdir
// (19, günlük okuyucu kuralı); kaybı olmayan kayıtlar ölçü değildir. skipBroken true ise
// ayrıştırılamayan satırlar atlanır (öldürülen sürecin yarım kalmış satırı); false ise test düşer.
func ValidLossMetrics(t testing.TB, path string, skipBroken bool) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type measure struct {
		step int64
		loss string
	}
	var measures []measure
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var rec map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			if skipBroken {
				continue
			}
			t.Fatalf("invalid line %q", line)
		}
		var step int64
		var kind string
		_ = json.Unmarshal(rec["adım"], &step)
		_ = json.Unmarshal(rec["tür"], &kind)
		switch {
		case kind == "devam":
			measures = slices.DeleteFunc(measures, func(o measure) bool { return o.step > step })
		case kind == "" && rec["kayıp"] != nil:
			measures = append(measures, measure{step, string(rec["kayıp"])})
		}
	}
	var out []string
	for _, o := range measures {
		out = append(out, fmt.Sprintf("%d:%s", o.step, o.loss))
	}
	return out
}

// AssertSameRunFiles iki koşu klasöründe files'ın her birinin bit düzeyinde aynı olduğunu ve
// metrics.jsonl'lerin geçerli kayıp dizilerinin (yarım satırlar atlanarak) eşit olduğunu denetler.
func AssertSameRunFiles(t testing.TB, dirA, dirB string, files []string) {
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
	ka := ValidLossMetrics(t, filepath.Join(dirA, "metrics.jsonl"), true)
	kb := ValidLossMetrics(t, filepath.Join(dirB, "metrics.jsonl"), true)
	if !slices.Equal(ka, kb) {
		t.Errorf("valid loss series differ (%d, %d records):\n%v\n%v", len(ka), len(kb), ka, kb)
	}
}
