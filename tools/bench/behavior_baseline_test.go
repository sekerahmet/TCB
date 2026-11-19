// Layer: tools (tools); not shipped to users.
package bench

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Temel ölçü setinin koşturucusu (baseline/run.py) ve programları: yöntem ölçüden önce run.py'de
// yazılıdır; bu testler yöntemin her kuralını küçük girdilerle sınar (tools/bench/baseline/README.md).

// baseline runs baseline/run.py with the arguments; the exit code is returned with the output.
func baseline(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(python(t), append([]string{"-I", filepath.Join("baseline", "run.py")}, args...)...)
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
	b, err := os.ReadFile(filepath.Join("baseline", "texts.json"))
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
}
