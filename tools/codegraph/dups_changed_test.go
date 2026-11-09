package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Kural: `dups --changed` farkta eklenen ya da değişen Go satırı yoksa bunu söyler; "0 duplicate candidate(s)" tek başına
// dalın kodu hakkında bir cevap sanılıyordu (gaps, "`dups --changed` değişiklik yokken bunu söylemiyor", D1 15.5 devri).
// Değişiklik varken satır çıkmaz; JSON biçimi değişmez.
func TestDupsChangedSaysWhenThereIsNothingToScan(t *testing.T) {
	root := branchWithCopy(t)
	var out, errb bytes.Buffer
	if code := run([]string{"dups", "--root", root, "--min", "60", "--changed", "main"}, &out, &errb); code != exitOK ||
		strings.Contains(out.String(), "no changes to scan") {
		t.Errorf("the branch changed new.go: exit %d, no such line\n%s%s", code, out.String(), errb.String())
	}
	gitMust(t, root, "checkout", "-q", "main")
	for _, extra := range [][]string{nil, {"--json"}} {
		out.Reset()
		errb.Reset()
		args := append([]string{"dups", "--root", root, "--min", "60", "--changed", "main"}, extra...)
		if code := run(args, &out, &errb); code != exitOK {
			t.Fatalf("%q: exit %d: %s", args, code, errb.String())
		}
		if extra == nil {
			if !strings.Contains(out.String(), "0 duplicate candidate(s) touching main...main ") ||
				!strings.Contains(out.String(), "no changes to scan: main...main adds or changes no Go line") {
				t.Errorf("main against itself has nothing to scan, and says so:\n%s", out.String())
			}
			continue
		}
		var res map[string]any
		if err := json.Unmarshal(out.Bytes(), &res); err != nil || res["changed"] != "main...main" || len(res) != 6 {
			t.Errorf("the JSON keeps its fields (%v): %v", err, res)
		}
	}
}
