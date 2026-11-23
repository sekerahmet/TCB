// Layer: tools (tools); not shipped to users.
package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
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

// Yalnız belleği ölçen soru (S4: uzun bağlam ve model belleği) bir ısınma + 2 koşu koşar (Ahmet 2026-11-22 20:45: bellek
// için 5 koşu gerekmez); sayı betikte değil koşturucunun sorusunda, koşudan önce yazılı. Öbür emsalsiz sorular değişmez.
func TestBaselineMemoryQuestionsRunTwice(t *testing.T) {
	work := t.TempDir()
	tools := `{"trlang": "trlang", "llama_bench": "llama-bench", "llama_completion": "llama-completion", "whisper_cli": "whisper-cli", "go_compare": "compare"}`
	if err := os.WriteFile(filepath.Join(work, "tools.json"), []byte(tools), 0o644); err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]struct{ Runs int }
	selfCheck(t, &got, "items", work)
	want := map[string]int{"S4": 2, "S8": 3, "S9": 3, "S10": 3, "S2": 5, "S6": 5, "S14": 5}
	seen := map[string]bool{}
	for key, it := range got["laptop"] {
		q := strings.Split(key, "/")[0]
		if n, ok := want[q]; ok {
			seen[q] = true
			if it.Runs != n {
				t.Errorf("laptop %s: %d runs, want %d", key, it.Runs, n)
			}
		}
	}
	if len(seen) != len(want) {
		t.Errorf("questions seen %v, want all of %v", seen, want)
	}
}

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

// S13'ün saf hesap işleri (TCB 2026-11-23 10:26, madde 2): TR programı --n'i okur ve Go'nun compare'iyle aynı değeri
// verir (compare_test.go aynı boyların Go değerini bağımsız hesapla denetler); S13'ün iki tarafı aynı büyük boyu alır.
func TestBaselinePureComputeTakesTheSize(t *testing.T) {
	for _, c := range []struct{ prog, n, want string }{{"dongu", "10", "45"}, {"fib", "10", "55"}, {"matris_dongu", "10", "443/4380"},
		{"dongu", "1000", "499500"}} {
		out, err := exec.Command(trlang(t), "tr/"+c.prog+".tr", "--n", c.n).CombinedOutput()
		f := strings.Fields(string(out))
		if err != nil || len(f) != 3 || f[0] != c.prog || f[1] != c.want {
			t.Errorf("%s --n %s: %v %q, want the value %s", c.prog, c.n, err, out, c.want)
		}
	}
	work := t.TempDir()
	tools := `{"trlang": "trlang", "llama_bench": "llama-bench", "llama_completion": "llama-completion", "whisper_cli": "whisper-cli", "go_compare": "compare"}`
	if err := os.WriteFile(filepath.Join(work, "tools.json"), []byte(tools), 0o644); err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]struct{ TR, Ref [][]string }
	selfCheck(t, &got, "items", work)
	for prog, n := range map[string]string{"dongu": "300000000", "fib": "37", "matris_dongu": "480"} {
		it := got["laptop"]["S13/"+prog]
		tr, ref := strings.Join(it.TR[0], " "), strings.Join(it.Ref[0], " ")
		if !strings.HasSuffix(tr, prog+".tr --n "+n) || ref != "compare "+prog+" --n "+n {
			t.Errorf("S13/%s: TR %q, Go %q; want both with --n %s", prog, tr, ref, n)
		}
	}
	// DENETİM 953 Ş2: uyuşma matrisin bütün değerini (iz ve toplam) karşılaştırır; köşegen dışı tutmazsa uyuşmaz.
	agree := `
import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
qs = rp.load_questions()
ctx = qs.Ctx(rp, "laptop", 97, sys.argv[1], lambda r: None, dry=True)
it = next(i for i in qs.items(ctx, "S13") if i["key"] == "S13/matris_dongu")
v = lambda s: [{"value": s}]
print(json.dumps({"same": it["agree"](v("443/4380"), v("443/4380")), "off": it["agree"](v("443/4380"), v("443/4381"))}))
`
	out, err := exec.Command(python(t), "-I", "-c", agree, work).Output()
	var a map[string]*string
	if err != nil || json.Unmarshal(out, &a) != nil || a["same"] != nil || a["off"] == nil {
		t.Errorf("S13/matris_dongu's agreement: %v %s; want the same value to agree and another sum to disagree", err, out)
	}
}

// idsCheck S7'nin Türkçe kimlik hazırlığını sahte koşucuyla, --key'e göre koşar.
const idsCheck = `
import importlib.util, json, sys, tempfile, types
spec = importlib.util.spec_from_file_location("questions", "baseline/questions.py")
qs = importlib.util.module_from_spec(spec)
spec.loader.exec_module(qs)
out = {}
for name, keys in (("en", ["S7/en"]), ("tr", ["S7/tr"]), ("all", None), ("question", ["S7/"])):
    ran = []
    rp = types.SimpleNamespace(run_once=lambda cmd: ran.append(cmd) or {"rc": 0}, Stop=Exception)
    ctx = types.SimpleNamespace(dry=False, work=tempfile.mkdtemp(), keys=keys, rp=rp, tools={"trlang": "trlang"},
                                tr_device="cpu", tr=lambda *a: ["trlang", *a], model=lambda n: "models/" + n)
    qs.piper_ids_tr(ctx)
    out[name] = len(ran)
print(json.dumps(out))
`

// BC28: S7/en tek başına koşunca Türkçe sesin kimlikleri yazılmaz (Türkçe ses bağlanmaz); S7/tr ya da bütün soru
// koşunca yazılır.
func TestBaselineS7EnglishWritesNoTurkishIds(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", idsCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got map[string]int
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got["en"] != 0 || got["tr"] != 1 || got["all"] != 1 || got["question"] != 1 {
		t.Errorf("ids written %v; want none for S7/en alone, once for S7/tr, the whole run and S7/", got)
	}
}

// scriptRuns bir dizüstü betiğinin bağladığı modelleri ve koştuğu kalemleri okur: olcu_model satırları (for döngüsü
// dahil), olcu_kos'un sorusu ve --key'leri (anahtar ilk $'da kesilir, kalanı önektir), olcu_tara (Ş1'in taraması).
func scriptRuns(body string) (models []string, runs [][2]string, scan bool) {
	for _, line := range strings.Split(body, "\n") {
		f := strings.Fields(strings.NewReplacer("for m in", "olcu_model", "; do olcu_model \"$m\"; done", "").Replace(line))
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		switch f[0] {
		case "olcu_model":
			models = append(models, f[1:]...)
		case "olcu_tara":
			scan = true
		case "olcu_kos":
			if len(f) < 3 {
				continue
			}
			keys := 0
			for i := 3; i+1 < len(f); i++ {
				if f[i] == "--key" {
					k := strings.Trim(f[i+1], `"`)
					if j := strings.Index(k, "$"); j >= 0 {
						k = k[:j]
					}
					runs = append(runs, [2]string{f[2], k})
					keys++
				}
			}
			if keys == 0 {
				runs = append(runs, [2]string{f[2], ""})
			}
		}
	}
	return models, runs, scan
}

// modelsIn bir komut listesinin iş klasöründen okuduğu modellerin adları (work/models/<ad>/...).
func modelsIn(cmds [][]string) map[string]bool {
	out := map[string]bool{}
	for _, cmd := range cmds {
		for _, a := range cmd {
			a = filepath.ToSlash(a)
			if i := strings.Index(a, "/models/"); i >= 0 {
				out[strings.SplitN(a[i+len("/models/"):], "/", 2)[0]] = true
			}
		}
	}
	return out
}

// BC28: dizüstü betiği yalnız kullandığı modeli bağlar: bağladığı her model, koştuğu kalemlerin bir komutunda (uyuşma,
// uyuşmanın kendi koştuğu emsaller, hazırlık ve Dost'un komutu dahil) geçer; komutların okuduğu her model de bağlıdır.
func checkScriptsBindTheirModels(t *testing.T, scripts []string) {
	t.Helper()
	work := t.TempDir()
	tools := `{"trlang": "trlang", "llama_bench": "llama-bench", "llama_completion": "llama-completion", "whisper_cli": "whisper-cli", "go_compare": "compare"}`
	if err := os.WriteFile(filepath.Join(work, "tools.json"), []byte(tools), 0o644); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Items map[string][][]string
		Scan  [][]string
	}
	selfCheck(t, &got, "uses", work)
	checked := 0
	for _, s := range scripts {
		body, _ := os.ReadFile(s)
		models, runs, scan := scriptRuns(string(body))
		var cmds [][]string
		if scan {
			cmds = append(cmds, got.Scan...)
		}
		for _, r := range runs {
			for key, uses := range got.Items {
				if strings.Split(key, "/")[0] == r[0] && strings.HasPrefix(key, r[1]) {
					cmds = append(cmds, uses...)
				}
			}
		}
		used := modelsIn(cmds)
		bound := map[string]bool{}
		for _, m := range models {
			bound[m] = true
			if !used[m] {
				t.Errorf("%s binds %s, which no command of its items reads (BC28); they read %v", filepath.Base(s), m, used)
			}
		}
		for m := range used {
			if !bound[m] {
				t.Errorf("%s: its items read %s, which it does not bind", filepath.Base(s), m)
			}
		}
		checked += len(runs)
	}
	if checked < 35 {
		t.Errorf("%d olcu_kos runs read from %d scripts; the parser lost them", checked, len(scripts))
	}
}

// shortCheck en kısa ölçülebilir sürenin denetimini (run.py too_short) üç türde sınırın iki yanında, S4'ün belleğinde,
// Linux makinesinde, emsalsiz koşuda ve sıfır ms'lik emsalde koşar.
const shortCheck = `
import importlib.util, json, sys, types
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
out = {}
def call(name, side, measures, runs=None, machine="laptop"):
    try:
        out[name] = rp.too_short("Q", side, measures, None, runs, machine)
    except rp.Stop as e:
        out[name] = "stop: " + str(e)
dur = {"name": "TR"}
call("duration under", dur, {"ms": 19.9})
call("duration over", dur, {"ms": 20.1})
call("duration at", dur, {"ms": 20.0})
der = {"name": "TR", "kinds": {"ms": {"kind": "derived", "what": "two walls",
                                      "span": lambda r, runs: runs[0]["wall_ms"] - runs[1]["wall_ms"]}}}
call("derived under", der, {"ms": 0.06}, [{"wall_ms": 1000}, {"wall_ms": 981}])
call("derived over", der, {"ms": 0.08}, [{"wall_ms": 1000}, {"wall_ms": 975}])
call("derived at", der, {"ms": 0.07}, [{"wall_ms": 1000}, {"wall_ms": 980}])
nd = {"name": "TR", "kinds": {"missed": {"kind": "not_duration", "why": "a count"}}}
call("not duration under", nd, {"missed": 0})
call("not duration over", nd, {"missed": 30})
mem = {"name": "TR", "short_ok": "memory"}
call("memory under", mem, {"load_ms": 0.5})
call("memory over", mem, {"load_ms": 25.0})
call("linux under", dur, {"ms": 0.5}, machine="highram")
vals = iter([0.4, 1.3, 0.0])
rp._run_side = lambda q, side, index, log, machine: {"results": [{"load_ms": next(vals)}], "runs": [{"peak_working_set_kb": 9}]}
side = {"name": "TR", "short_ok": "memory", "measure": lambda r, runs: {"load_ms": r[0]["load_ms"]}}
out["alone"] = rp.run_alone("S4/x/0", side, lambda r: None, "laptop", runs=2)["measures"]["load_ms"]
rp._run_side = lambda q, side, index, log, machine, cmds=None: {"results": [{"v": 5.0 if side["name"] == "TR" else 0.0}], "runs": [{}]}
pick = lambda r, runs: r[0]["v"]
try:
    rp.run_pair("S13/fib", {"name": "TR", "measure": pick}, {"name": "Go", "measure": pick}, None, lambda r: None, "highram")
    out["zero"] = "no stop"
except rp.Stop as e:
    out["zero"] = str(e)
rp._run_side = lambda q, side, index, log, machine, cmds=None: {"results": [{"v": 0.0 if side["name"] == "TR" else 5.0}], "runs": [{}]}
try:
    rp.run_pair("S13/fib", {"name": "TR", "measure": pick}, {"name": "Go", "measure": pick}, None, lambda r: None, "highram")
    out["tr zero"] = "no stop"
except rp.Stop as e:
    out["tr zero"] = str(e)
print(json.dumps(out))
`

// BC27 (TCB 2026-11-23 10:33, H1 soru 1): dizüstünde (Windows'un saat adımı ~0,5 ms) her ölçünün okunduğu saat aralığı
// 20 ms'yle karşılaştırılır: süre değerin kendisiyle, türetilmiş süre okunduğu aralıkla (karakter modeli: iki koşunun duvar
// farkı), süre olmayan ölçü (sayı, gecikme, hız) denetlenmez. Altında kalem durur, satır basılmaz; yalnız belleği ölçen S4'te
// ölçü "ölçülemeyecek kadar kısa" diye işaretlenir, bellek kalır. Linux makinesinde sınır yok; 0 ms'lik emsal orana girmez.
func TestBaselineShortestMeasurableTime(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", shortCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for name, want := range map[string]string{"duration under": "TR ms: 19.900 ms read, too short to measure (ölçülemeyecek kadar kısa)",
		"derived under": "TR ms: 19.000 ms read (two walls), too short to measure"} {
		if s, _ := got[name].(string); !strings.Contains(s, want) {
			t.Errorf("%s: %v, want a stop with %q", name, got[name], want)
		}
	}
	// DENETİM 953 N1: sınırın tam üstü (20,0 ms) geçer.
	for _, pass := range []string{"duration over", "duration at", "derived over", "derived at", "not duration under", "not duration over",
		"memory over", "linux under"} {
		if l, ok := got[pass].([]any); !ok || len(l) != 0 {
			t.Errorf("%s: %v, want no stop and nothing marked", pass, got[pass])
		}
	}
	if l, ok := got["memory under"].([]any); !ok || len(l) != 1 || l[0] != "load_ms" {
		t.Errorf("memory under: %v, want load_ms marked, no stop", got["memory under"])
	}
	alone, _ := got["alone"].(map[string]any)
	if mark, _ := alone["too_short"].(map[string]any); mark == nil || mark["limit_ms"] != 20.0 {
		t.Errorf("S4's short load: %v, want it marked too short with the 20 ms limit", alone)
	}
	if s, _ := got["zero"].(string); !strings.Contains(s, "Go ms measured 0 ms") {
		t.Errorf("a 0 ms reference: %v, want a stop before the ratio", got["zero"])
	}
	// DENETİM 953 N2: TR'nin 0 ms'si de Linux'ta durur; oran 0 olup "TR daha iyi" demez.
	if s, _ := got["tr zero"].(string); !strings.Contains(s, "TR ms measured 0 ms") {
		t.Errorf("a 0 ms TR side on Linux: %v, want a stop before the ratio", got["tr zero"])
	}
}

// DENETİM 953 N3: uzun bağlam betikleri deneme kipini lib.sh gibi yalnız OLCU_DENEME=1 ile tanır: denemede kısa bağlamın
// kalemi, öbür her değerde (yok, 0) bütün bağlamlar. Betiğin anahtar satırları sahte olcu_kos'la koşar.
func TestLaptopLongContextTrialKey(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash yok")
	}
	scripts, _ := reporoot.Glob(filepath.Join("baseline", "machines", "laptop", "long-context-memory-*.sh"))
	if len(scripts) != 5 {
		t.Fatalf("%d long-context scripts, want 5", len(scripts))
	}
	for _, s := range scripts {
		body, _ := os.ReadFile(s)
		var lines []string
		for _, l := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(l, "baglam=") || strings.HasPrefix(l, "olcu_kos ") {
				lines = append(lines, l)
			}
		}
		for env, trial := range map[string]bool{"": false, "OLCU_DENEME=0": false, "OLCU_DENEME=1": true} {
			cmd := exec.Command(bash, "-c", "set -u; olcu_kos() { echo \"${@: -1}\"; }; "+strings.Join(lines, "\n"))
			cmd.Env = os.Environ()
			if env != "" {
				cmd.Env = append(cmd.Env, env)
			}
			out, err := cmd.Output()
			key := strings.TrimSpace(string(out))
			if err != nil || strings.HasSuffix(key, "/") == trial || !strings.HasPrefix(key, "S4/") {
				t.Errorf("%s with %q: key %q (%v); want a context only when OLCU_DENEME=1", filepath.Base(s), env, key, err)
			}
		}
	}
}

// stopCheck dizüstünde sınırın altında bir ölçüyü eşli koşuya (S13 biçiminde: TR 200 ms, Go 5 ms), taramaya ve Dost'un
// özel koşusuna verir; her biri durmalı.
const stopCheck = `
import importlib.util, json, sys, types
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
rp.wait_for_quiet = lambda q, log: 0.0
rp.process_list = lambda: []
out = {}
def stops(name, f):
    try:
        f()
        out[name] = "no stop"
    except rp.Stop as e:
        out[name] = str(e)
ms = lambda r, runs: r[0]["ms"]
rp._run_side = lambda q, side, index, log, machine, cmds=None: {"results": [{"ms": 200.0 if side["name"] == "TR" else 5.0}], "runs": [{}]}
stops("pair", lambda: rp.run_pair("S13/dongu", {"name": "TR", "measure": ms}, {"name": "Go", "measure": ms}, None, lambda r: None, "laptop"))
ctx = types.SimpleNamespace(machine="laptop")
rp._run_side = lambda q, side, index, log, machine, cmds=None: {"results": [{"ms": 5.0}], "runs": [{}]}
stops("scan", lambda: rp.time_side(ctx, {"name": "whisper.cpp", "cmds": [["w"]]}, "S5/tr/whisper.cpp", lambda r: None,
                                   "scan 0", lambda r, runs: {"transcribe_ms": r[0]["ms"]}))
it = {"key": "S10/tr/warm", "runs": 3, "custom": lambda: ({"ready_ms": 5.0}, {})}
stops("custom", lambda: rp.run_custom(ctx, it, types.SimpleNamespace(log=lambda r: None)))
print(json.dumps(out))
`

// DENETİM 953 Ş1: sınır dört çağrı yerinin her birinde kalemi durdurur; emsalsiz koşu TestBaselineShortestMeasurableTime'da.
// Eşli koşuda (S13'ün asıl yolu), taramada ve özel koşuda çağrı silinince bu test düşer (devirde bir kez bozuldu).
func TestBaselineShortMeasureStopsEveryPath(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", stopCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for name, want := range map[string]string{"pair": "S13/dongu: Go ms: 5.000 ms read", "scan": "S5/tr/whisper.cpp: whisper.cpp transcribe_ms: 5.000 ms read",
		"custom": "S10/tr/warm: TR ready_ms: 5.000 ms read"} {
		if !strings.Contains(got[name], want) || !strings.Contains(got[name], "ölçülemeyecek kadar kısa") {
			t.Errorf("%s: %q, want a stop with %q", name, got[name], want)
		}
	}
}

// BC27: kısa diye işaretlenen ölçünün satırı sayı basmaz, nedenini söyler; bellek satırı kalır; Ayar sınırı yazar.
// Ölçülerin türü questions.py'de koşudan önce yazılıdır.
func TestBaselineShortMeasureRowAndKinds(t *testing.T) {
	dir := t.TempDir()
	metrics := `{"_machine": {"threads": 20, "threads_source": "gctrace P (O-1)", "shortest_measurable_ms": 20.0},
 "S4/x/0": {"reference": null, "measures": {"load_ms": {"tr_ms": [1.3, 0.0], "median": 0.65, "min": 0.0, "max": 1.3,
   "too_short": {"limit_ms": 20.0, "why": "S4 measures memory"}}}, "memory": [{"peak_working_set_kb": 9}]}}`
	if err := os.WriteFile(filepath.Join(dir, "metrics.json"), []byte(metrics), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, code := baseline(t, "--rows", "--out", dir)
	for _, want := range []string{"; shortest measurable 20 ms (BC27)", "S4/x/0 load_ms: — (ölçülemeyecek kadar kısa: a run under 20 ms; S4 measures memory)",
		"S4/x/0 memory TR peaks: {'peak_working_set_kb': 9}"} {
		if code != 0 || !strings.Contains(rows, want) {
			t.Errorf("--rows: exit %d, want %q\n%s", code, want, rows)
		}
	}
	if strings.Contains(rows, "0.650") || strings.Contains(rows, "1.300") {
		t.Errorf("--rows printed a too-short number\n%s", rows)
	}
	work := t.TempDir()
	tools := `{"trlang": "trlang", "llama_bench": "llama-bench", "llama_completion": "llama-completion", "whisper_cli": "whisper-cli", "go_compare": "compare"}`
	if err := os.WriteFile(filepath.Join(work, "tools.json"), []byte(tools), 0o644); err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]struct {
		Kinds   map[string]string
		ShortOK bool `json:"short_ok"`
	}
	selfCheck(t, &got, "items", work)
	laptop := got["laptop"]
	for key, want := range map[string]map[string]string{"S13/char_model": {"ms": "derived"},
		"S13/loop_100hz": {"missed": "not_duration", "worst_late_ms": "not_duration"},
		"S9/tr":          {"tokens_per_second": "not_duration"}, "S9/en": {"tokens_per_second": "not_duration"}, "S13/fib": {}} {
		if fmt.Sprint(laptop[key].Kinds) != fmt.Sprint(want) {
			t.Errorf("%s kinds %v, want %v", key, laptop[key].Kinds, want)
		}
	}
	for key, it := range laptop {
		if it.ShortOK != strings.HasPrefix(key, "S4/") {
			t.Errorf("%s: short_ok %v; only S4 (memory) lets a short measure pass", key, it.ShortOK)
		}
	}
}
