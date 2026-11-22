// Layer: tools (tools); not shipped to users.
package bench

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// Dizüstünün ölçü betiklerinde bulunan düzeltmelerin testleri (H1, 2026-11-20 ve DENETİM 944): yük denetiminin
// beklemesi ve koşturucunun kendi süreç ağacı, her koşunun gerçek aygıtı (Ş2), emsallerin her konsolda UTF-8 çıktısı (BC24).

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
    "tr cpu": (["trlang", "x.tr", "--device", "cpu"], {}, {"device": "cpu"}),
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
	for _, stop := range []string{"bench cuda asked cpu", "completion offloaded", "whisper gpu asked cpu", "ref cuda asked cpu"} {
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
