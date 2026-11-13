package main

import (
	"path/filepath"
	"testing"
)

// Kural: sözlük dosyalarının geçmişi kod başına olay listesidir, en yeni önce: bir kodu ekleyen ve silen her commit
// (iki dosyada iki satır: iki olay); aynı commit'te eklenip silinen (taşınan) satır olay değildir. deleted bu taramayı
// okur (deletedCodes).
func TestScanCodeHistoryMarksAddedAndRemoved(t *testing.T) {
	slowFixture(t)
	root := gitFixture(t)
	tr := filepath.Join(root, "internal", "lang", "messages", "tr.json")
	defs := filepath.Join(root, "internal", "lang", "messages", "definitions.json")
	replaceIn(t, tr, "\"H0002\": \"second message\"", "\"H0002\": \"second message\",\n\"H0003\": \"third\"")
	replaceIn(t, defs, "\"H0002\": { \"param\": [] }", "\"H0002\": { \"param\": [] },\n\"H0003\": { \"param\": [] }")
	commitAll(t, root, "messages: add H0003")
	replaceIn(t, tr, ",\n\"H0003\": \"third\"", "")
	replaceIn(t, defs, ",\n\"H0003\": { \"param\": [] }", "")
	commitAll(t, root, "messages: drop H0003")

	events, err := scanCodeHistory(root, messageFiles)
	if err != nil {
		t.Fatal(err)
	}
	list := events["H0003"]
	if len(list) != 4 || !list[0].removed || !list[len(list)-1].added {
		t.Fatalf("H0003 events newest first (removed twice, added twice): %+v", list)
	}
	if list[0].commit.subject != "messages: drop H0003" || list[3].commit.subject != "messages: add H0003" {
		t.Errorf("commits: %+v", list)
	}
	// Taşınan (aynı commit'te eklenen ve silinen) satır olay sayılmaz: H0001 yalnızca ilk commit'te eklendi.
	if got := events["H0001"]; len(got) != 2 || !got[0].added {
		t.Errorf("H0001: %+v", got)
	}
}
