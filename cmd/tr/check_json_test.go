package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kaydın alanları hatanın kaydından gelir: en içteki kod, konum öneksiz ileti, dışlayıcı bitiş, ilgili
// konum (dosyası doldurulur) ve düzeltme.
func TestRecordOfCarriesTheErrorRecord(t *testing.T) {
	rng := messages.Range{Line: 3, Col: 5, EndLine: 3, EndCol: 7}
	err := messages.AtRange(rng, messages.New(messages.H0100, messages.New(messages.H0009))).
		WithRelated(messages.Related{Range: messages.Range{Line: 1, Col: 1, EndLine: 1, EndCol: 2}, Err: messages.New(messages.H8300, "m")}).
		WithFix(messages.Fix{Range: rng, Replacement: "", Applicable: true})
	r := recordOf(err, severityError, "a.tr")
	if r.Code != "H0009" || r.Severity != "error" || r.File != "a.tr" || r.Message != err.Err.Error() {
		t.Errorf("kayıt %+v", r)
	}
	if *r.Start != (jsonPosition{3, 5}) || *r.End != (jsonPosition{3, 7}) {
		t.Errorf("konum %+v-%+v", r.Start, r.End)
	}
	if len(r.Related) != 1 || r.Related[0].File != "a.tr" || r.Related[0].Message == "" {
		t.Errorf("ilgili konum %+v", r.Related)
	}
	// Boş değiştirme (silme) yazılır: replacement alanı atlanmaz.
	line, _ := json.Marshal(r.Fixes[0])
	if string(line) != `{"start":{"line":3,"column":5},"end":{"line":3,"column":7},"replacement":"","applicable":true}` {
		t.Errorf("düzeltme %s", line)
	}
}

// Konumu bilinmeyen hatada file, start ve end yazılmaz; sütunu bilinmeyende start yalnız satır taşır.
func TestRecordOfLeavesOutWhatIsUnknown(t *testing.T) {
	line, _ := json.Marshal(recordOf(messages.New(messages.H8605, "kontrol", "a.tr"), severityError, "a.tr"))
	var fields map[string]any
	if err := json.Unmarshal(line, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"file", "start", "end", "related", "fixes"} {
		if _, ok := fields[name]; ok {
			t.Errorf("konumsuz kayıt %q yazıyor: %s", name, line)
		}
	}
	lineOnly, _ := json.Marshal(recordOf(messages.At(7, 0, messages.New(messages.H1009, "toplam")), severityWarning, "a.tr"))
	want := `"start":{"line":7}`
	if got := string(lineOnly); !strings.Contains(got, want) || strings.Contains(got, `"end"`) {
		t.Errorf("sütunsuz kayıt %s", got)
	}
}

// --json seçeneği dosya adından önce aranır; '--'dan ve dosya adından sonrası programındır.
func TestScanJSONOnlyLooksBeforeTheFile(t *testing.T) {
	for args, want := range map[string]bool{
		"--json a.tr": true, "--bogus --json a.tr": true, "a.tr --json": false, "-- --json": false, "a.tr": false,
	} {
		if got := scanJSON(strings.Fields(args)); got != want {
			t.Errorf("scanJSON(%q) = %v, want %v", args, got, want)
		}
	}
}
