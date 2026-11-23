// Layer: tools (tools); not shipped to users.
package bench

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
)

// Koşturucunun 2026-11-22 penceresinden sonraki düzeltmelerinin testleri (H1, kontrol/h1-olcu-betik-duzelt; TCB
// 2026-11-23 10:26): Dost'un özel koşusunun yük denetimi ve süreç listesi (DENETİM 949 N1), yalnız bellek ölçen soruların
// koşu sayısı, betiğin yalnız kullandığı modeli bağlaması (BC28), en kısa ölçülebilir süre (BC27).

// customRunCheck Dost'un özel koşusunu sahte yük ve sahte süreç listesiyle koşar, günlüğü basar.
const customRunCheck = `
import importlib.util, json, sys, types
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
rp.wait_for_quiet = lambda q, log: 4.5
rp.process_list = lambda: ["p 1"]
logged = []
rec = types.SimpleNamespace(log=logged.append)
it = {"key": "S10/tr/warm", "runs": 3, "custom": lambda: ({"ready_ms": 30.0}, {"peak_working_set_kb": 1})}
out = rp.run_custom(types.SimpleNamespace(machine="laptop"), it, rec)
print(json.dumps({"log": logged, "runs": len(out["measures"]["ready_ms"]["tr_ms"])}))
`

// DENETİM 949 N1: Dost'un özel koşusu (S9, S10) da her koşudan önce yükü bekleyip ölçer (foreign_load) ve süreç listesini
// koşudan önce ve sonra yazar; düzeltmeden önce ikisi de yoktu (run.py run_item'ın özel dalı).
func TestBaselineCustomRunChecksTheLoad(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", customRunCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got struct {
		Log  []map[string]any
		Runs int
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got.Runs != 3 {
		t.Errorf("%d runs, want 3 after the warm-up", got.Runs)
	}
	for _, index := range []string{"warmup", "0", "1", "2"} {
		seen := map[string]bool{}
		for _, rec := range got.Log {
			if fmt.Sprint(rec["index"]) != index || rec["question"] != "S10/tr/warm" {
				continue
			}
			for _, field := range []string{"foreign_load", "processes_before", "processes_after"} {
				if _, ok := rec[field]; ok {
					seen[field] = true
				}
			}
		}
		if !seen["foreign_load"] || !seen["processes_before"] || !seen["processes_after"] {
			t.Errorf("run %s logs %v; want foreign_load, processes_before and processes_after\n%v", index, seen, got.Log)
		}
	}
}
