// Layer: tools (tools); not shipped to users.
package bench

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Emsallerin en hızlı ayarının taraması (Ş1, DENETİM 944, 947 ve 948): taramanın seçimi ve ölçüye geçişi, ölçünün yalnız
// kendi commit'inin bütün taramasını alması, taramanın eski pencerenin dosyasını bırakmaması.

// settingsCheck en hızlı ayarın emsal komutlarına geçişini ve taramanın seçimini (sahte ölçülerle) basar.
const settingsCheck = `
import importlib.util, json, os, sys, tempfile
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
qs = rp.load_questions()
work = tempfile.mkdtemp()
json.dump({"trlang": "trlang", "whisper_cli": "w", "llama_bench": "b", "llama_completion": "c", "go_compare": "g"},
          open(os.path.join(work, "tools.json"), "w"))
open(os.path.join(work, "ids_tr.json"), "w").write("{}")
qs.piper_ids_tr = lambda ctx: None
ctx = qs.Ctx(rp, "laptop", 97, work, lambda r: None, dry=True)
ctx.settings = {"llama.cpp read": {"threads": 0}, "llama.cpp gen": {"threads": 6}, "faster-whisper int8": {"threads": 6},
                "onnxruntime": {"threads": 20}, "llama.cpp S3": {"flash_attn": 1}}
items = {it["key"]: it for q in ("S1", "S3", "S5", "S7") for it in qs.items(ctx, q)}
out = {"s1": items["S1/qwen3-4b-q4km"]["ref"]["cmds"][0], "s1gen": items["S1/qwen3-4b-q4km"]["ref"]["cmds"][1], "s3": items["S3/qwen3-4b-q4km"]["ref"]["cmds"][0],
       "int8": items["S5/tr/faster-whisper-int8"]["ref"]["cmds"][0], "f32": items["S5/tr/faster-whisper-float32"]["ref"]["cmds"][0],
       "s7": items["S7/tr"]["ref"]["cmds"][0], "ort": sorted(k for k in items if "/ort-" in k)}
# the scan with fake runs: llama.cpp's reading is fastest with 6 threads, its generation with the logical count,
# whisper.cpp with the default (0)
def fake(key, side, index, log, machine, cmds=None):
    c = side["cmds"][0]
    n = 0 if "-t" not in c else int(c[c.index("-t") + 1])
    gen = "-p" in c and c[c.index("-p") + 1] == "0"
    # every fake time over the shortest measurable 20 ms (BC27), the order unchanged
    ms = {"S1 read": {0: 500, 6: 300, os.cpu_count(): 400}, "S1 gen": {0: 200, 6: 250, os.cpu_count(): 100},
          "S5/tr/whisper.cpp": {0: 50, 6: 90, os.cpu_count(): 70}}[("S1 gen" if gen else "S1 read") if key.startswith("S1") else key][n]
    return {"results": [{"avg_ns": ms * 1e6}] * len(side["cmds"]), "runs": [{"stderr": "total time = %d ms\\nload time = 0 ms" % ms}]}
rp._run_side = fake
ctx.settings = {}
scan_out = tempfile.mkdtemp()
rec = type("R", (), {"log": lambda self, r: None})()
rp.scan_settings(ctx, qs, rec, ["llama.cpp read", "llama.cpp gen"], scan_out, runs=3)
rp.scan_settings(ctx, qs, rec, ["whisper.cpp"], scan_out, runs=1)
out["scan"] = json.load(open(os.path.join(scan_out, "en-hizli.json"), encoding="utf-8"))
out["commit"] = rp.head_commit()
out["groups"] = rp.SCAN_GROUPS
print(json.dumps(out))
`

// Ş1 (DENETİM 944 ve 947, TCB 2026-11-22): pencerenin başındaki kısa tarama (S1'in okuması ve üretimi ayrı grup, N1) her emsalin en hızlı ayarını (iş parçacığı:
// varsayılan, 6, mantıksal sayı; S3'te flash attention) en kısa ortancayla seçer ve dosyaya birleştirir; ölçü o ayarla
// koşar: 0 kütüphanenin varsayılanıdır (llama.cpp'de -t yok), ayarı seçilmeyen emsal makinenin sayısını alır, S7'nin ayrı
// onnxruntime kalemleri düşer.
func TestLaptopFastestSettings(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", settingsCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got struct {
		S1, S1gen, S3, Int8, F32, S7 []string
		Ort                          []string
		Scan                         map[string]any
		Commit                       string
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n") // taramanın satırları, sonda JSON
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	line := func(c []string) string { return " " + strings.Join(c, " ") + " " }
	if strings.Contains(line(got.S1), " -t ") || !strings.Contains(line(got.S1gen), " -t 6 ") {
		t.Errorf("llama.cpp reading with the default %v, want no -t; generation %v, want -t 6 (N1)", got.S1, got.S1gen)
	}
	if !strings.Contains(line(got.S3), " -fa 1 ") {
		t.Errorf("S3 with flash attention 1: %v", got.S3)
	}
	if !strings.Contains(line(got.Int8), " --threads 6 ") || !strings.Contains(line(got.F32), " --threads 97 ") ||
		!strings.Contains(line(got.S7), " --threads 20 ") {
		t.Errorf("int8 %v, float32 %v, onnxruntime %v; want 6, the machine's 97, 20", got.Int8, got.F32, got.S7)
	}
	if len(got.Ort) != 0 {
		t.Errorf("S7's onnxruntime items %v with a scan, want none", got.Ort)
	}
	r, _ := got.Scan["llama.cpp read"].(map[string]any)
	g, _ := got.Scan["llama.cpp gen"].(map[string]any)
	w, _ := got.Scan["whisper.cpp"].(map[string]any)
	table, _ := got.Scan["_scan"].(map[string]any)
	gt, _ := table["llama.cpp gen"].(map[string]any)
	// K2 (H1e): commit artık dosyanın değil grubun kaydıdır; ölçü onu denetlemez.
	if r["threads"] != 6.0 || g["threads"] == 6.0 || g["threads"] == 0.0 || w["threads"] != 0.0 || len(table) != 3 || gt["measures_ms"] == nil ||
		len(got.Commit) != 40 || gt["commit"] != got.Commit || got.Scan["_commit"] != nil {
		t.Errorf("scan: %v, want reading 6 threads, generation the logical count, whisper.cpp the default, each measure's median, the group's commit %s", got.Scan, got.Commit)
	}
}

// DENETİM 947 ve 948 Ş1: ölçü kipinde taramanın ilk adımı (olcu_tara_basla) eski en-hizli.json'ları siler ve bütün
// grupları ister; eksik listeyle durur. Denemede dosyaya dokunmaz.
func TestLaptopMeasureScanStartsEmpty(t *testing.T) {
	script := `mkdir -p "$KAYIT/run-en-hizli-ayar"
echo '{}' > "$AYAR"; echo '{}' > "$KAYIT/run-en-hizli-ayar/en-hizli.json"
( olcu_tara_basla "llama.cpp read" ); echo "bir grup rc=$?"
[ -f "$AYAR" ] && echo "ayar kaldı" || echo "ayar silindi"
[ -f "$KAYIT/run-en-hizli-ayar/en-hizli.json" ] && echo "kopya kaldı" || echo "kopya silindi"
( olcu_tara_basla "${TARA_GRUPLARI[@]}" ); echo "bütün gruplar rc=$?"
echo '{}' > "$AYAR"; ( OLCU_DENEME=1; olcu_tara_basla "llama.cpp read" ); echo "deneme rc=$?"
[ -f "$AYAR" ] && echo "denemede ayar kaldı" || echo "denemede ayar silindi"`
	out, code := libRun(t, t.TempDir(), nil, script)
	for _, want := range []string{"bir grup rc=2", "ayar silindi", "kopya silindi", "ölçü kipinde tarama bütün grupları koşar",
		"bütün gruplar rc=0", "deneme rc=0", "denemede ayar kaldı"} {
		if code != 0 || !strings.Contains(out, want) {
			t.Errorf("olcu_tara_basla: exit %d, want %q\n%s", code, want, out)
		}
	}
}

// DENETİM 948 Ş1: ölçü kipinde hazırlıkta (burada model bağlamada) düşen en-hizli-ayar.sh eski pencerenin ayarını
// bırakmaz; düzeltmeden önce dosya yalnız olcu_tara'da, betiğin sonunda siliniyordu. Denemede dosya kalır. Derleme sahte
// (go), modellerin klasörü boş.
func TestLaptopScanFailingInSetupLeavesNoSetting(t *testing.T) {
	script := `mkdir -p "$KAYIT/run-en-hizli-ayar"; echo '{}' > "$AYAR"; echo '{}' > "$KAYIT/run-en-hizli-ayar/en-hizli.json"
go() { :; }; export -f go
bash "$HERE/en-hizli-ayar.sh"; echo "rc=$?"
[ -f "$AYAR" ] && echo "ayar kaldı" || echo "ayar silindi"
[ -f "$KAYIT/run-en-hizli-ayar/en-hizli.json" ] && echo "kopya kaldı" || echo "kopya silindi"`
	env := []string{"PY=" + filepath.ToSlash(python(t)), "OLCU_MODELLER=" + filepath.ToSlash(t.TempDir())}
	for _, c := range []struct {
		env  []string
		want []string
	}{
		{env, []string{"rc=2", "model qwen3-4b-q4km hazır değil", "ayar silindi", "kopya silindi"}},
		{append(env, "OLCU_DENEME=1"), []string{"rc=2", "model qwen3-4b-q4km hazır değil", "ayar kaldı", "kopya kaldı"}},
	} {
		out, code := libRun(t, t.TempDir(), c.env, script)
		for _, want := range c.want {
			if code != 0 || !strings.Contains(out, want) {
				t.Errorf("%v: exit %d, want %q\n%s", c.env[len(c.env)-1], code, want, out)
			}
		}
	}
}

// noiseCheck taramayı sahte koşularla koşar: ayar başına üç koşunun değerleri sırayla verilir, çağrıların sırası yazılır.
const noiseCheck = `
import importlib.util, json, os, sys, tempfile
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
qs = rp.load_questions()
work = tempfile.mkdtemp()
json.dump({"trlang": "trlang", "whisper_cli": "w", "llama_bench": "b", "llama_completion": "c", "go_compare": "g"},
          open(os.path.join(work, "tools.json"), "w"))
N = os.cpu_count()
def scan(times):
    ctx = qs.Ctx(rp, "laptop", 97, work, lambda r: None, dry=True)
    calls, count = [], {}
    def fake(key, side, index, log, machine, cmds=None):
        c = side["cmds"][0]
        n = 0 if "-t" not in c else int(c[c.index("-t") + 1])
        calls.append([index, n])
        if index == "scan warmup":
            ms = 1000
        else:
            ms = times[n][count.get(n, 0)]
            count[n] = count.get(n, 0) + 1
        return {"results": [{}], "runs": [{"stderr": "whisper_print_timings:    total time = %d.00 ms\\nwhisper_print_timings:     load time =     0.00 ms" % ms}]}
    rp._run_side = fake
    out = tempfile.mkdtemp()
    rp.scan_settings(ctx, qs, type("R", (), {"log": lambda self, r: None})(), ["whisper.cpp"], out, runs=3)
    t = json.load(open(os.path.join(out, "en-hizli.json")))["_scan"]["whisper.cpp"]
    return {"chosen": t["chosen"], "calls": calls, "order": t["order"], "runs_ms": t["runs_ms"], "rule": t["rule"]}
print(json.dumps({"N": N,
    "overlap": scan({0: [100, 110, 120], 6: [130, 140, 150], N: [95, 105, 125]}),
    "dominant": scan({0: [100, 110, 120], 6: [130, 140, 150], N: [80, 85, 90]}),
    "edge": scan({0: [100, 110, 120], 6: [130, 140, 150], N: [90, 95, 100]}),
    "six": scan({0: [100, 110, 120], 6: [60, 70, 80], N: [65, 75, 85]})}))
`

// K6 (H1e; DENETİM 960 Ş1): her ayar bir kez ısınır, sonra koşular ayarlar arasında döner (0, 6, N, 0, 6, N, …); bir
// ayar, o ana kadar seçilenin yerine ancak üç koşusunun hepsi onunkilerin hepsinden kesin kısaysa geçer. Ortancası kısa
// ama koşuları örtüşen ayar seçilmez; sınırdaki eşitlik de geçmez (< ile <= ayrımı).
func TestBaselineScanKeepsTheEarlierSettingWithinNoise(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", noiseCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n") // taramanın satırları, sonda JSON
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var n float64
	_ = json.Unmarshal(got["N"], &n)
	for name, want := range map[string]float64{"overlap": 0, "dominant": n, "edge": 0, "six": 6} {
		var c struct {
			Chosen float64
			Calls  [][]any
			Order  []float64
			Rule   string
		}
		if err := json.Unmarshal(got[name], &c); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if c.Chosen != want || c.Rule != "noise" {
			t.Errorf("%s: chose %v (rule %q), want %v", name, c.Chosen, c.Rule, want)
		}
		wantOrder := []float64{0, 6, n, 0, 6, n, 0, 6, n}
		if !reflect.DeepEqual(c.Order, wantOrder) {
			t.Errorf("%s: runs in order %v, want %v", name, c.Order, wantOrder)
		}
		var calls []string
		for _, x := range c.Calls {
			calls = append(calls, fmt.Sprintf("%v %v", x[0], x[1]))
		}
		wantCalls := []string{"scan warmup 0", "scan warmup 6", fmt.Sprintf("scan warmup %v", n)}
		for i := 0; i < 3; i++ {
			for _, v := range []float64{0, 6, n} {
				wantCalls = append(wantCalls, fmt.Sprintf("scan %d %v", i, v))
			}
		}
		if !reflect.DeepEqual(calls, wantCalls) {
			t.Errorf("%s: calls %v, want %v", name, calls, wantCalls)
		}
	}
}
