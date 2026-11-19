// Layer: tools (tools); not shipped to users.
package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Temel ölçü setinin koşturucusu (baseline/run.py) ve programları: yöntem ölçüden önce run.py'de
// yazılıdır; bu testler yöntemin her kuralını küçük girdilerle sınar (tools/bench/baseline/README.md).

var (
	trlangOnce sync.Once
	trlangDir  string
	trlangPath string
	trlangErr  error
)

// TestMain trlang'ı bir kez geçici klasöre derleyen testlerden sonra o klasörü siler.
func TestMain(m *testing.M) {
	code := m.Run()
	if trlangDir != "" {
		os.RemoveAll(trlangDir)
	}
	os.Exit(code)
}

// trlang builds cmd/trlang once into a temporary folder (cmd/trlang/e2e/helpers_test.go's way) and returns it.
func trlang(t *testing.T) string {
	t.Helper()
	trlangOnce.Do(func() {
		if trlangDir, trlangErr = os.MkdirTemp("", "baseline-trlang"); trlangErr != nil {
			return
		}
		name := "trlang"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		trlangPath = filepath.Join(trlangDir, name)
		out, err := exec.Command("go", "build", "-o", trlangPath, "github.com/sekerahmet/TCB/cmd/trlang").CombinedOutput()
		if err != nil {
			trlangErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if trlangErr != nil {
		t.Fatal(trlangErr)
	}
	return trlangPath
}

// program runs a baseline .tr program (its path from this folder, written whole so codegraph links the
// file to the test) and decodes its BASELINE line.
func program(t *testing.T, name string, args ...string) map[string]any {
	t.Helper()
	cmd := exec.Command(trlang(t), append([]string{name}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	var v map[string]any
	if !strings.HasPrefix(last, "BASELINE ") || json.Unmarshal([]byte(last[len("BASELINE "):]), &v) != nil {
		t.Fatalf("%s: no BASELINE line:\n%s", name, out)
	}
	return v
}

// programFails runs a baseline .tr program that must stop with an error and returns its output.
func programFails(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(trlang(t), append([]string{name}, args...)...).CombinedOutput()
	if err == nil {
		t.Fatalf("%s %v: no error:\n%s", name, args, out)
	}
	return string(out)
}

// copyDir copies the files of a test data folder (and extra files) into a new temporary folder.
func copyDir(t *testing.T, src string, extra map[string]string) string {
	t.Helper()
	dst := t.TempDir()
	entries, err := reporoot.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, e := range entries {
		files[e.Name()] = filepath.Join(src, e.Name())
	}
	for name, from := range extra {
		files[name] = from
	}
	for name, from := range files {
		b, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// baseline runs baseline/run.py with the arguments; the exit code is returned with the output.
func baseline(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(python(t), append([]string{"-I", "baseline/run.py"}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(out), ee.ExitCode()
		}
		t.Fatalf("run.py %v: %v", args, err)
	}
	return string(out), 0
}

// selfCheck runs a self check of run.py and decodes its JSON answer into v.
func selfCheck(t *testing.T, v any, args ...string) int {
	t.Helper()
	out, code := baseline(t, append([]string{"--self-check"}, args...)...)
	if err := json.Unmarshal([]byte(out), v); err != nil {
		t.Fatalf("--self-check %v: %v\n%s", args, err, out)
	}
	return code
}

// Her taraf bir ısınma koşusu yapar, sonra ABBA düzeni: TR, E, E, TR, TR, E, E, TR, TR, E (setin yöntemi).
func TestBaselineOrderIsABBA(t *testing.T) {
	var got struct{ Warmup, Runs []string }
	selfCheck(t, &got, "order")
	if !reflect.DeepEqual(got.Warmup, []string{"TR", "E"}) {
		t.Errorf("warm-up %v, want one per side", got.Warmup)
	}
	want := strings.Fields("TR E E TR TR E E TR TR E")
	if !reflect.DeepEqual(got.Runs, want) {
		t.Errorf("runs %v, want %v", got.Runs, want)
	}
}

// Hüküm çeyreklerden verilir (exclusive): ayrışan, çakışan ve 1'e %1'den yakın üç örnek.
func TestBaselineVerdict(t *testing.T) {
	in := `{"apart":[0.8,0.85,0.9,0.82,0.88],"overlap":[0.9,1.1,1.0,0.95,1.05],"border":[1.005,1.008,1.02,1.03,1.04]}`
	var got map[string]struct {
		Q1, Median, Q3 float64
		Verdict        string
		Borderline     bool
	}
	selfCheck(t, &got, "verdict", in)
	for name, want := range map[string]struct {
		verdict    string
		borderline bool
		q1, q3     float64
	}{
		"apart":   {"tr_better", false, 0.81, 0.89},
		"overlap": {"not_separated", false, 0.925, 1.075},
		"border":  {"tr_worse", true, 1.0065, 1.035},
	} {
		g := got[name]
		if g.Verdict != want.verdict || g.Borderline != want.borderline || !near(g.Q1, want.q1) || !near(g.Q3, want.q3) {
			t.Errorf("%s: got %+v, want %+v", name, g, want)
		}
	}
}

func near(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

// Bellek okuyucuları: /proc/<pid>/status'un VmHWM ve RssAnon'u, nvidia-smi satırları, gctrace'in
// canlı öbeği ve P sayısı (iş parçacığı sayısının kaynağı).
func TestBaselineMemoryParsers(t *testing.T) {
	sample := map[string]string{
		"proc": "Name:\ttrlang\nVmPeak:\t 9000 kB\nVmHWM:\t  812340 kB\nVmRSS:\t  700000 kB\nRssAnon:\t  650112 kB\nRssFile:\t 50000 kB\n",
		"gpu":  "1234\n5678\n",
		"gctrace": "gc 1 @0.012s 1%: 0.01+0.5+0.02 ms clock, 0.1+0.2/0.3/0+0.1 ms cpu, 4->4->1 MB, 4 MB goal, 0 MB stacks, 0 MB globals, 7 P\n" +
			"some program output\n" +
			"gc 2 @0.300s 2%: 0.01+1.5+0.02 ms clock, 0.1+0.2/0.3/0+0.1 ms cpu, 90->95->61 MB, 96 MB goal, 0 MB stacks, 0 MB globals, 7 P\n",
	}
	b, _ := json.Marshal(sample)
	path := filepath.Join(t.TempDir(), "sample.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Proc    map[string]int
		GPU     []int
		Gctrace struct {
			Procs      int `json:"procs"`
			LiveHeapMB int `json:"live_heap_mb"`
		}
	}
	selfCheck(t, &got, "memory", path)
	if got.Proc["VmHWM"] != 812340 || got.Proc["RssAnon"] != 650112 {
		t.Errorf("proc status %v", got.Proc)
	}
	if !reflect.DeepEqual(got.GPU, []int{1234, 5678}) {
		t.Errorf("nvidia-smi %v", got.GPU)
	}
	if got.Gctrace.Procs != 7 || got.Gctrace.LiveHeapMB != 61 {
		t.Errorf("gctrace %+v, want 7 P and 61 MB live", got.Gctrace)
	}
}

// Uyuşmayan iki taraf ölçü koşularından önce durdurur ve çıkış kodu 3'tür.
func TestBaselineAgreementStops(t *testing.T) {
	var got struct {
		Stopped bool
		Reason  string
	}
	code := selfCheck(t, &got, "agreement")
	if code != 3 || !got.Stopped || !strings.Contains(got.Reason, "disagree") {
		t.Errorf("exit %d, %+v; want exit 3 and a disagreement", code, got)
	}
}

// Sabit metinler: S5'in iki metni, S6–S7'nin dil başına üç cümlesi, S8'in dil başına on sorusu.
func TestBaselineTextsExist(t *testing.T) {
	b, err := os.ReadFile("baseline/texts.json")
	if err != nil {
		t.Fatal(err)
	}
	var texts struct {
		SpeechToText  map[string]string   `json:"speech_to_text"`
		TextToSpeech  map[string][]string `json:"text_to_speech"`
		VoiceTurn     map[string][]string `json:"voice_turn"`
		LanguageModel map[string]string   `json:"language_model"`
	}
	if err := json.Unmarshal(b, &texts); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"tr", "en"} {
		if n := len(strings.Fields(texts.SpeechToText[lang])); n < 60 {
			t.Errorf("speech_to_text.%s: %d words, want ~30 s of speech (60 or more)", lang, n)
		}
		if n := len(texts.TextToSpeech[lang]); n != 3 {
			t.Errorf("text_to_speech.%s: %d sentences, want 3", lang, n)
		}
		if n := len(texts.VoiceTurn[lang]); n != 10 {
			t.Errorf("voice_turn.%s: %d questions, want 10", lang, n)
		}
	}
	if texts.LanguageModel["tr"] == "" {
		t.Error("language_model.tr is empty")
	}
	// İngilizce Piper'ın kimlik dosyası (A-4) texts.json'un İngilizce cümlelerinin kimlikleridir: metin değişince
	// dosya sessizce eskimez. Dosya A-11'in Colab sınamasında `prepare.sh phonemes` ile üretildi (piper-phonemize 1.1.0).
	// Her kimlik dizisi Piper'ın biçimindedir: `^` (1) ile başlar, `$` (2) ile biter.
	b, err = os.ReadFile("baseline/english_phoneme_ids.json")
	if err != nil {
		t.Fatal(err)
	}
	var ids struct {
		PiperPhonemize string `json:"piper_phonemize"`
		Sentences      []struct {
			Text string `json:"text"`
			IDs  []int  `json:"ids"`
		} `json:"sentences"`
	}
	if err := json.Unmarshal(b, &ids); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range ids.Sentences {
		got = append(got, s.Text)
		if len(s.IDs) < 3 || s.IDs[0] != 1 || s.IDs[len(s.IDs)-1] != 2 {
			t.Errorf("ids of %q: %v, want ^ (1) first and $ (2) last", s.Text, s.IDs)
		}
	}
	if ids.PiperPhonemize != "1.1.0" {
		t.Errorf("piper_phonemize %q, want 1.1.0 (A-4)", ids.PiperPhonemize)
	}
	if !reflect.DeepEqual(got, texts.TextToSpeech["en"]) {
		t.Errorf("english_phoneme_ids.json sentences %q, want texts.json's text_to_speech.en %q", got, texts.TextToSpeech["en"])
	}
}

// Dil modeli programı üç kipte koşar; parçalı klasör yolu (her *.safetensors birleşir) tek dosyalı
// küçük modelle sınanır, üretilen token sayısı istenen kadardır ve uyuşma kipi metni de yazar.
func TestBaselineLanguageModel(t *testing.T) {
	data := reporoot.Path("tests/data")
	folder := copyDir(t, filepath.Join(data, "hf_tiny"), map[string]string{"tokenizer.json": filepath.Join(data, "tokenizer.json")})
	common := []string{"--folder", folder, "--text", "Bir gün saat durdu.", "--prompt", "4", "--gen", "27"}
	speed := program(t, "baseline/language_model.tr", append(common, "--mode", "speed")...)
	for _, field := range []string{"load_ms", "prompt_ms", "gen_ms"} {
		if _, ok := speed[field].(float64); !ok {
			t.Errorf("speed: %s missing: %v", field, speed)
		}
	}
	if toks, _ := speed["tokens"].([]any); len(toks) != 27 || speed["gen_depth"] != 4.0 {
		t.Errorf("speed: %d tokens from depth %v, want 27 from 4", len(toks), speed["gen_depth"])
	}
	depth := program(t, "baseline/language_model.tr", append(common, "--mode", "depth", "--depths", "8,16", "--depth_prompt", "4")...)
	if rows, _ := depth["depths"].([]any); len(rows) != 2 {
		t.Errorf("depth: %v, want two depths", depth["depths"])
	}
	agree := program(t, "baseline/language_model.tr", append(common, "--mode", "agree", "--agree_tokens", "5")...)
	if toks, _ := agree["tokens"].([]any); len(toks) != 5 {
		t.Errorf("agree: %v, want 5 tokens", agree["tokens"])
	}
	if _, ok := agree["text"].(string); !ok {
		t.Errorf("agree: no text: %v", agree)
	}
}

// whisperModel tests/whisper_model_test.tr'nin çeviri modelini (ağırlıklar formülden, parçalayıcı
// `parçalayıcı_metni`) ve kısa sesini (`ses_kur(8000)`) aynı formüllerle diske yazar. Formüller oradan aynen
// alınmıştır; özgün `transcribe`'ın bu model ve sesteki kaydı tests/testdata/whisper/kucuk_cevriyazi.json'dadır.
const whisperModel = `# TR 4
TR_import "tr/audio"
TR_import "tr/pretrained"

TR_const _BOYUT = 8
TR_const _ARA = 32
TR_const _KAYNAK = 1500
TR_const _HEDEF = 24
TR_const _METİN = "Ġabcdefghijklmnopqrstuvwxyz.,?!'"
TR_const _ÖZEL = ["<|endoftext|>", "<|startoftranscript|>", "<|en|>", "<|tr|>", "<|translate|>", "<|transcribe|>", "<|startoflm|>", "<|startofprev|>", "<|nospeech|>", "<|notimestamps|>"]
TR_const _EOT = 32
TR_const _DAMGA_SAYISI = 128
TR_const _DAĞARCIK = 170

TR_func değerler(tohum: TR_Int, n: TR_Int, ölçek: TR_Float) -> TR_List[TR_Float] {
    TR_return [(((i + 1) * 40503 + tohum * 9973) % 65536 - 32768) / 32768.0 * ölçek TR_for i TR_in 0..n]
}

TR_func ölçek(ad: TR_String) -> TR_Float {
    TR_if ad.TR_ends_with("q_proj.weight") TR_or ad.TR_ends_with("k_proj.weight") { TR_return 1.0 }
    TR_if ad.TR_ends_with("conv1.weight") TR_or ad.TR_ends_with("conv2.weight") TR_or ad.TR_ends_with("fc1.weight") TR_or ad.TR_ends_with("embed_tokens.weight") { TR_return 2.0 }
    TR_return 0.25
}

TR_struct Ad {
    ad: TR_String
    şekil: TR_List[TR_Int]
}

TR_func dikkat(ön: TR_String) -> TR_List[Ad] {
    TR_var b = [_BOYUT, _BOYUT]
    TR_return [Ad(ad: "\(ön).q_proj.weight", şekil: b), Ad(ad: "\(ön).q_proj.bias", şekil: [_BOYUT]), Ad(ad: "\(ön).k_proj.weight", şekil: b),
        Ad(ad: "\(ön).v_proj.weight", şekil: b), Ad(ad: "\(ön).v_proj.bias", şekil: [_BOYUT]),
        Ad(ad: "\(ön).out_proj.weight", şekil: b), Ad(ad: "\(ön).out_proj.bias", şekil: [_BOYUT])]
}

TR_func norm(ön: TR_String) -> TR_List[Ad] { TR_return [Ad(ad: "\(ön).weight", şekil: [_BOYUT]), Ad(ad: "\(ön).bias", şekil: [_BOYUT])] }

TR_func blok(ön: TR_String, çözücü: TR_Bool) -> TR_List[Ad] {
    TR_var r = dikkat("\(ön).self_attn") + norm("\(ön).self_attn_layer_norm")
    TR_if çözücü { r = r + dikkat("\(ön).encoder_attn") + norm("\(ön).encoder_attn_layer_norm") }
    r = r + [Ad(ad: "\(ön).fc1.weight", şekil: [_ARA, _BOYUT]), Ad(ad: "\(ön).fc1.bias", şekil: [_ARA]),
        Ad(ad: "\(ön).fc2.weight", şekil: [_BOYUT, _ARA]), Ad(ad: "\(ön).fc2.bias", şekil: [_BOYUT])]
    TR_return r + norm("\(ön).final_layer_norm")
}

TR_func adlar() -> TR_List[Ad] {
    TR_var e = "model.encoder"
    TR_var d = "model.decoder"
    TR_var r = [Ad(ad: "\(e).conv1.weight", şekil: [_BOYUT, 80, 3]), Ad(ad: "\(e).conv1.bias", şekil: [_BOYUT]),
        Ad(ad: "\(e).conv2.weight", şekil: [_BOYUT, _BOYUT, 3]), Ad(ad: "\(e).conv2.bias", şekil: [_BOYUT]),
        Ad(ad: "\(e).embed_positions.weight", şekil: [_KAYNAK, _BOYUT])]
    r = r + blok("\(e).layers.0", TR_false) + blok("\(e).layers.1", TR_false) + norm("\(e).layer_norm")
    r = r + [Ad(ad: "\(d).embed_tokens.weight", şekil: [_DAĞARCIK, _BOYUT]), Ad(ad: "\(d).embed_positions.weight", şekil: [_HEDEF, _BOYUT])]
    TR_return r + blok("\(d).layers.0", TR_true) + blok("\(d).layers.1", TR_true) + norm("\(d).layer_norm")
}

TR_func ağırlıklar() -> TR_Dict[TR_String, TR_Tensor] {
    TR_var sonuç: TR_Dict[TR_String, TR_Tensor] = {}
    TR_for tohum, a TR_in adlar() {
        TR_var n = 1
        TR_for boy TR_in a.şekil { n = n * boy }
        sonuç[a.ad] = TR_Tensor(değerler(tohum, n, ölçek(a.ad))).TR_reshape(a.şekil)
    }
    TR_return sonuç
}

TR_func json_metni(a: TR_Dict[TR_String, TR_String]) -> TR_String {
    TR_var alanlar: TR_List[TR_String] = []
    TR_for ad, değer TR_in a { alanlar.TR_append("\"\(ad)\": \(değer)") }
    TR_return "{" + alanlar.TR_join(", ") + "}"
}

TR_func ayar() -> TR_Dict[TR_String, TR_String] {
    TR_return {"model_type": "\"whisper\"", "d_model": "\(_BOYUT)", "num_mel_bins": "80", "vocab_size": "\(_DAĞARCIK)", "max_source_positions": "\(_KAYNAK)",
        "max_target_positions": "\(_HEDEF)", "encoder_layers": "2", "encoder_attention_heads": "2", "encoder_ffn_dim": "\(_ARA)", "decoder_layers": "2",
        "decoder_attention_heads": "4", "decoder_ffn_dim": "\(_ARA)", "activation_function": "\"gelu\"", "scale_embedding": "false"}
}

TR_func üretim() -> TR_Dict[TR_String, TR_String] {
    TR_return {"suppress_tokens": "[5, 9, 33, 36, 37, 38, 39, 40]", "no_timestamps_token_id": "41", "lang_to_id": "{\"<|en|>\": 34, \"<|tr|>\": 35}"}
}

TR_func damga_adı(i: TR_Int) -> TR_String {
    TR_var yüzde = i * 2 % 100
    TR_var sıfır = TR_if yüzde < 10 { "0" } TR_else { "" }
    TR_return "<|\(i * 2 // 100).\(sıfır)\(yüzde)|>"
}

TR_func parçalayıcı_metni() -> TR_String {
    TR_var sözlük: TR_List[TR_String] = []
    TR_var eklenen: TR_List[TR_String] = []
    TR_for i, c TR_in _METİN { sözlük.TR_append("\"\(c)\": \(i)") }
    TR_var adlar_listesi = _ÖZEL + [damga_adı(j) TR_for j TR_in 0.._DAMGA_SAYISI]
    TR_for i, ad TR_in adlar_listesi {
        TR_var kimlik = _EOT + i
        TR_var özel = TR_if i < _ÖZEL.TR_length { "true" } TR_else { "false" }
        sözlük.TR_append("\"\(ad)\": \(kimlik)")
        eklenen.TR_append("{\"id\": \(kimlik), \"content\": \"\(ad)\", \"single_word\": false, \"lstrip\": false, \"rstrip\": false, \"normalized\": false, \"special\": \(özel)}")
    }
    TR_var ön = "{\"type\": \"ByteLevel\", \"add_prefix_space\": false, \"trim_offsets\": true, \"use_regex\": false}"
    TR_var çözücü = "{\"type\": \"ByteLevel\", \"add_prefix_space\": true, \"trim_offsets\": true, \"use_regex\": true}"
    TR_var model = "{\"type\": \"BPE\", \"dropout\": null, \"unk_token\": null, \"continuing_subword_prefix\": null, \"end_of_word_suffix\": null, \"fuse_unk\": false, \"byte_fallback\": false, \"ignore_merges\": false, \"vocab\": {\(sözlük.TR_join(", "))}, \"merges\": []}"
    TR_return "{\"version\": \"1.0\", \"truncation\": null, \"padding\": null, \"added_tokens\": [\(eklenen.TR_join(", "))], \"normalizer\": null, \"pre_tokenizer\": \(ön), \"post_processor\": null, \"decoder\": \(çözücü), \"model\": \(model)}"
}

TR_func ses_kur(n: TR_Int) -> TR_Sound {
    TR_var örnekler = [((i * i + 7 * i) % 7919 / 7919.0 - 0.5) * 0.25 * ((i // 8000) % 2) TR_for i TR_in 0..n]
    TR_return TR_Sound(TR_samples: TR_Tensor([örnekler]), TR_sample_rate: 16000)
}

TR_var opt = TR_command_line({"out": "", "wav": ""})
TR_var k = TR_String(opt["out"])
TR_write_file(TR_Path(k, "config.json"), json_metni(ayar()))
TR_write_file(TR_Path(k, "generation_config.json"), json_metni(üretim()))
TR_write_file(TR_Path(k, "tokenizer.json"), parçalayıcı_metni())
TR_save_tensors(TR_String(TR_Path(k, "model.safetensors")), ağırlıklar())
TR_write_wav(TR_String(opt["wav"]), ses_kur(8000))
TR_print("BASELINE {}")
`

// Whisper programı (S5) tests/whisper_model_test.tr'nin küçük çeviri modelini açar ve kısa sesten özgün
// `transcribe`'ın metnini verir (kucuk_cevriyazi.json "kisa"); süreler ve sesin süresi JSON'dadır.
func TestBaselineWhisper(t *testing.T) {
	dir := t.TempDir()
	gen := filepath.Join(dir, "whisper_model.tr")
	if err := os.WriteFile(gen, []byte(whisperModel), 0o644); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(dir, "model")
	if err := os.Mkdir(model, 0o755); err != nil {
		t.Fatal(err)
	}
	wav := filepath.Join(dir, "short.wav")
	program(t, gen, "--out", model, "--wav", wav)
	b, err := os.ReadFile(reporoot.Path("tests/testdata/whisper/kucuk_cevriyazi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Short struct {
			Text     string
			Segments []any
		} `json:"kisa"`
	}
	if err := json.Unmarshal(b, &ref); err != nil {
		t.Fatal(err)
	}
	got := program(t, "baseline/whisper.tr", "--model", model, "--audio", wav, "--language", "en")
	if got["text"] != ref.Short.Text || got["segments"] != float64(len(ref.Short.Segments)) {
		t.Errorf("text %q in %v segments, want the original's %q in %d", got["text"], got["segments"], ref.Short.Text, len(ref.Short.Segments))
	}
	if got["audio_s"] != 0.5 {
		t.Errorf("audio_s %v, want 0.5", got["audio_s"])
	}
	for _, field := range []string{"load_ms", "transcribe_ms", "audio_per_compute"} {
		if _, ok := got[field].(float64); !ok {
			t.Errorf("%s missing: %v", field, got)
		}
	}
	if out := programFails(t, "baseline/whisper.tr", "--model", model, "--audio", wav, "--language", "de"); !strings.Contains(out, "--language is tr or en") {
		t.Errorf("--language de: %s", out)
	}
}

// voiceRun checks the shared fields of mms.tr and piper.tr: one row per sentence, the first sentence's time,
// the samples' sum and the sample rate.
func voiceRun(t *testing.T, name string, got map[string]any, sentences int, rate float64) []map[string]any {
	t.Helper()
	raw, _ := got["sentences"].([]any)
	if len(raw) != sentences || got["sample_rate"] != rate {
		t.Fatalf("%s: %d sentences at %v Hz, want %d at %v: %v", name, len(raw), got["sample_rate"], sentences, rate, got)
	}
	rows := make([]map[string]any, len(raw))
	sum := 0.0
	for i, r := range raw {
		rows[i] = r.(map[string]any)
		sum += rows[i]["samples"].(float64)
	}
	if got["samples"] != sum || got["first_ms"] != rows[0]["ms"] || got["noise"] != "zero" {
		t.Errorf("%s: samples %v (sum %v), first_ms %v (row 0 %v), noise %v", name, got["samples"], sum, got["first_ms"], rows[0]["ms"], got["noise"])
	}
	return rows
}

// MMS (S6) ve Piper (S7) programları: cümleler texts.json'dan (liste ya da paragraf), gürültü 0; MMS'in WAV'ı
// cümleler arasında sessizlikle yazılır; Piper kimlik dosyasıyla konuşur ve cümleleri texts.json'unkiler olmayan dosyayla durur.
func TestBaselineVoices(t *testing.T) {
	dir := t.TempDir()
	wav := filepath.Join(dir, "mms.wav")
	mms := program(t, "baseline/mms.tr", "--model", reporoot.Path("tests/data/vits_tiny"), "--texts", "baseline/texts.json", "--wav", wav, "--gap_s", "0.5")
	voiceRun(t, "mms", mms, 3, 16000)
	if st, err := os.Stat(wav); err != nil || st.Size() != 44+2*(int64(mms["samples"].(float64))+2*8000) {
		t.Errorf("mms wav: %v, want 44 header bytes and 16-bit samples with two 0.5 s gaps", err)
	}
	// speech_to_text tek paragraftır; program onu ". "de cümlelerine böler (O-7'nin S5 sesi; A-11'in ikinci
	// sınaması liste beklenen yerde metinle durmuştu).
	stt := program(t, "baseline/mms.tr", "--model", reporoot.Path("tests/data/vits_tiny"), "--texts", "baseline/texts.json", "--set", "speech_to_text", "--language", "en")
	voiceRun(t, "mms speech_to_text", stt, 5, 16000)
	b, err := os.ReadFile("baseline/texts.json")
	if err != nil {
		t.Fatal(err)
	}
	var texts struct {
		TextToSpeech map[string][]string `json:"text_to_speech"`
	}
	if err := json.Unmarshal(b, &texts); err != nil {
		t.Fatal(err)
	}
	type row struct {
		Text     string `json:"text"`
		Phonemes string `json:"phonemes"`
		IDs      []int  `json:"ids"`
	}
	var file struct {
		Sentences []row `json:"sentences"`
	}
	for _, s := range texts.TextToSpeech["en"] {
		file.Sentences = append(file.Sentences, row{s, "test", []int{1, 0, 12, 0, 21, 0, 18, 0, 2}})
	}
	ids := filepath.Join(dir, "ids.json")
	b, _ = json.Marshal(file)
	if err := os.WriteFile(ids, b, 0o644); err != nil {
		t.Fatal(err)
	}
	common := []string{"--model", reporoot.Path("tests/data/vits_piper_tiny"), "--texts", "baseline/texts.json", "--ids", ids}
	piper := program(t, "baseline/piper.tr", append(common, "--language", "en")...)
	rows := voiceRun(t, "piper", piper, 3, 22050)
	for _, r := range rows {
		// Gürültü 0'da aynı kimlikler aynı sayıda örnek verir (O-4'ün uyuşması örnek sayısına da bakar).
		if r["ids"] != 9.0 || r["samples"] != rows[0]["samples"] {
			t.Errorf("piper row %v, want 9 ids and %v samples", r, rows[0]["samples"])
		}
	}
	if out := programFails(t, "baseline/piper.tr", append(common, "--language", "tr")...); !strings.Contains(out, "make the file again") {
		t.Errorf("ids for other sentences: %s", out)
	}
}
