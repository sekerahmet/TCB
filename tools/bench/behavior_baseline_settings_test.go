// Layer: tools (tools); not shipped to users.
package bench

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// Ayar taramasının geçerliliği (H1e, TCB 2026-11-23 18:30): ölçü yalnız kendi kalemlerinin gruplarını ister (K1, K7);
// grup, makine ya da emsal değişene kadar geçerlidir (K2, K3); kimliği olmayan eski dosya kendi taramasının kaydıyla
// benimsenir (K9).

// settingsHead koşturucuyu ve soru kaydını yükler, sahte araçlarla bir iş klasörü kurar.
const settingsHead = `
import importlib.util, json, os, shutil, subprocess, sys, tempfile
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
`

// K1 (DENETİM 960 Ş2, N2): grup, ayarın okunduğu yerde ve ayar olsun olmasın kaydedilir. Bütün gruplar doluyken 45
// kalem, 15'inin grubu var; dosya boşken S7'nin ort-default ve ort-6 kalemleriyle 49 kalem ve S3 yine iki grupla.
func TestBaselineItemScanGroups(t *testing.T) {
	check := settingsHead + `
def groups(settings):
    ctx = qs.Ctx(rp, "laptop", 97, work, lambda r: None, dry=True)
    ctx.settings = settings
    return {it["key"]: qs.scan_groups(ctx, it) for q in qs.QUESTIONS["laptop"] for it in qs.items(ctx, q)}
full = {g: ({"flash_attn": 0} if g == "llama.cpp S3" else {"threads": 0}) for g in rp.SCAN_GROUPS}
print(json.dumps({"full": groups(full), "empty": groups({})}))
`
	out, err := exec.Command(python(t), "-I", "-c", check).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got struct{ Full, Empty map[string][]string }
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	llama := []string{"llama.cpp gen", "llama.cpp read"}
	want := map[string][]string{"S3/qwen3-4b-q4km": {"llama.cpp S3", "llama.cpp read"}, "S7/tr": {"onnxruntime"}, "S7/en": {"onnxruntime"}}
	for _, m := range []string{"qwen3-4b-q4km", "qwen3-4b-q8", "gemma3-4b-q4km", "gemma3-4b-qat-q40", "qwen3-8b-q4km", "gemma3-12b-q4km"} {
		want["S1/"+m] = llama
	}
	for _, lang := range []string{"tr", "en"} {
		want["S5/"+lang+"/faster-whisper-float32"] = []string{"faster-whisper float32"}
		want["S5/"+lang+"/faster-whisper-int8"] = []string{"faster-whisper int8"}
		want["S5/"+lang+"/whisper.cpp"] = []string{"whisper.cpp"}
	}
	for name, items := range map[string]map[string][]string{"full": got.Full, "empty": got.Empty} {
		n := map[string]int{"full": 45, "empty": 49}[name]
		if len(items) != n {
			t.Errorf("%s: %d items, want %d", name, len(items), n)
		}
		for key, groups := range items {
			if w := want[key]; !reflect.DeepEqual(groups, w) && !(len(groups) == 0 && len(w) == 0) {
				t.Errorf("%s: %s reads %v, want %v", name, key, groups, w)
			}
		}
	}
	if _, ok := got.Empty["S7/tr/ort-default"]; !ok {
		t.Errorf("empty file: no S7/tr/ort-default item")
	}
}

// identityCheck sahte sources.json'la ve makine kaydıyla kimliği kurar ve settings_state'i her alanın değişimiyle koşar.
const identityCheck = settingsHead + `
d = tempfile.mkdtemp()
src = {"tools": {"llama.cpp": {"version": "build 1", "files": {"a.exe": "a" * 64}}},
       "python": {"faster-whisper": {"version": "1.0", "sha256": "b" * 64}, "ctranslate2": {"version": "2.0", "sha256": "c" * 64}}}
def files(src, cpu="CPU X", logical="8"):
    s, m = tempfile.mktemp(dir=d), tempfile.mktemp(dir=d)
    json.dump(src, open(s, "w"))
    open(m, "w", encoding="utf-8").write(f"tarih: x\ncommit: c 0 değişik dosya\nişlemci: {cpu}\nmantıksal işlemci: {logical}\n")
    return s, m
s, m = files(src)
ident = lambda g, machine="laptop", s=s, m=m: rp.group_identity(g, machine, m, s)
def doc(runs=3, **change):
    out = {"llama.cpp read": {"threads": 20}, "faster-whisper int8": {"threads": 6}, "_scan": {}}
    for g in ("llama.cpp read", "faster-whisper int8"):
        i = json.loads(json.dumps(ident(g)))
        i.update(change)
        out["_scan"][g] = {"runs": runs, "chosen": 1, "identity": i}
    return out
def state(dc, groups=("llama.cpp read", "faster-whisper int8"), identity=ident):
    missing, reason, adopted = rp.settings_state(dc, list(groups), d, identity)
    return {"missing": sorted(missing), "reason": reason}
other = json.loads(json.dumps(src)); other["tools"]["llama.cpp"]["version"] = "build 2"
sha = json.loads(json.dumps(src)); sha["tools"]["llama.cpp"]["files"]["a.exe"] = "f" * 64
pkg = json.loads(json.dumps(src)); pkg["python"]["ctranslate2"]["version"] = "2.1"
cases = {"same": state(doc()), "machine": state(doc(machine="highram")), "processor": state(doc(processor="CPU Y")),
         "logical": state(doc(logical="16")), "python": state(doc(python="3.0.0"), ["faster-whisper int8"]),
         "version": state(doc(), identity=lambda g: ident(g, s=files(other)[0])),
         "sha256": state(doc(), identity=lambda g: ident(g, s=files(sha)[0])),
         "package": state(doc(), identity=lambda g: ident(g, s=files(pkg)[0])),
         "one run": state(doc(runs=1)), "no record": state({"llama.cpp read": {"threads": 6}, "_scan": {"llama.cpp read": {"runs": 3}}}, ["llama.cpp read"]),
         "not scanned": state({}), "no identity": state(doc(), identity=lambda g: None)}
cases["fields"] = sorted(ident("faster-whisper int8"))
print(json.dumps(cases))
`

// K2, K3: grubun seçimi makine ve emsal değişene kadar geçerlidir; kimliğin her alanı tek tek değişince grup eksik
// sayılır (DENETİM 960 N4). Tek koşulu grup (denemenin taraması) ölçüyü durdurur; kimlik okunamazsa ölçü durur.
func TestBaselineScanHoldsUntilIdentityChanges(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", identityCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var fields []string
	_ = json.Unmarshal(got["fields"], &fields)
	if !reflect.DeepEqual(fields, []string{"logical", "machine", "processor", "python", "references"}) {
		t.Errorf("identity fields %v", fields)
	}
	for name, want := range map[string]struct {
		missing []string
		reason  string
	}{
		"same": {nil, ""}, "machine": {[]string{"faster-whisper int8", "llama.cpp read"}, ""},
		"processor": {[]string{"faster-whisper int8", "llama.cpp read"}, ""}, "logical": {[]string{"faster-whisper int8", "llama.cpp read"}, ""},
		"python": {[]string{"faster-whisper int8"}, ""}, "version": {[]string{"llama.cpp read"}, ""}, "sha256": {[]string{"llama.cpp read"}, ""},
		"package": {[]string{"faster-whisper int8"}, ""}, "one run": {nil, "fewer than 3 runs (a trial's scan)"},
		"no record": {[]string{"llama.cpp read"}, ""}, "not scanned": {[]string{"faster-whisper int8", "llama.cpp read"}, ""},
		"no identity": {nil, "the scan's identity is not known"},
	} {
		var c struct {
			Missing []string
			Reason  *string
		}
		if err := json.Unmarshal(got[name], &c); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		reason := ""
		if c.Reason != nil {
			reason = *c.Reason
		}
		if !reflect.DeepEqual(c.Missing, want.missing) && !(len(c.Missing) == 0 && len(want.missing) == 0) ||
			(want.reason == "") != (reason == "") || !strings.Contains(reason, want.reason) {
			t.Errorf("%s: missing %v, stop %q; want %v, %q", name, c.Missing, reason, want.missing, want.reason)
		}
	}
}

// adoptCheck 2026-11-22 dosyasının ve kaydının biçimini sahte değerlerle kurar (DENETİM 960 N5: gerçek kayıt yol ve makine
// bilgisi taşır) ve her bozulmuş kayıtla benimsemeyi koşar.
const adoptCheck = settingsHead + `
src = {"tools": {"llama.cpp": {"version": "build 1, commit 9", "files": {"a.exe": "a" * 64, "b.dll": "d" * 64}}},
       "python": {"faster-whisper": {"version": "1.0", "sha256": "b" * 64}, "ctranslate2": {"version": "2.0", "sha256": "c" * 64}}}
py = sys.version.split()[0]
table = {"llama.cpp read": {"medians_ms": {"0": 5.0, "6": 7.0, "20": 4.0}, "measures_ms": {}, "runs": 3, "chosen": 20},
         "faster-whisper int8": {"medians_ms": {"0": 3.0, "6": 2.0, "20": 4.0}, "measures_ms": {}, "runs": 3, "chosen": 6}}
prep = ["llama.cpp a.exe: sha256 aaaaaaaaaaaa tuttu", "llama.cpp b.dll: sha256 dddddddddddd tuttu",
        "llama.cpp sürüm: version: 0.9 (build 1, commit 9) (https://x)",
        "faster-whisper 1.0 (fw.whl, sha256 bbbbbbbbbbbb)", "ctranslate2 2.0 (ct.whl, sha256 cccccccccccc)"]
def case(change=lambda k: None, cpu="CPU X"):
    k = tempfile.mkdtemp()
    r = os.path.join(k, "run-en-hizli-ayar")
    os.makedirs(r)
    f = {"doc": {"llama.cpp read": {"threads": 20}, "faster-whisper int8": {"threads": 6}, "_commit": "c0ffee",
                 "_scan": json.loads(json.dumps(table))},
         "log": [{"time": "t", "scan": g, **t} for g, t in table.items()],
         "machine": "tarih: 2026-11-22 17:29:28 +0300\ncommit: c0ffee 0 değişik dosya\nişlemci: CPU X\nmantıksal işlemci: 8\n",
         "prep": list(prep), "pip": "ctranslate2==2.0\nfaster-whisper==1.0\n", "py": "Python " + py}
    change(f)
    json.dump(f["doc"], open(os.path.join(k, "en-hizli.json"), "w"))
    open(os.path.join(k, "en-hizli-ayar-makine.txt"), "w", encoding="utf-8").write(f["machine"])
    open(os.path.join(r, "log.jsonl"), "w", encoding="utf-8").write("".join(json.dumps(x) + "\n" for x in f["log"]))
    open(os.path.join(r, "hazirlik.txt"), "w", encoding="utf-8").write("\n".join(f["prep"]) + "\n")
    open(os.path.join(r, "pip_list.txt"), "w").write(f["pip"])
    open(os.path.join(r, "python_version.txt"), "w").write(f["py"])
    s, m = os.path.join(k, "sources.json"), os.path.join(k, "olcu-makine.txt")
    json.dump(src, open(s, "w"))
    open(m, "w", encoding="utf-8").write(f"tarih: x\ncommit: 1 0 değişik dosya\nişlemci: {cpu}\nmantıksal işlemci: 8\n")
    doc = f["doc"]
    missing, reason, adopted = rp.settings_state(doc, ["faster-whisper int8", "llama.cpp read"], k,
                                                 lambda g: rp.group_identity(g, "laptop", m, s))
    t = doc["_scan"].get("llama.cpp read", {})
    return {"missing": missing, "reason": reason, "adopted": adopted, "identity": "identity" in t,
            "record": t.get("adopted")}
def edit(path, fn):
    def change(f):
        f[path] = fn(f[path])
    return change
def first_line(f):
    f["log"][0] = dict(f["log"][0], chosen=0)
print(json.dumps({
    "ok": case(),
    "machine": case(cpu="CPU Y"),
    "sha256": case(edit("prep", lambda p: [x.replace("dddddddddddd", "eeeeeeeeeeee") for x in p])),
    "version": case(edit("prep", lambda p: [x for x in p if "sürüm" not in x])),
    "table": case(first_line),
    "commit": case(edit("machine", lambda m: m.replace("c0ffee", "beef"))),
    "package": case(edit("prep", lambda p: [x for x in p if not x.startswith("ctranslate2")])),
    "pip": case(edit("pip", lambda p: "faster-whisper==1.0\n")),
    "python": case(edit("py", lambda p: "Python 2.7.0")),
}))
`

// K9 (TCB 2026-11-23 18:32, 18:47): kimliği olmayan eski dosya, taramanın kendi kaydı o taramanınsa (günlük satırı
// dosyanın tablosu, makine kaydının commit'i dosyanınki) ve aynı makineyi ve emsali gösteriyorsa benimsenir; gruba kimlik
// ve kayıt yazılır. Kaydın her parçası tek tek bozulunca o grup eksik sayılır (DENETİM 960 N1, N4).
func TestBaselineOldScanIsAdoptedFromItsRecord(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", adoptCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got map[string]struct {
		Missing  map[string]string
		Reason   *string
		Adopted  []string
		Identity bool
		Record   map[string]string
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	ok := got["ok"]
	if len(ok.Missing) != 0 || ok.Reason != nil || len(ok.Adopted) != 2 || !ok.Identity ||
		ok.Record["record"] != "2026-11-22 17:29:28 +0300" || ok.Record["commit"] != "c0ffee" || ok.Record["rule"] != "median" {
		t.Errorf("the record of this machine and reference: %+v, want both groups adopted with their record", ok)
	}
	for name, want := range map[string]struct{ group, why string }{
		"machine": {"llama.cpp read", "another machine: CPU X"}, "sha256": {"llama.cpp read", "b.dll: no sha256 dddddddddddd"},
		"version": {"llama.cpp read", "no version 'build 1, commit 9'"}, "table": {"llama.cpp read", "scan line is not the file's table"},
		"commit": {"llama.cpp read", "the record's commit beef"}, "package": {"faster-whisper int8", "ctranslate2 2.0: not in"},
		"pip":    {"faster-whisper int8", "ctranslate2==2.0 is not in the record's pip list"},
		"python": {"faster-whisper int8", "the record's Python 2.7.0"},
	} {
		c := got[name]
		if !strings.Contains(c.Missing[want.group], want.why) || c.Reason != nil {
			t.Errorf("%s: %s missing %q, stop %v; want %q", name, want.group, c.Missing[want.group], c.Reason, want.why)
		}
	}
}

// gateCheck ölçünün kapısını komut satırından koşar: S3'ün "llama.cpp S3"'ü olmayan, olmayan ve tek koşulu dosyayla.
const gateCheck = settingsHead + `
k = tempfile.mkdtemp()
src = {"tools": {"llama.cpp": {"version": "build 1", "files": {"a.exe": "a" * 64}}}}
s, m = os.path.join(k, "sources.json"), os.path.join(k, "m.txt")
json.dump(src, open(s, "w"))
open(m, "w", encoding="utf-8").write("işlemci: CPU X\nmantıksal işlemci: 8\n")
read = rp.group_identity("llama.cpp read", "laptop", m, s)
def run(doc, *args):
    p = os.path.join(tempfile.mkdtemp(), "en-hizli.json")
    if doc is not None:
        json.dump(doc, open(p, "w"))
    r = subprocess.run([sys.executable, "-I", "baseline/run.py", "--work", work, "--machine", "laptop", "--out", tempfile.mkdtemp(),
                        "--settings", p, "--sources", s, "--machine-file", m, *args],
                       capture_output=True, text=True, encoding="utf-8", stdin=subprocess.DEVNULL)
    return {"rc": r.returncode, "out": r.stdout}
no_s3 = {"llama.cpp read": {"threads": 20}, "_scan": {"llama.cpp read": {"runs": 3, "identity": read}}}
trial = {"llama.cpp read": {"threads": 20}, "_scan": {"llama.cpp read": {"runs": 1, "identity": read}}}
s3 = ["--question", "S3", "--key", "S3/qwen3-4b-q4km"]
print(json.dumps({"no S3": run(no_s3, *s3), "no S3 missing": run(no_s3, *s3, "--settings-missing"),
                  "no file missing": run(None, *s3, "--settings-missing"), "no file S14": run(None, "--question", "S14"),
                  "one run": run(trial, *s3, "--settings-missing"), "trial": run(None, *s3, "--trial")}))
`

// K7 (DENETİM 960 Ş2, 947 Ş1): ölçü, seçilen kalemin bir grubu dosyada geçerli değilse durur; dosyanın olmaması boş dosya
// sayılır. --settings-missing eksik grupları yazar (lib.sh onları tarar, K4). Grubu olmayan kalem (S14) dosyasız ölçer;
// deneme kapıdan geçmez.
func TestBaselineMeasureStopsWithoutItsGroups(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", gateCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got map[string]struct {
		Rc  int
		Out string
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if c := got["no S3"]; c.Rc != 3 || !strings.Contains(c.Out, "STOPPED: en-hizli.json has no valid setting for {'llama.cpp S3': 'not scanned'}") {
		t.Errorf("S3 without its -fa group: exit %d, want 3\n%s", c.Rc, c.Out)
	}
	if c := got["no S3 missing"]; c.Rc != 0 || c.Out != "MISSING llama.cpp S3\n" {
		t.Errorf("--settings-missing without S3's group: exit %d\n%q", c.Rc, c.Out)
	}
	if c := got["no file missing"]; c.Rc != 0 || c.Out != "MISSING llama.cpp S3\nMISSING llama.cpp read\n" {
		t.Errorf("--settings-missing without a file: exit %d\n%q", c.Rc, c.Out)
	}
	if c := got["one run"]; c.Rc != 3 || !strings.Contains(c.Out, "fewer than 3 runs") {
		t.Errorf("a trial's one-run group: exit %d, want 3\n%s", c.Rc, c.Out)
	}
	for _, name := range []string{"no file S14", "trial"} {
		if c := got[name]; strings.Contains(c.Out, "STOPPED: en-hizli") || strings.Contains(c.Out, "MISSING") {
			t.Errorf("%s: the gate stopped it\n%s", name, c.Out)
		}
	}
}
