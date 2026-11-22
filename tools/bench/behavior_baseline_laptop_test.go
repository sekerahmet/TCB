// Layer: tools (tools); not shipped to users.
package bench

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Dizüstünün ölçü betiklerinde bulunan düzeltmelerin testleri (H1, 2026-11-20 ve DENETİM 944): yük denetiminin
// beklemesi ve koşturucunun kendi süreç ağacı, her koşunun gerçek aygıtı (Ş2), emsallerin her konsolda UTF-8 çıktısı (BC24),
// modelin ve emsalin tek kaynağı (Ş3), değişik dosyalı ağaç (Ş4), açık Dost (N1).

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
    ms = {"S1 read": {0: 50, 6: 30, os.cpu_count(): 40}, "S1 gen": {0: 20, 6: 25, os.cpu_count(): 10},
          "S5/tr/whisper.cpp": {0: 5, 6: 9, os.cpu_count(): 7}}[("S1 gen" if gen else "S1 read") if key.startswith("S1") else key][n]
    return {"results": [{"avg_ns": ms * 1e6}] * len(side["cmds"]), "runs": [{"stderr": "total time = %d ms\\nload time = 0 ms" % ms}]}
rp._run_side = fake
ctx.settings = {}
scan_out = tempfile.mkdtemp()
rec = type("R", (), {"log": lambda self, r: None})()
rp.scan_settings(ctx, qs, rec, ["llama.cpp read", "llama.cpp gen"], scan_out, runs=3)
rp.scan_settings(ctx, qs, rec, ["whisper.cpp"], scan_out, runs=1)
out["scan"] = json.load(open(os.path.join(scan_out, "en-hizli.json"), encoding="utf-8"))
whole = {g: {"threads": 6} for g in rp.SCAN_GROUPS}
whole["_scan"] = {g: {"runs": 3} for g in rp.SCAN_GROUPS}
out["check"] = {"whole": rp.check_settings(whole),
                "missing": rp.check_settings({k: v for k, v in whole.items() if k != "llama.cpp gen"}),
                "trial": rp.check_settings({**whole, "_scan": {g: {"runs": 1} for g in rp.SCAN_GROUPS}})}
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
		Check                        map[string]*string
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
	if r["threads"] != 6.0 || g["threads"] == 6.0 || g["threads"] == 0.0 || w["threads"] != 0.0 || len(table) != 3 || gt["measures_ms"] == nil {
		t.Errorf("scan: %v, want reading 6 threads, generation the logical count, whisper.cpp the default, each measure's median", got.Scan)
	}
	// DENETİM 947 Ş1: ölçü yalnız bütün gruplar 3 koşuyla taranmış dosyayı alır.
	if c := got.Check; c["whole"] != nil || c["missing"] == nil || !strings.Contains(*c["missing"], "llama.cpp gen") ||
		c["trial"] == nil || !strings.Contains(*c["trial"], "fewer than 3 runs") {
		t.Errorf("check_settings: whole %v, a group missing %v, a trial's file %v", c["whole"], c["missing"], c["trial"])
	}
}

// Denemede (--trial) uyuşması olmayan eşli kalem (S13'ün karakter modeli) iki tarafını birer kez koşar; uyuşması olan
// kalemi uyuşma geçişi dener. Düzeltmeden önce böyle bir kalemin denemesi hiçbir şey koşmadan çıkış 0 veriyordu.
func TestLaptopTrialRunsAPairWithoutAgreement(t *testing.T) {
	check := `
import importlib.util, json, sys, types
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
calls = []
rp._run_side = lambda q, side, index, log, machine: calls.append((q, side["name"], index))
ctx, rec = types.SimpleNamespace(machine="laptop"), types.SimpleNamespace(log=lambda r: None)
rp.trial_pair(ctx, {"key": "S13/char_model", "tr": {"name": "TR"}, "ref": {"name": "PyTorch"}, "agree": None}, rec)
rp.trial_pair(ctx, {"key": "S13/fib", "tr": {"name": "TR"}, "ref": {"name": "Go"}, "agree": lambda a, b: None}, rec)
print(json.dumps(calls))
`
	out, err := exec.Command(python(t), "-I", "-c", check).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var calls [][]string
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &calls); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(calls) != 2 || calls[0][1] != "TR" || calls[1][1] != "PyTorch" || calls[0][0] != "S13/char_model" {
		t.Errorf("trial runs %v, want the char model's two sides once and nothing for the item with an agreement", calls)
	}
}

// Ş3 (DENETİM 944): model.py ağa çıkmaz; dosyayı yalnız sources.json'daki yerinde arar (başka klasördeki aynı adlı dosyaya
// düşmez), boyunu ve kimliğini (LFS'te sha256, küçük dosyada git blob) denetler, bağladığı yolu yazar; tutmazsa çıkış 2.
func TestLaptopModelBinderUsesOnlyTheSourceFile(t *testing.T) {
	py := python(t)
	dir := t.TempDir()
	models, work := filepath.Join(dir, "models"), filepath.Join(dir, "work")
	write := func(rel, body string) {
		p := filepath.Join(models, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("m/weights.bin", "weights")
	write("m/config.json", `{"a": 1}`)
	write("other/vocab.json", "someone else's") // aynı adlı, başka modelin dosyası
	sum := sha256.Sum256([]byte("weights"))
	blob := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(`{"a": 1}`), `{"a": 1}`)))
	src := func(weights string, vocab bool) string {
		files := map[string]any{
			"weights.bin": map[string]any{"path": "m/weights.bin", "size": 7, "sha256": weights},
			"config.json": map[string]any{"path": "m/config.json", "size": 8, "git_blob": hex.EncodeToString(blob[:])},
		}
		if vocab {
			files["vocab.json"] = map[string]any{"path": "m/vocab.json", "size": 14, "git_blob": "0"}
		}
		b, _ := json.Marshal(map[string]any{"models": map[string]any{"m": map[string]any{"repo": "r", "revision": "abc", "files": files}}})
		p := filepath.Join(dir, "sources.json")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	run := func(sources string) (string, int) {
		out, err := exec.Command(py, "-I", filepath.Join("baseline", "machines", "laptop", "model.py"), sources, work, models, "m").CombinedOutput()
		code := 0
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return string(out), code
	}
	stale := filepath.Join(work, "models", "m", "old.bin") // N3 (DENETİM 947): önceki bağlamadan kalan dosya
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := run(src(hex.EncodeToString(sum[:]), false)); code != 0 || !strings.Contains(out, "<- modeller/m/weights.bin, sha256") ||
		!strings.Contains(out, "<- modeller/m/config.json, git_blob") {
		t.Errorf("good files: exit %d\n%s", code, out)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("a file left in work/models/m by an earlier binding is still there: %v", err)
	}
	// İş klasörü modellerin içindeyse boşaltma asılları silerdi: bağlayıcı durur, asıl yerinde kalır.
	inside, err := exec.Command(py, "-I", filepath.Join("baseline", "machines", "laptop", "model.py"), src(hex.EncodeToString(sum[:]), false),
		filepath.Dir(models), models, "m").CombinedOutput()
	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 2 || !strings.Contains(string(inside), "İŞ KLASÖRÜ MODELLERİN İÇİNDE") {
		t.Errorf("work folder holding the models: %v\n%s", err, inside)
	}
	if _, err := os.Stat(filepath.Join(models, "m", "weights.bin")); err != nil {
		t.Errorf("the original model file: %v", err)
	}
	if out, code := run(src(strings.Repeat("0", 64), false)); code != 2 || !strings.Contains(out, "SHA256 TUTMADI m/weights.bin") {
		t.Errorf("wrong sha256: exit %d, want 2\n%s", code, out)
	}
	if out, code := run(src(hex.EncodeToString(sum[:]), true)); code != 2 || !strings.Contains(out, "YOK m/vocab.json: m/vocab.json") {
		t.Errorf("file only in another model's folder: exit %d, want 2 without falling back\n%s", code, out)
	}
}

// laptopSources dizüstü betiklerinin tek kaynak dosyası (Ş3).
const laptopSources = "baseline/machines/laptop/sources.json"

// Ş3: dizüstü betiklerinin bağladığı her model (olcu_model), denetlediği her emsal (olcu_arac) ve paket (olcu_surum)
// machines/laptop/sources.json'da kimliğiyle yazılıdır: model dosyasının yeri, boyu, sha256'sı ya da git blob'u; ikilinin
// sürüm satırı ve sha256'sı; paketin sürümü.
func TestLaptopSourceFileCoversEveryScript(t *testing.T) {
	dir := filepath.Join("baseline", "machines", "laptop")
	b, err := os.ReadFile(laptopSources)
	if err != nil {
		t.Fatal(err)
	}
	var src struct {
		Models map[string]struct {
			Repo, Revision string
			Files          map[string]struct {
				Path, Sha256 string
				GitBlob      string `json:"git_blob"`
				Size         int64
			}
		}
		Tools map[string]struct {
			Version, Release, Asset string
			AssetSha256             string `json:"asset_sha256"`
			Files                   map[string]string
		}
		Python map[string]struct{ Version string }
	}
	if err := json.Unmarshal(b, &src); err != nil {
		t.Fatal(err)
	}
	scripts, _ := reporoot.Glob(filepath.Join(dir, "*.sh"))
	seen := 0
	for _, s := range scripts {
		body, _ := os.ReadFile(s)
		for _, line := range strings.Split(string(body), "\n") {
			f := strings.Fields(strings.NewReplacer("for m in", "olcu_model", "; do olcu_model \"$m\"; done", "").Replace(line))
			if len(f) < 2 {
				continue
			}
			switch f[0] {
			case "olcu_model":
				for _, m := range f[1:] {
					e, ok := src.Models[m]
					if !ok || e.Repo == "" || e.Revision == "" || len(e.Files) == 0 {
						t.Errorf("%s: model %s has no record in sources.json", filepath.Base(s), m)
					}
					for name, x := range e.Files {
						if x.Path == "" || x.Size <= 0 || (len(x.Sha256) != 64 && len(x.GitBlob) != 40) {
							t.Errorf("sources.json %s/%s: path %q, size %d, sha256 %q, git blob %q", m, name, x.Path, x.Size, x.Sha256, x.GitBlob)
						}
					}
					seen++
				}
			case "olcu_arac":
				if tool, ok := src.Tools[f[1]]; !ok || tool.Version == "" || len(tool.Files) == 0 {
					t.Errorf("%s: tool %s has no version or files in sources.json", filepath.Base(s), f[1])
				}
			case "olcu_surum":
				for _, p := range f[1:] {
					if src.Python[p].Version == "" {
						t.Errorf("%s: package %s has no version in sources.json", filepath.Base(s), p)
					}
				}
			}
		}
	}
	if seen < 30 {
		t.Errorf("%d model bindings read from %d scripts; the parser lost them", seen, len(scripts))
	}
	// DENETİM 947 Ş3: emsalin sürüm dosyası adıyla ve sha256'sıyla; hesabı yapan arka uç kitaplıkları (ggml-cpu-*.dll)
	// ikililerle birlikte sha256'larıyla denetlenir.
	for name, tool := range src.Tools {
		backend := 0
		for f, sum := range tool.Files {
			if len(sum) != 64 {
				t.Errorf("sources.json %s %s: sha256 %q", name, f, sum)
			}
			if strings.HasPrefix(f, "ggml-cpu") && strings.HasSuffix(f, ".dll") {
				backend++
			}
		}
		if backend == 0 || tool.Asset == "" || len(tool.AssetSha256) != 64 || tool.Release == "" {
			t.Errorf("sources.json %s: %d backend libraries, release %q, asset %q (%q); want both named with their sha256",
				name, backend, tool.Release, tool.Asset, tool.AssetSha256)
		}
	}
}

// N2 (DENETİM 947): satır taslağı, satırın zorunlu Ayar alanı için makinenin iş parçacığını ve koşunun aldığı en hızlı
// ayarı basar.
func TestLaptopRowsPrintTheSetting(t *testing.T) {
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "metrics.json"), []byte(`{"_machine": {"threads": 20, "threads_source": "gctrace P (O-1)"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "en-hizli.json"), []byte(`{"llama.cpp read": {"threads": 6}, "_scan": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, code := baseline(t, "--rows", "--out", out)
	if code != 0 || !strings.Contains(got, `setting: machine threads 20 (gctrace P (O-1)); fastest settings {"llama.cpp read": {"threads": 6}}`) {
		t.Errorf("--rows: exit %d\n%s", code, got)
	}
}

// Ş3: emsalin Python paketi sources.json'daki sürümde değilse ya da kurulu değilse betik durur.
func TestLaptopPackagesMatchTheSourceFile(t *testing.T) {
	py := python(t)
	p := filepath.Join(t.TempDir(), "sources.json")
	if err := os.WriteFile(p, []byte(`{"python": {"pip": {"version": "0.0.1"}, "no-such-package-tr": {"version": "1"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, "-I", filepath.Join("baseline", "machines", "laptop", "sources.py"), p, "python", "pip", "no-such-package-tr").CombinedOutput()
	e, ok := err.(*exec.ExitError)
	if !ok || e.ExitCode() != 2 || !strings.Contains(string(out), "TUTMADI pip: kurulu ") ||
		!strings.Contains(string(out), "TUTMADI no-such-package-tr: kurulu None") {
		t.Errorf("packages off the source file: %v\n%s", err, out)
	}
}

// libRun lib.sh'i okuyan bir bash komutu koşar; REPO, iş ve kayıt klasörü sınama için verilir.
func libRun(t *testing.T, repo string, env []string, script string) (string, int) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash yok")
	}
	lib, _ := filepath.Abs(filepath.Join("baseline", "machines", "laptop", "lib.sh"))
	dir := t.TempDir()
	cmd := exec.Command(bash, "-c", fmt.Sprintf(`. %q; REPO=%q; %s`, filepath.ToSlash(lib), filepath.ToSlash(repo), script))
	cmd.Env = append(os.Environ(), append([]string{"USERPROFILE=" + dir, "OLCU_IS=" + filepath.ToSlash(filepath.Join(dir, "is")),
		"OLCU_KAYIT=" + filepath.ToSlash(filepath.Join(dir, "kayit"))}, env...)...)
	out, err := cmd.CombinedOutput()
	if e, ok := err.(*exec.ExitError); ok {
		return string(out), e.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), 0
}

// Ş4 (DENETİM 944): değişik dosyalı ağaçta satırın Commit'i ölçülen kodu göstermez: ölçü kipinde çıkış 2, denemede uyarı.
func TestLaptopDirtyTreeStopsTheMeasure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "t"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "changed.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	prep := `mkdir -p "$IS" "$KAYIT"; : > "$IS/hazirlik.txt"; olcu_makine t`
	if out, code := libRun(t, repo, nil, prep); code != 2 || !strings.Contains(out, "1 değişik dosya: satırın Commit'i ölçülen kodu göstermez") {
		t.Errorf("measure on a dirty tree: exit %d, want 2\n%s", code, out)
	}
	if out, code := libRun(t, repo, []string{"OLCU_DENEME=1"}, prep); code != 0 || !strings.Contains(out, "UYARI: ağaçta 1 değişik dosya") {
		t.Errorf("trial on a dirty tree: exit %d, want 0 with a warning\n%s", code, out)
	}
}

// DENETİM 947 Ş1: ölçü kipinde tarama boş dosyayla başlar (eski en-hizli.json silinir) ve bütün grupları ister; eksik
// listeyle koşmaz.
func TestLaptopMeasureScanStartsEmpty(t *testing.T) {
	script := `mkdir -p "$KAYIT/run-en-hizli-ayar"; echo '{}' > "$AYAR"; echo '{}' > "$KAYIT/run-en-hizli-ayar/en-hizli.json"
( olcu_tara "llama.cpp read" ); echo "rc=$?"
[ -f "$AYAR" ] && echo "ayar kaldı" || echo "ayar silindi"
[ -f "$KAYIT/run-en-hizli-ayar/en-hizli.json" ] && echo "kopya kaldı" || echo "kopya silindi"`
	out, code := libRun(t, t.TempDir(), nil, script)
	if code != 0 || !strings.Contains(out, "rc=2") || !strings.Contains(out, "ayar silindi") || !strings.Contains(out, "kopya silindi") ||
		!strings.Contains(out, "ölçü kipinde tarama bütün grupları koşar") {
		t.Errorf("measure scan with one group: exit %d\n%s", code, out)
	}
}

// N1 (DENETİM 944): Ahmet'in açık bir Dost'u (asistan.tr'yi koşan süreç) işlemciyi ve kapıyı paylaşır; betik durur.
func TestLaptopOpenDostStopsTheScript(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("süreç listesi PowerShell'le, dizüstünde")
	}
	if out, code := libRun(t, t.TempDir(), nil, "olcu_dost_yok"); code != 0 {
		t.Fatalf("no Dost: exit %d\n%s", code, out)
	}
	dost := exec.Command(python(t), "-c", "import time; time.sleep(60)", "asistan.tr")
	if err := dost.Start(); err != nil {
		t.Fatal(err)
	}
	defer dost.Process.Kill()
	if out, code := libRun(t, t.TempDir(), nil, "olcu_dost_yok"); code != 2 || !strings.Contains(out, "Dost açık (") {
		t.Errorf("Dost open: exit %d, want 2\n%s", code, out)
	}
}

// deviceCheck aygıt denetimini dizüstünde ve GPU'da görülen çıktı örnekleriyle sınar.
const deviceCheck = `
import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
bench = lambda b, n: {"stdout": json.dumps([{"backends": b, "n_gpu_layers": n, "avg_ns": 1}]), "stderr": ""}
cpu_info = "system_info: n_threads = 14 (n_threads_batch = 14) / 20 | CPU : SSE3 = 1 | AVX2 = 1 | "
cases = {
    "bench cpu": (["llama-bench.exe", "-m", "g", "-ngl", "0"], bench("CPU", 0), {}),
    "bench cuda asked cpu": (["llama-bench", "-m", "g", "-ngl", "0"], bench("CUDA", 99), {}),
    "bench cuda": (["llama-bench", "-m", "g", "-ngl", "99"], bench("CUDA", 99), {}),
    "completion cpu": (["llama-completion.exe", "-ngl", "0"], {"stdout": "x", "stderr": cpu_info}, {}),
    "completion offloaded": (["llama-completion", "-ngl", "0"], {"stdout": "x", "stderr": cpu_info + "\nload_tensors: offloaded 37/37 layers to GPU"}, {}),
    "completion silent": (["llama-completion", "-ngl", "0"], {"stdout": "x", "stderr": ""}, {}),
    "whisper cpu": (["whisper-cli.exe", "-ng"], {"stdout": "", "stderr": "load_backend: loaded CPU backend from x\nwhisper_init: use gpu    = 0"}, {}),
    "whisper gpu asked cpu": (["whisper-cli", "-ng"], {"stdout": "", "stderr": "load_backend: loaded CUDA backend from x\nuse gpu    = 1"}, {}),
    "tr cpu": (["trlang", "x.tr", "--device", "cpu"], {}, {"device": "cpu", "actual_device": "cpu"}),
    "tr echo but gpu": (["trlang", "x.tr", "--device", "cpu"], {}, {"device": "cpu", "actual_device": "gpu:0"}),
    "ref cuda asked cpu": (["python", "references.py", "lm", "--device", "cpu"], {}, {"device": "cpu", "actual_device": "cuda"}),
    "ref silent": (["python", "references.py", "lm", "--device", "cpu"], {}, {"text": "a"}),
    "no device": (["compare", "fib"], {}, {}),
}
out = {}
for name, (cmd, run, parsed) in cases.items():
    try:
        rp.check_device("Q", "side", cmd, run, parsed, lambda r: None, 0)
        out[name] = "ok"
    except rp.Stop as e:
        out[name] = "stop: " + str(e)
print(json.dumps(out))
`

// Ş2 (DENETİM 944; TCB 2026-11-19 20:39): her koşunun gerçek aygıtı kendi çıktısından okunur (llama-bench'in json'u,
// llama-completion'ın ve whisper-cli'nin günlüğü, emsalin actual_device'ı, TR'nin device'ı); komutun istediği aygıt
// değilse ya da aygıt yazılmamışsa kalem "aygıt uyuşmadı" diye durur.
func TestBaselineDeviceCheck(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", deviceCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, ok := range []string{"bench cpu", "bench cuda", "completion cpu", "whisper cpu", "tr cpu", "no device"} {
		if got[ok] != "ok" {
			t.Errorf("%s: %q, want the run to pass", ok, got[ok])
		}
	}
	// "tr echo but gpu": TR'nin device'ı komutun yankısı, actual_device bir tensörünkü (DENETİM 947 Ş2).
	for _, stop := range []string{"bench cuda asked cpu", "completion offloaded", "whisper gpu asked cpu", "ref cuda asked cpu", "tr echo but gpu"} {
		if !strings.Contains(got[stop], "device mismatch (aygıt uyuşmadı): ran on cuda") {
			t.Errorf("%s: %q, want a device mismatch stop", stop, got[stop])
		}
	}
	for _, silent := range []string{"completion silent", "ref silent"} {
		if !strings.HasPrefix(got[silent], "stop: ") || !strings.Contains(got[silent], "ran on None") {
			t.Errorf("%s: %q, want a stop when the run names no device", silent, got[silent])
		}
	}
}

// Emsallerin çıktısı her konsolda UTF-8'dir: koşturucu UTF-8 okur, Windows'ta boru cp1254'tür (dizüstünde S5'in uyuşması
// Türkçe harfler bozulunca 58 sözcük ayrı çıktı, 2026-11-20). Sınama: çıktı ASCII'ye zorlanır; düzeltmesiz UnicodeEncodeError.
func TestBaselineReferencesPrintUTF8(t *testing.T) {
	check := `
import importlib.util, io, sys
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="ascii")
spec = importlib.util.spec_from_file_location("references", "baseline/references.py")
refs = importlib.util.module_from_spec(spec)
spec.loader.exec_module(refs)
refs.emit({"text": "Küçük bir kasabada"})
`
	out, err := exec.Command(python(t), "-I", "-c", check).CombinedOutput()
	if err != nil {
		t.Fatalf("references.py's emit with an ASCII console: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"text": "Küçük bir kasabada"`) {
		t.Errorf("emit printed %q, want the text in UTF-8", out)
	}
}

// quietCheck wait_for_quiet'i sahte yük ve sahte saatle, foreign_load'ı sahte sayaçlarla sınar.
const quietCheck = `
import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
out = {}
now = [0.0]
def seq(values):
    it = iter(values)
    def measure():
        now[0] += 10.0
        return next(it)
    return measure
logged = []
out["falls"] = rp.wait_for_quiet("Q", logged.append, measure=seq([90, 50, 15]), clock=lambda: now[0], top=lambda: "x 99 %")
now[0] = 0.0
try:
    rp.wait_for_quiet("Q", logged.append, measure=seq([90] * 20), clock=lambda: now[0], top=lambda: "MsMpEng.exe 80 %")
    out["stays"] = "no stop"
except rp.Stop as e:
    out["stays"] = str(e)
out["logged"] = logged
rp.time.sleep = lambda s: None
rp._CPU_UNIT = 10_000_000
for name, own in (("own", 0.5), ("foreign", 0.0)):
    times = [(0, 0), (5_000_000, 200_000_000)]  # the machine: 0.5 CPU s busy of 20 CPU s
    owns = [0.0, own]
    rp._cpu_times = lambda: times.pop(0)
    rp._own_cpu = lambda: owns.pop(0)
    out[name] = rp.foreign_load(1.0)
print(json.dumps(out))
`

// Yük denetimi (TCB 2026-11-20; tanı: Windows Defender her koşudan sonra tarar, koşturucu kendini sayıyordu): yük
// eşiğin altına inerse beklenip ölçülür, 60 sn'de inmezse soru en çok yük yapan süreçle durur; koşturucunun kendi
// işlemci süresi yabancı yük sayılmaz.
func TestBaselineWaitsForQuiet(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", quietCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got struct {
		Falls   float64
		Stays   string
		Logged  []map[string]any
		Own     float64
		Foreign float64
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got.Falls != 15 {
		t.Errorf("load falling 90, 50, 15: got %v, want 15 after waiting", got.Falls)
	}
	if !strings.Contains(got.Stays, "after 60 s") || !strings.Contains(got.Stays, "MsMpEng.exe") {
		t.Errorf("load staying 90: %q, want a stop after 60 s naming the busiest process", got.Stays)
	}
	if len(got.Logged) != 1 || got.Logged[0]["busiest"] != "MsMpEng.exe 80 %" {
		t.Errorf("log %v, want one line with the busiest process", got.Logged)
	}
	if got.Own != 0 || got.Foreign != 2.5 {
		t.Errorf("0.5 CPU s busy of 20: foreign load %v when the runner did it, %v when another process did; want 0 and 2.5",
			got.Own, got.Foreign)
	}
}
