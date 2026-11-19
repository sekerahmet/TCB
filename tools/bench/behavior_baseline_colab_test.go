// Layer: tools (tools); not shipped to users.
package bench

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Temel ölçü setinin B–D'deki (Colab ölçüleri) düzeltmelerinin testleri: --quantized auto'nun ağırlık kuralı (BC4),
// Dost'un seslerinin sondaki sessizliği ve tur kaydı (BC6), S10'un soğuk kipi, faster-whisper'ın CUDA 12 kitaplıkları
// (BC3). Yardımcılar behavior_baseline_test.go'dadır (trlang, program, programFails, copyDir) ve run_test.go'da (python).

// q8GGUF 32 boyutlu, tek katmanlı küçük bir llama GGUF'u yazar: çarpım ağırlıkları ve gömme Q8_0 (blok başına fp16 ölçek
// 2⁻⁷ ve 32 işaretli kod), normlar F32; parçalayıcı 40 tek harfli token (A'dan başlayarak, gpt2).
func q8GGUF(t *testing.T) string {
	t.Helper()
	const dim, vocab = 32, 40
	b := &gguftest.Builder{}
	b.KV("general.architecture", "llama").KV("llama.block_count", uint32(1)).KV("llama.embedding_length", uint32(dim)).
		KV("llama.feed_forward_length", uint32(dim)).KV("llama.attention.head_count", uint32(1)).
		KV("llama.attention.head_count_kv", uint32(1)).KV("llama.context_length", uint32(64)).
		KV("llama.rope.freq_base", float32(10000)).KV("llama.attention.layer_norm_rms_epsilon", float32(1e-5))
	tokens, kinds := make([]string, vocab), make([]int32, vocab)
	for i := range tokens {
		tokens[i], kinds[i] = string(rune('A'+i)), 1
	}
	b.KV("tokenizer.ggml.model", "gpt2").KV("tokenizer.ggml.pre", "llama-bpe").KV("tokenizer.ggml.tokens", tokens).
		KV("tokenizer.ggml.token_type", kinds).KV("tokenizer.ggml.merges", []string{}).KV("tokenizer.ggml.bos_token_id", uint32(0))
	seed := uint32(7)
	q8 := func(rows int) []byte { // satır başına bir blok: fp16 ölçek (0x2000 = 2⁻⁷), 32 kod (-20..20)
		out := make([]byte, 0, rows*34)
		for range rows {
			out = binary.LittleEndian.AppendUint16(out, 0x2000)
			for range dim {
				seed = seed*1664525 + 1013904223
				out = append(out, byte(int8(int(seed>>24)%41-20)))
			}
		}
		return out
	}
	ones := make([]byte, 0, 4*dim)
	for range dim {
		ones = binary.LittleEndian.AppendUint32(ones, math.Float32bits(1))
	}
	b.Tensor("token_embd.weight", gguf.TypeQ8_0, []uint64{dim, vocab}, q8(vocab))
	for _, n := range []string{"output_norm.weight", "blk.0.attn_norm.weight", "blk.0.ffn_norm.weight"} {
		b.Tensor(n, gguf.TypeF32, []uint64{dim}, ones)
	}
	for _, n := range []string{"attn_q", "attn_k", "attn_v", "attn_output", "ffn_gate", "ffn_up", "ffn_down"} {
		b.Tensor("blk.0."+n+".weight", gguf.TypeQ8_0, []uint64{dim, dim}, q8(dim))
	}
	file := filepath.Join(t.TempDir(), "q8.gguf")
	if err := os.WriteFile(file, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// dostSoundsCheck questions.py'yi sahte koşturucuyla yükler: make_sounds'un ffmpeg komutlarını ve logged_turns'ün kaydını basar.
const dostSoundsCheck = `
import importlib.util, json, os, sys, tempfile, types
sys.stdout.reconfigure(encoding="utf-8")
spec = importlib.util.spec_from_file_location("questions", "baseline/questions.py")
qs = importlib.util.module_from_spec(spec)
spec.loader.exec_module(qs)
d, cmds, log = tempfile.mkdtemp(), [], []
def run_once(cmd):
    if cmd[0] == "ffmpeg" and "-version" in cmd:
        return {"rc": 0, "stdout": "ffmpeg version test", "stderr": ""}
    out = cmd[-1] if cmd[0] == "ffmpeg" else cmd[cmd.index("--wav") + 1]
    open(out, "wb").write(b"x")
    if cmd[0] == "ffmpeg":
        cmds.append(cmd)
    return {"rc": 0, "stdout": "", "stderr": ""}
rp = types.SimpleNamespace(run_once=run_once, Stop=RuntimeError)
ctx = types.SimpleNamespace(rp=rp, sound=lambda n: os.path.join(d, n), model=lambda m: m, folder=lambda m: m,
                            tr=lambda p, *a: ["trlang", p, *map(str, a)], log=log.append)
qs.make_sounds(ctx)
pads = {os.path.basename(c[-1]): [x for x in c if x.startswith("apad=")] for c in cmds}
r = qs.logged_turns(ctx, "S8/tr", "SÖZ: Merhaba nasılsın?\nOLCU tur=1 ilk_ses_ms=5937\nSÖZ: Bugün günlerden ne?\nOLCU tur=2 ilk_ses_ms=6751\n")
print(json.dumps({"pads": pads, "turns": len(r["turns"]), "log": log[-1]}, ensure_ascii=False))
`

// Dost'un sesleri (S8) sorular arasındakiyle aynı sessizlikle biter: sahte mikrofonun dosyası bitince Dost son soruyu
// ancak ardındaki sessizlikle kapatır (B'de G4: 10 sorudan 9 tur, BC6). S5'in sesleri değişmez (sha256'ları A-11'inki).
// Her S8 koşusunun turları ve duyulan sorular kayda yazılır: eksik tur kayıttan okunur.
func TestBaselineDostSounds(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", dostSoundsCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got struct {
		Pads  map[string][]string
		Turns int
		Log   struct {
			Question  string
			OlcuTurns int `json:"olcu_turns"`
			Heard     []string
		}
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for name, want := range map[string]int{"turn_tr.wav": 1, "turn_en.wav": 1, "stt_tr.wav": 0, "stt_en.wav": 0} {
		if p := got.Pads[name]; len(p) != want || (want == 1 && p[0] != "apad=pad_dur=48.0") {
			t.Errorf("%s: ffmpeg pad %v, want %d (48 s, the gap between the questions)", name, p, want)
		}
	}
	if got.Turns != 2 || got.Log.Question != "S8/tr" || got.Log.OlcuTurns != 2 ||
		!reflect.DeepEqual(got.Log.Heard, []string{"Merhaba nasılsın?", "Bugün günlerden ne?"}) {
		t.Errorf("logged turns: %+v", got)
	}
}

// fwLibsCheck BASELINE_FW_LIBS verilince S5'in komutlarını basar: kitaplık yolu yalnız faster-whisper'ın komutunda.
const fwLibsCheck = `
import importlib.util, json, os, sys, tempfile
os.environ["BASELINE_FW_LIBS"] = "/cu12/cublas/lib:/cu12/cudnn/lib"
os.environ["LD_LIBRARY_PATH"] = "/usr/lib64-nvidia"
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
qs = rp.load_questions()
work = tempfile.mkdtemp()
json.dump({"trlang": "trlang", "whisper_cli": "whisper-cli", "llama_bench": "b", "llama_completion": "c", "go_compare": "g"},
          open(os.path.join(work, "tools.json"), "w"))
ctx = qs.Ctx(rp, "g4", 4, work, lambda r: None, dry=True)
out = {}
for it in qs.items(ctx, "S5"):
    out[it["key"]] = {"ref": it["ref"]["cmds"][0][:2], "tr": it["tr"]["cmds"][0][:1]}
lc = qs.items(ctx, "S1")[0]["ref_agree"][0]
out["S1 -n"] = {"ref": [str(lc[lc.index("-n") + 1])], "tr": []}
import types
pos = [{"llama": 5, "tr": 5, "gap": 0.0}] * 32
said = 'BASELINE ' + json.dumps({"positions": pos, "epsilon": 0.0, "epsilon_note": "no quantized path for these weights (15.53a)"})
gpu = types.SimpleNamespace(tmp=work, gpu=True, epsilon_from=None, log=lambda r: None, extra={},
                            rp=types.SimpleNamespace(run_once=lambda c: {"rc": 0, "stdout": said, "stderr": ""}, parse_result=rp.parse_result))
reason = qs.agree_forced(gpu, "S1/qwen3-4b-q8", ["trlang"])([{"text": "x"}], [{"stdout": "x"}])
out["S1 q8 gpu"] = {"ref": [str(reason), str(gpu.extra["S1/qwen3-4b-q8"]["epsilon"]), gpu.extra["S1/qwen3-4b-q8"]["epsilon_source"]], "tr": []}
print(json.dumps(out))
`

// faster-whisper'ın CUDA 12 kitaplıkları yalnız onun sürecine gider (BC3: aynı süreçte torch'un cuDNN'iyle sürümler
// karıştı): BASELINE_FW_LIBS verilince faster-whisper'ın iki komutu env LD_LIBRARY_PATH=… ile başlar, öbür emsallerin ve
// TR'nin komutu başlamaz.
func TestBaselineFasterWhisperLibs(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", fwLibsCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got map[string]struct{ Ref, TR []string }
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	// S1'in uyuşmasında llama-completion 33 token üretir, karşılaştırılan 32'dir: sona bastığı boş satır son token'la
	// birleşebilir (G4: "**" + "\n\n", BC9).
	if n := got["S1 -n"].Ref; len(n) != 1 || n[0] != "33" {
		t.Errorf("S1's llama-completion -n %v, want 33", n)
	}
	// GPU'da nicemli yolu olmayan model (Q8_0): ε kuralla 0, HIGHRAM'in dosyası gerekmez (TCB 17:57).
	if q := got["S1 q8 gpu"].Ref; len(q) != 3 || q[0] != "None" || q[1] != "0.0" || !strings.HasPrefix(q[2], "by rule:") {
		t.Errorf("S1 q8 on a GPU machine: %v, want agreement with ε 0 by rule", q)
	}
	want := []string{"env", "LD_LIBRARY_PATH=/cu12/cublas/lib:/cu12/cudnn/lib:/usr/lib64-nvidia"} // sürücünün yolu kalır
	for _, lang := range []string{"tr", "en"} {
		for name, libs := range map[string]bool{"faster-whisper-float32": true, "faster-whisper-int8": true, "transformers": false, "whisper.cpp": false} {
			c, ok := got["S5/"+lang+"/"+name]
			if !ok || reflect.DeepEqual(c.Ref, want) != libs || (len(c.TR) > 0 && c.TR[0] == "env") {
				t.Errorf("S5/%s/%s: %+v, faster-whisper's libraries %v", lang, name, c, libs)
			}
		}
	}
}

// coldCheck questions.py'nin S10 soğuk kip yardımcılarını basar: dosya listesi, atmanın hükmü; Linux'ta mincore'un
// baytları gerçek bir dosyada (okununca önbellekte, fadvise'tan sonra düşmüş).
const coldCheck = `
import importlib.util, json, os, sys, tempfile
sys.stdout.reconfigure(encoding="utf-8")
spec = importlib.util.spec_from_file_location("questions", "baseline/questions.py")
qs = importlib.util.module_from_spec(spec)
spec.loader.exec_module(qs)
d = tempfile.mkdtemp()
model = os.path.join(d, "m.gguf"); open(model, "wb").write(os.urandom(4 << 20))
folder = os.path.join(d, "whisper"); os.makedirs(folder); open(os.path.join(folder, "model.safetensors"), "wb").write(b"x")
paths = qs.cold_files([model, "asistan.tr", "--dil_modeli", model, "--whisper", folder, "--bağlam", "8192"])
out = {"model": model in paths, "folder": os.path.join(folder, "model.safetensors") in paths,
       "lib": any(p.endswith(os.path.join("lib", "vits.tr")) for p in paths),
       "assistant": any(p.endswith(os.path.join("assistant", "asistan.tr")) for p in paths),
       "words": any(p in ("--dil_modeli", "8192") for p in paths),
       "dropped": [qs.cold_dropped(b, a) for b, a in ((100000, 9000), (100000, 20000), (0, 0))], "linux": sys.platform == "linux"}
if out["linux"]:
    open(model, "rb").read()
    before = qs.cached_bytes([model])
    fd = os.open(model, os.O_RDONLY); os.fsync(fd); os.posix_fadvise(fd, 0, 0, os.POSIX_FADV_DONTNEED); os.close(fd)
    out["before"], out["after"] = before, qs.cached_bytes([model])
print(json.dumps(out))
`

// S10'un soğuk kipi (Colab'da /proc/sys salt okunur; TCB 17:45): ekranın açtığı dosyalar önbellekten atılır. Liste trlang'ı,
// komutun adını verdiği dosyaları ve klasörlerin dosyalarını, lib'in ve tools/assistant'ın TR dosyalarını içerir, öbür
// sözcükleri içermez; onda birin altı "atıldı" sayılır. Linux'ta mincore'un baytları: okunan 4 MB'lık dosya önbellekte,
// fadvise'tan sonra onda birin altında (Colab'da fincore yok; G4'te S10 onun yüzünden düştü).
func TestBaselineColdStart(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", coldCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got struct {
		Model, Folder, Lib, Assistant, Words, Linux bool
		Dropped                                     []bool
		Before, After                               int
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !got.Model || !got.Folder || !got.Lib || !got.Assistant || got.Words {
		t.Errorf("cold files: %+v", got)
	}
	if !reflect.DeepEqual(got.Dropped, []bool{true, false, true}) {
		t.Errorf("dropped %v, want [true false true]", got.Dropped)
	}
	if got.Linux && (got.Before < 4<<20/2 || got.After > got.Before/10) {
		t.Errorf("mincore: %d bytes cached after reading 4 MB, %d after fadvise; want most, then under a tenth", got.Before, got.After)
	}
}

// --quantized auto Dost'un kuralıyla seçer (tools/assistant/nicemli.tr, nicemli_çöz): işlemcide yalnız nicemlenmiş
// çarpım ağırlıklarının hepsi q4_k ya da q6_k olan modelde açık. Q8_0 ağırlıklı modelde auto nicemsiz yolu seçer ve
// koşar; --quantized on aynı modelde 15.53a'nın hatasını verir (B'de S1/qwen3-4b-q8'in düştüğü yer). Float modelde
// (hf_tiny) auto kapalıdır.
func TestBaselineQuantizedAuto(t *testing.T) {
	model := q8GGUF(t)
	common := []string{"--model", model, "--text", "ABCDABCD", "--mode", "agree", "--agree_tokens", "2"}
	auto := program(t, "baseline/language_model.tr", common...)
	if auto["quantized"] != false {
		t.Errorf("q8 model, --quantized auto: quantized %v, want false (%v)", auto["quantized"], auto)
	}
	if out := programFails(t, "baseline/language_model.tr", append(common, "--quantized", "on")...); !strings.Contains(out, "15.53a") {
		t.Errorf("q8 model, --quantized on: want 15.53a's error, got %s", out)
	}
	data := reporoot.Path("tests/data")
	folder := copyDir(t, filepath.Join(data, "hf_tiny"), map[string]string{"tokenizer.json": filepath.Join(data, "tokenizer.json")})
	float := program(t, "baseline/language_model.tr", "--folder", folder, "--text", "Bir gün", "--mode", "agree", "--agree_tokens", "2")
	if float["quantized"] != false {
		t.Errorf("float model, --quantized auto: quantized %v, want false", float["quantized"])
	}
	// Zorlanmış okuma Q8_0'da: ε'un öbür yolu nicemli bloktur, bu ağırlıkları kaldırmaz; ε 0 ve nedeni yazılır (B'de
	// S1/qwen3-4b-q8'in ε adımı 15.53a'yla düşüyordu). Başa boşluk koyan parçalayıcının çıktısı (" " + istem) da okunur;
	// cevabın token'ları sınır parçası (BOS) olmadan sayılır: "ABAB" dört token.
	for _, out := range []string{"ABCDABCDABAB", " ABCDABCDABAB"} {
		given := filepath.Join(t.TempDir(), "given.txt")
		if err := os.WriteFile(given, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		forced := program(t, "baseline/language_model.tr", "--model", model, "--text", "ABCDABCD", "--mode", "forced",
			"--agree_tokens", "8", "--given", given)
		if forced["epsilon"] != 0.0 || forced["epsilon_note"] == nil || forced["given_tokens"] != 4.0 {
			t.Errorf("q8 forced, given %q: epsilon %v (%v), %v given tokens; want 0 with its note and 4 tokens", out,
				forced["epsilon"], forced["epsilon_note"], forced["given_tokens"])
		}
	}
}

// nglCheck dört makinede S1 ve S3'ün llama.cpp komutlarındaki -ngl değerlerini basar (ölçü ve uyuşma komutları).
const nglCheck = `
import importlib.util, json, os, sys, tempfile
spec = importlib.util.spec_from_file_location("run", "baseline/run.py")
rp = importlib.util.module_from_spec(spec)
sys.modules["run"] = rp
spec.loader.exec_module(rp)
qs = rp.load_questions()
work = tempfile.mkdtemp()
json.dump({"trlang": "trlang", "whisper_cli": "whisper-cli", "llama_bench": "llama-bench", "llama_completion": "llama-completion",
           "go_compare": "g"}, open(os.path.join(work, "tools.json"), "w"))
out = {}
for machine in ("highram", "l4cpu", "l4gpu", "g4"):
    ctx = qs.Ctx(rp, machine, 4, work, lambda r: None, dry=True)
    got = []
    for q in ("S1", "S3"):
        for it in qs.items(ctx, q):
            for c in it["ref"]["cmds"] + it.get("ref_agree", []):
                if c[0] in ("llama-bench", "llama-completion"):
                    got.append(str(c[c.index("-ngl") + 1]) if "-ngl" in c else "none")
    out[machine] = got
print(json.dumps(out))
`

// llama.cpp'nin öntanımlısı bütün katmanları GPU'ya koyar (-ngl -1; b12124 llama.h); L4'ün llama.cpp'si CUDA'lıdır ve
// l4cpu aynı makinededir: işlemci sorusunda emsal GPU'da koşuyordu (BC20, Ahmet'in sorusu). S1 ve S3'ün her llama-bench ve
// llama-completion komutu işlemci makinesinde -ngl 0, GPU makinesinde -ngl 99 taşır.
func TestBaselineLlamaGPULayers(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "-c", nglCheck).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var got map[string][]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for machine, want := range map[string]string{"highram": "0", "l4cpu": "0", "l4gpu": "99", "g4": "99"} {
		if len(got[machine]) == 0 {
			t.Errorf("%s: no llama.cpp command in S1 and S3", machine)
		}
		for _, v := range got[machine] {
			if v != want {
				t.Errorf("%s: llama.cpp -ngl %s, want %s (%v)", machine, v, want, got[machine])
				break
			}
		}
	}
}

// Karakter modelinin PyTorch tarafı çıktısını her konsolda UTF-8 basar: Windows'ta borunun kodlaması cp1254'tür, koşturucu
// `python -I` ile çağırır ve yalıtılmış kip PYTHONIOENCODING'i yok sayar; dizüstünde S13/char_model düşüyordu (BC1).
// Sınama: çıktı ASCII'ye zorlanır (PYTHONIOENCODING, -I'sız); düzeltme olmadan üretilen metin UnicodeEncodeError verir.
func TestCharModelPrintsUTF8(t *testing.T) {
	py := python(t)
	if err := exec.Command(py, "-c", "import torch").Run(); err != nil {
		t.Skip("torch yok")
	}
	cmd := exec.Command(py, filepath.Join("tools", "bench", "py", "char_model.py"), "0", "1")
	cmd.Dir = reporoot.Path(".")
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=ascii")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("char_model.py with an ASCII console: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "model şunu yazdı:") {
		t.Errorf("char_model.py printed no sample:\n%s", out)
	}
}
