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
	for _, c := range []struct{ prog, n, want string }{{"dongu", "10", "45"}, {"fib", "10", "55"}, {"matris_dongu", "10", "443"},
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
