package codeindex

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "testdata/schema altın dosyalarını yeniden yaz")

// exampleReferences şema örneğidir (tasarım belgesindeki `kullananlar Muon --json`).
func exampleReferences() *Result {
	r := NewResult(CommandReferences, Query{Text: "Muon", Language: "tr"})
	r.Target = &Node{ID: "struct:lib/iyilestirici.tr#Muon", Kind: KindStruct, Name: "Muon",
		Pos: &Pos{File: "lib/iyilestirici.tr", Line: 72, Col: 1, EndLine: 112, EndCol: 2}, Signature: "yapı Muon"}
	r.Users = []Use{
		{Role: RoleCall, Confidence: Extracted, Pos: Pos{File: "lib/iyilestirici.tr", Line: 197, Col: 16},
			In: &Node{ID: "method:lib/iyilestirici.tr#MuonVeAdamW.kurucu", Kind: KindMethod, Name: "MuonVeAdamW.kurucu"}},
		{Role: RoleTest, Confidence: Extracted, Pos: Pos{File: "lib/iyilestirici_test.tr", Line: 33, Col: 20},
			In: &Node{ID: "test_block:lib/iyilestirici_test.tr#30", Kind: KindTest, Name: "Muon: güncelleme dikleştirilmiş momentumdur"}},
	}
	r.Counts = &Counts{Total: 2, ByRole: map[Role]int{RoleCall: 1, RoleTest: 1}}
	return r
}

func exampleAmbiguous() *Result {
	r := NewResult(CommandDefinition, Query{Text: "ileri", Language: "tr"})
	r.Status = StatusAmbiguous
	r.Candidates = []Node{
		{ID: "method:lib/katman.tr#Doğrusal.ileri", Kind: KindMethod, Name: "Doğrusal.ileri", Pos: &Pos{File: "lib/katman.tr", Line: 30, Col: 5}},
		{ID: "method:lib/katman.tr#GPT.ileri", Kind: KindMethod, Name: "GPT.ileri", Pos: &Pos{File: "lib/katman.tr", Line: 170, Col: 5}},
	}
	return r
}

func exampleNotFound() *Result {
	r := NewResult(CommandDefinition, Query{Text: "zaman_mz", Language: "tr"})
	r.Status = StatusNotFound
	r.DidYouMean = []Node{{ID: "builtin:G502", Kind: KindBuiltin, Name: "G502", Display: "zaman_ms"}}
	return r
}

// Şema örnekleri komut çıktısıyla birebirdir: altın dosyalar değişirse şema değişmiştir (alan
// eklemek uyumlu, çıkarmak SchemaVersion'ı artırmayı gerektirir).
func TestSchemaGoldenFiles(t *testing.T) {
	cases := map[string]*Result{
		"references.json": exampleReferences(),
		"ambiguous.json":  exampleAmbiguous(),
		"not_found.json":  exampleNotFound(),
	}
	for name, res := range cases {
		got, err := EncodeJSON(res)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", "schema", name)
		if *update {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(want) != string(got) {
			t.Errorf("%s changed; if the schema change is intended raise SchemaVersion for removals, then run go test -update.\n got:\n%s\nwant:\n%s", name, got, want)
		}
	}
}

// Altın dosyalar okunabilir ve yeniden yazılınca aynı: sonuç tipi JSON ile kayıpsız gidip gelir.
func TestResultRoundTrip(t *testing.T) {
	for _, res := range []*Result{exampleReferences(), exampleAmbiguous(), exampleNotFound()} {
		data, err := EncodeJSON(res)
		if err != nil {
			t.Fatal(err)
		}
		var back Result
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(res, &back) {
			t.Errorf("round trip differs for %s:\n%+v\n%+v", res.Command, res, &back)
		}
	}
}

func TestSchemaNameFollowsVersion(t *testing.T) {
	if want := "tr-codeindex/" + string(rune('0'+SchemaVersion)); Schema != want {
		t.Errorf("Schema = %q, want %q", Schema, want)
	}
	data, _ := EncodeJSON(exampleReferences())
	if !strings.Contains(string(data), `"schema": "`+Schema+`"`) {
		t.Errorf("the result does not carry the schema name:\n%s", data)
	}
}

func TestExitCodes(t *testing.T) {
	for status, want := range map[string]int{StatusOK: ExitOK, StatusAmbiguous: ExitAmbiguous, StatusNotFound: ExitNotFound} {
		if got := ExitCode(status); got != want {
			t.Errorf("ExitCode(%q) = %d, want %d", status, got, want)
		}
	}
}
