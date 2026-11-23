// Layer: tools (tools); not shipped to users.
package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Emsallerin en hızlı ayarının taraması (Ş1, DENETİM 944, 947 ve 948): taramanın seçimi ve ölçüye geçişi, ölçünün yalnız
// kendi commit'inin bütün taramasını alması, taramanın eski pencerenin dosyasını bırakmaması (H1e: kimlik, dönüşüm, eksik grup).

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

// K5 (H1e; DENETİM 948 Ş1'in yeni hâli): hazırlıkta (burada model bağlamada) düşen en-hizli-ayar.sh kayıt kökünün
// dosyasına dokunmaz: taranmayan grupların girdisi korunur, ölçü onları kimlikleriyle yargılar (K2, K3). Ölçü kipinde
// dosya artık silinmez; denemede de aynı. Derleme sahte (go), modellerin klasörü boş.
func TestLaptopScanFailingInSetupKeepsOtherGroups(t *testing.T) {
	script := `mkdir -p "$KAYIT/run-en-hizli-ayar"; echo '{"onnxruntime": {"threads": 6}}' > "$AYAR"
go() { :; }; export -f go
bash "$HERE/en-hizli-ayar.sh" "llama.cpp read"; echo "rc=$?"
echo "ayar: $(cat "$AYAR")"`
	env := []string{"PY=" + filepath.ToSlash(python(t)), "OLCU_MODELLER=" + filepath.ToSlash(t.TempDir())}
	for _, e := range [][]string{env, append(env, "OLCU_DENEME=1")} {
		out, code := libRun(t, t.TempDir(), e, script)
		for _, want := range []string{"rc=2", "model qwen3-4b-q4km hazır değil", `ayar: {"onnxruntime": {"threads": 6}}`} {
			if code != 0 || !strings.Contains(out, want) {
				t.Errorf("%v: exit %d, want %q\n%s", e[len(e)-1], code, want, out)
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

// K4 (H1e): ölçü kipinde olcu_ayar_hazir seçilen kalemlerin eksik gruplarını koşturucuya sorar (--settings-missing) ve
// tarama betiğini yalnız o gruplarla, kendi iş klasöründe ($IS-tara) koşar. Eksik yoksa taramaz; koşturucu durursa
// (tek koşulu grup) ya da tarama düşerse ölçü yok; deneme hiçbir şey sormaz. Koşturucu ve tarama betiği sahte.
func TestLaptopMeasureScansOnlyMissingGroups(t *testing.T) {
	repo := t.TempDir()
	runPy := filepath.Join(repo, "tools", "bench", "baseline", "run.py")
	fake := "import os, sys\nopen(os.environ['FAKE_ARGS'], 'a').write(' '.join(sys.argv[1:]) + '\\n')\n" +
		"print(os.environ.get('FAKE_OUT', ''))\nsys.exit(int(os.environ.get('FAKE_RC', '0')))\n"
	if err := os.MkdirAll(filepath.Dir(runPy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runPy, []byte(fake), 0o644); err != nil {
		t.Fatal(err)
	}
	scan := filepath.Join(repo, "tara.sh")
	if err := os.WriteFile(scan, []byte("printf '%s|' \"$OLCU_IS\" \"$@\" >> \"$FAKE_TARA\"\nexit ${FAKE_TARA_RC:-0}\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, out, rc, scanRC, trial string
		code                         int
		scanned, asked               bool
	}{
		{"two missing", "MISSING llama.cpp read\nMISSING llama.cpp gen", "0", "0", "", 0, true, true},
		{"none missing", "", "0", "0", "", 0, false, true},
		{"a trial's group", "STOPPED: en-hizli.json's llama.cpp read has fewer than 3 runs", "3", "0", "", 2, false, true},
		{"scan fails", "MISSING whisper.cpp", "0", "2", "", 2, true, true},
		{"trial", "MISSING llama.cpp read", "0", "0", "1", 0, false, false},
	} {
		dir := t.TempDir()
		args, tara := filepath.Join(dir, "args.txt"), filepath.Join(dir, "tara.txt")
		env := []string{"PY=" + filepath.ToSlash(python(t)), "TARA_BETIK=" + filepath.ToSlash(scan), "FAKE_ARGS=" + args,
			"FAKE_TARA=" + tara, "FAKE_OUT=" + c.out, "FAKE_RC=" + c.rc, "FAKE_TARA_RC=" + c.scanRC, "OLCU_DENEME=" + c.trial}
		out, code := libRun(t, repo, env, `olcu_ayar_hazir gen-q S1 --key S1/qwen3-4b-q4km`)
		gotArgs, _ := os.ReadFile(args)
		gotScan, _ := os.ReadFile(tara)
		if code != c.code || (len(gotScan) > 0) != c.scanned || (len(gotArgs) > 0) != c.asked {
			t.Errorf("%s: exit %d (want %d), scanned %q, asked %q\n%s", c.name, code, c.code, gotScan, gotArgs, out)
			continue
		}
		if c.name == "two missing" && !strings.HasSuffix(string(gotScan), "/is-tara|llama.cpp read|llama.cpp gen|") {
			t.Errorf("scan %q, want only the two groups in the scan's own work folder", gotScan)
		}
		for _, want := range []string{"--question S1 --key S1/qwen3-4b-q4km --settings ", "--sources ", "--machine-file ", "gen-q-makine.txt", "--settings-missing"} {
			if c.asked && !strings.Contains(string(gotArgs), want) {
				t.Errorf("%s: run.py %q, want %q", c.name, gotArgs, want)
			}
		}
	}
}
