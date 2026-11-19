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

// Temel ölçü setinin S4, S12, S13 ve S14 programları (tools/bench/baseline/README.md); yardımcılar
// behavior_baseline_test.go'dadır (trlang, program, programFails).

// Katalog programı (S4) Dost'un dosyalarını göreli yolla aktarır ve denetimden geçer. Dil modelinin yolu gerçek
// model ister (B'nin uyuşma adımı); burada ses girdisi küçük VITS modeliyle kurulur ve kataloğun ölçmediği bağlam durur.
func TestBaselineCatalogChecks(t *testing.T) {
	if out, err := exec.Command(trlang(t), "check", "baseline/catalog_memory.tr").CombinedOutput(); err != nil {
		t.Fatalf("trlang check: %v\n%s", err, out)
	}
	catalog := reporoot.Path("tools/assistant/katalog.json")
	got := program(t, "baseline/catalog_memory.tr", "--catalog", catalog, "--entry", "mms-tts-tur", "--path", reporoot.Path("tests/data/vits_tiny"), "--context", "0")
	if got["kind"] != "voice" || got["context"] != 0.0 || got["filled"] != 0.0 {
		t.Errorf("mms-tts-tur: %v, want a voice at context 0", got)
	}
	if _, ok := got["load_ms"].(float64); !ok {
		t.Errorf("load_ms missing: %v", got)
	}
	out := programFails(t, "baseline/catalog_memory.tr", "--catalog", catalog, "--entry", "qwen3-4b-q4km", "--path", "missing.gguf", "--context", "99")
	if !strings.Contains(out, "is not one of qwen3-4b-q4km's measured contexts [45, 7981]") {
		t.Errorf("unmeasured context: %s", out)
	}
	if out := programFails(t, "baseline/catalog_memory.tr", "--catalog", catalog, "--entry", "none", "--path", "x", "--context", "0"); !strings.Contains(out, "--entry none is not in") {
		t.Errorf("unknown entry: %s", out)
	}
}

// Matris programı (S12) işlemcide 64² birlerle: iki çekirdekte ve iki tipte köşe öğesi kenardır (64), süreler
// --repeat kadardır.
func TestBaselineMatmul(t *testing.T) {
	for _, kernel := range []string{"tr", "vendor"} {
		for _, dtype := range []string{"float32", "bf16"} {
			got := program(t, "baseline/matmul.tr", "--size", "64", "--fill", "ones", "--kernel", kernel, "--dtype", dtype, "--repeat", "3")
			if times, _ := got["times_ms"].([]any); got["corner"] != 64.0 || len(times) != 3 || got["kernel"] != kernel || got["dtype"] != dtype {
				t.Errorf("%s %s: %v, want corner 64 and 3 times", kernel, dtype, got)
			}
		}
	}
	if out := programFails(t, "baseline/matmul.tr", "--dtype", "fp8"); !strings.Contains(out, "--dtype is float32 or bf16") {
		t.Errorf("--dtype fp8: %s", out)
	}
}

// 100 Hz döngüsü (S13) 100 tur döner ve tur, yükteki tur ve kaçan tur sayılarını yazar; kaçan turun değeri
// makinenin yüküne bağlı olduğu için denetlenmez.
func TestBaselineLoop100Hz(t *testing.T) {
	got := program(t, "baseline/loop_100hz.tr", "--turns", "100", "--load_s", "0.2", "--size", "64")
	if got["turns"] != 100.0 {
		t.Errorf("turns %v, want 100", got["turns"])
	}
	for _, field := range []string{"missed", "turns_under_load", "missed_under_load", "worst_late_ms", "products"} {
		if _, ok := got[field].(float64); !ok {
			t.Errorf("%s missing: %v", field, got)
		}
	}
	if got["products"] == 0.0 || got["turns_under_load"] == 0.0 {
		t.Errorf("the load did not run: %v", got)
	}
}

// İş parçacığı (O-1): makinenin tek sayısı TR sürecinin gctrace satırındaki "<n> P"'dir, os.cpu_count() değil;
// her emsalin komutu onu kendi seçeneğinin adıyla taşır, references.py her kitaplığa kendi adıyla verir.
func TestBaselineThreadRule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gctrace.txt")
	line := "gc 1 @0.010s 1%: 0.01+0.5+0.02 ms clock, 0.1+0.2/0.3/0+0.1 ms cpu, 4->4->1 MB, 4 MB goal, 0 MB stacks, 0 MB globals, 97 P\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Procs    int
		CPUCount int `json:"cpu_count"`
		Settings struct {
			Torch         int `json:"torch.set_num_threads"`
			FasterWhisper struct {
				CPUThreads int `json:"cpu_threads"`
				NumWorkers int `json:"num_workers"`
			} `json:"faster_whisper.WhisperModel"`
			Onnx struct {
				Intra int `json:"intra_op_num_threads"`
				Inter int `json:"inter_op_num_threads"`
			} `json:"onnxruntime.SessionOptions"`
		}
		Commands map[string][]string
	}
	selfCheck(t, &got, "threads", path)
	if got.Procs != 97 || got.CPUCount == 97 {
		t.Fatalf("procs %d (cpu_count %d), want the gctrace's 97", got.Procs, got.CPUCount)
	}
	s := got.Settings
	if s.Torch != 97 || s.FasterWhisper.CPUThreads != 97 || s.FasterWhisper.NumWorkers != 1 || s.Onnx.Intra != 97 || s.Onnx.Inter != 1 {
		t.Errorf("library settings %+v, want 97 by each name (inter-op and workers 1)", s)
	}
	flags := map[string]string{"llama-bench": "-t", "llama-completion": "-t", "whisper-cli": "-t", "lm": "--threads",
		"whisper_hf": "--threads", "faster_whisper": "--threads", "mms": "--threads", "piper": "--threads", "matmul": "--threads"}
	for name, flag := range flags {
		cmd := strings.Join(got.Commands[name], " ")
		if !strings.Contains(cmd, " "+flag+" 97") {
			t.Errorf("%s: %q, want %s 97", name, cmd, flag)
		}
	}
	if len(got.Commands) != len(flags) {
		t.Errorf("%d reference commands, want %d: %v", len(got.Commands), len(flags), got.Commands)
	}
	// references.py sayıyı yalnız thread_settings'ten alır: torch, faster-whisper ve onnxruntime'ın üç yolu.
	b, err := os.ReadFile("baseline/references.py")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if strings.Contains(src, "cpu_count") || strings.Count(src, "thread_settings(a.threads)") != 3 {
		t.Errorf("references.py: cpu_count %v, thread_settings(a.threads) %d times; want no cpu_count and the three library paths",
			strings.Contains(src, "cpu_count"), strings.Count(src, "thread_settings(a.threads)"))
	}
}

// Soru kaydı (questions.py) her makinede setin sorularını kurar: makinenin listesi, S1'in altı modeli, GPU'da
// llama.cpp'nin -ngl'si, S4 yalnız HIGHRAM ve dizüstünde, GPU'da S13 yalnız 100 Hz döngüsü, dizüstünde S10 yalnız sıcak;
// emsal komutları makinenin tek iş parçacığı sayısını taşır.
func TestBaselineQuestions(t *testing.T) {
	work := t.TempDir()
	tools := `{"trlang": "trlang", "llama_bench": "llama-bench", "llama_completion": "llama-completion", "whisper_cli": "whisper-cli", "go_compare": "compare"}`
	if err := os.WriteFile(filepath.Join(work, "tools.json"), []byte(tools), 0o644); err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]struct {
		TR, Ref       [][]string
		Agree, Custom bool
	}
	selfCheck(t, &got, "items", work)
	// S5'in uyuşma işlevleri sonra, uyuşma geçişinde koşar; her biri kendi kaleminin adıyla yazar (A-11'de hepsi "en"di).
	var labels map[string]string
	selfCheck(t, &labels, "agree_labels", work)
	if len(labels) != 8 {
		t.Errorf("S5 agreement labels %v, want 8 items", labels)
	}
	for key, label := range labels {
		if key != label {
			t.Errorf("S5 item %s logs its agreement as %s", key, label)
		}
	}
	lists := map[string]string{
		"highram": "S1 S2 S3 S4 S5 S6 S7 S8 S9 S10 S13 S14", "l4gpu": "S1 S2 S3 S5 S6 S7 S8 S9 S10 S11 S12 S13 S14",
		"l4cpu": "S1 S2 S3 S5 S6 S7 S8 S9 S10 S14", "g4": "S1 S2 S3 S5 S6 S7 S8 S9 S10 S11 S12 S13 S14",
		"laptop": "S1 S2 S3 S4 S5 S6 S7 S8 S9 S10 S13 S14"}
	for machine, list := range lists {
		items := got[machine]
		seen := map[string]bool{}
		for key := range items {
			seen[strings.Split(key, "/")[0]] = true
		}
		for _, q := range strings.Fields(list) {
			if !seen[q] {
				t.Errorf("%s: no %s item", machine, q)
			}
			delete(seen, q)
		}
		if len(seen) != 0 {
			t.Errorf("%s: questions outside its list: %v", machine, seen)
		}
		gpu := machine == "l4gpu" || machine == "g4"
		s1 := items["S1/qwen3-4b-q4km"]
		if ngl := strings.Contains(strings.Join(s1.Ref[0], " "), "-ngl 99"); ngl != gpu || !s1.Agree {
			t.Errorf("%s S1: -ngl %v, agree %v; want -ngl on GPUs only and an agreement", machine, ngl, s1.Agree)
		}
		for key, it := range items {
			if strings.Contains(key, "/ort-") { // S7'nin en hızlı emsal ayarları bilerek başka sayıyla (0, 6; TCB 00:31)
				continue
			}
			for _, cmd := range it.Ref {
				line := strings.Join(cmd, " ")
				if (strings.Contains(line, "references.py") || strings.HasPrefix(line, "llama-") || strings.HasPrefix(line, "whisper-cli")) &&
					!strings.Contains(line, " 97") {
					t.Errorf("%s %s: %q has no thread number", machine, key, line)
				}
			}
		}
		_, cold := items["S10/tr/cold"]
		if cold == (machine == "laptop") {
			t.Errorf("%s: S10 cold %v; the laptop measures warm only", machine, cold)
		}
		_, pure := items["S13/fib"]
		if _, loop := items["S13/loop_100hz"]; strings.Contains(list, "S13") && (!loop || pure == gpu) {
			t.Errorf("%s: S13 loop %v, fib %v; GPUs run only the loop", machine, loop, pure)
		}
	}
	n := 0
	for key := range got["highram"] {
		if strings.HasPrefix(key, "S1/") {
			n++
		}
	}
	if _, ok := got["highram"]["S4/qwen3-4b-q4km/7981"]; n != 6 || !ok {
		t.Errorf("highram: %d S1 models (want 6), S4 qwen3 7981 %v", n, ok)
	}
	// --fetch --models yalnız MODELS'in adlarını kabul eder; bilinmeyen ad hiçbir şey indirmeden durur (çıkış 3).
	if out, code := baseline(t, "--work", work, "--fetch", "--models", "qwen3-4b-q4km,nope"); code != 3 || !strings.Contains(out, "unknown ['nope']") {
		t.Errorf("--models nope: exit %d, %s", code, out)
	}
}

// Okuyucular: Dost'un OLCU satırı ve olay akışının çerçevesi (S8, S9), llama-bench'in JSON'u, eğitim günlüğünün
// ms/adım'ı, whisper.cpp'nin süre satırları ve sözcük hata oranı.
func TestBaselineAssistantParsers(t *testing.T) {
	sample := map[string]any{
		"olcu": "başka satır\nOLCU tur=1 soz_ms=900 yazi_ms=410 ilk_parca_ms=620 ilk_cumle_ms=980 cevap_ms=2100 ilk_ses_ms=1300 " +
			"toplam_ms=1350 calma_sonrasi=false onek=12\n",
		"sse":         "id: ab-1\ndata: {\"v\":2,\"type\":\"state\",\"data\":{\"state\":\"idle\",\"mic\":false}}\n\nid: ab-2\ndata: {\"v\":2,\"type\":\"metrics\",\"data\":{\"first_token_ms\":910,\"tokens_per_second\":11.2}}\n\n",
		"llama_bench": "load\n[{\"n_prompt\": 512, \"n_gen\": 0, \"n_depth\": 0, \"avg_ns\": 2500000000, \"avg_ts\": 204.8}]",
		"rgn":         "adım 10 / 30 | 412 ms/adım\nadım 30 / 30 | 398.5 ms/adım\n",
		"whisper_cpp": "whisper_print_timings:     load time =   120.50 ms\nwhisper_print_timings:    total time =  3120.50 ms\n",
		"wer":         []string{"Bir gün saat durdu", "bir gün, saat durmadı."},
	}
	b, _ := json.Marshal(sample)
	path := filepath.Join(t.TempDir(), "sample.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Olcu struct{ Turns []map[string]any }
		SSE  []struct {
			Type string
			Data map[string]any
		}
		LlamaBench map[string]float64 `json:"llama_bench"`
		MsPerStep  map[string]float64 `json:"ms_per_step"`
		WhisperCpp float64            `json:"whisper_cpp_ms"`
		Wer        float64
	}
	selfCheck(t, &got, "parsers", path)
	if len(got.Olcu.Turns) != 1 || got.Olcu.Turns[0]["ilk_ses_ms"] != 1300.0 || got.Olcu.Turns[0]["calma_sonrasi"] != false {
		t.Errorf("OLCU: %v", got.Olcu)
	}
	if len(got.SSE) != 2 || got.SSE[0].Data["state"] != "idle" || got.SSE[1].Data["first_token_ms"] != 910.0 {
		t.Errorf("SSE: %+v", got.SSE)
	}
	if got.LlamaBench["avg_ns"] != 2.5e9 || got.LlamaBench["n_prompt"] != 512 {
		t.Errorf("llama-bench: %v", got.LlamaBench)
	}
	if got.MsPerStep["ms_per_step"] != 398.5 || got.WhisperCpp != 3000 {
		t.Errorf("ms/adım %v, whisper.cpp %v; want the last 398.5 and total − load 3000", got.MsPerStep, got.WhisperCpp)
	}
	if !near(got.Wer, 0.25) {
		t.Errorf("WER %v, want 1 of 4 words (0.25)", got.Wer)
	}
}

// Boş program (S14) hatasız biter ve bir şey yazmaz: run.py yalnız açılıp kapanma süresini ölçer.
func TestBaselineEmpty(t *testing.T) {
	out, err := exec.Command(trlang(t), "baseline/empty.tr").CombinedOutput()
	if err != nil || len(out) != 0 {
		t.Errorf("empty.tr: %v, output %q; want no error and no output", err, out)
	}
}

// S5'in uyuşması (TCB'nin B15 kararı): TR'nin her float32 emsale sözcük uzaklığı, öbür float32 emsalin ona
// uzaklığını aşmaz. whisper.cpp sınır kurmaz: onun "Altyazı M.K." uydurması TR'ninkini geçirmez. Metinler A-11'in
// dördüncü sınamasındaki ayrılığın kısaltılmışıdır.
func TestBaselineS5Verdict(t *testing.T) {
	fw, hf := "Yuvvab boşalınca saatçi kapağı kapattı.", "Yuvva boşalınca, saatçi kapağı kapattı."
	texts := map[string]string{"faster-whisper-float32": fw, "transformers": hf, "whisper.cpp": hf + " Altyazı M.K."}
	source := "Yuva boşalınca saatçi kapağı kapattı."
	cases := []map[string]any{
		{"tr": fw, "texts": texts, "source": source},                                      // sınama 4: tutar
		{"tr": "Yuvvab boşaldı saatçi kapağı kapattı.", "texts": texts, "source": source}, // transformers'a 2 > 1
		{"tr": fw + " Altyazı M.K.", "texts": texts, "source": source},                    // TR'nin uydurması
		{"tr": fw, "texts": map[string]string{"faster-whisper-float32": fw}, "source": source},
	}
	path := filepath.Join(t.TempDir(), "s5.json")
	b, _ := json.Marshal(cases)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	var got []struct {
		TRDistance map[string]int `json:"tr_distance"`
		Among      map[string]int `json:"among_references"`
		Bound      map[string]int
		Wer        map[string]float64
		Reason     *string
	}
	selfCheck(t, &got, "s5", path)
	if len(got) != 4 {
		t.Fatalf("%d verdicts, want 4", len(got))
	}
	g := got[0]
	if g.Reason != nil || g.TRDistance["faster-whisper-float32"] != 0 || g.TRDistance["transformers"] != 1 ||
		g.TRDistance["whisper.cpp"] != 4 || g.Bound["transformers"] != 1 || g.Bound["faster-whisper-float32"] != 1 {
		t.Errorf("A-11's case: %+v, want it to hold with TR 0 and 1 from the float32 references, bound 1, whisper.cpp 4", g)
	}
	if g.Among["transformers / whisper.cpp"] != 3 || g.Wer["TR"] == 0 {
		t.Errorf("A-11's case record: %+v, want the references' own distances and the WERs", g)
	}
	for i, want := range map[int]string{1: "beyond the float32 references", 2: "beyond the float32 references", 3: "missing"} {
		if got[i].Reason == nil || !strings.Contains(*got[i].Reason, want) {
			t.Errorf("case %d: %+v, want a reason with %q", i, got[i], want)
		}
	}
}

// S1 ve S3'ün uyuşması (TCB'nin B14 kararı): 32 konumun her biri ya uyuşur ya başa baştır (TR'nin en iyi logiti
// llama.cpp'nin token'ınınkinden ε'dan az büyük); ε'suz (GPU), 32'den az token ve ε'u aşan ayrılık durdurur.
func TestBaselineForcedVerdict(t *testing.T) {
	rows := func(apartAt int, gap float64) []map[string]any {
		out := make([]map[string]any, 32)
		for i := range out {
			out[i] = map[string]any{"llama": 5, "tr": 5, "gap": 0.0}
		}
		if apartAt >= 0 {
			out[apartAt] = map[string]any{"llama": 5, "tr": 6, "gap": gap}
		}
		return out
	}
	cases := []map[string]any{
		{"positions": rows(-1, 0), "epsilon": 1.0},      // hepsi uyuşur
		{"positions": rows(2, 0.3), "epsilon": 1.0},     // konum 2 başa baş
		{"positions": rows(2, 1.5), "epsilon": 1.0},     // konum 2 ε'u aşar
		{"positions": rows(-1, 0)},                      // ε yok (GPU)
		{"positions": rows(-1, 0)[:31], "epsilon": 1.0}, // 31 token
	}
	path := filepath.Join(t.TempDir(), "forced.json")
	b, _ := json.Marshal(cases)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	var got []struct {
		Agree  int
		Ties   []map[string]float64
		Apart  []map[string]float64
		Reason *string
	}
	selfCheck(t, &got, "forced", path)
	if len(got) != 5 {
		t.Fatalf("%d verdicts, want 5", len(got))
	}
	if got[0].Reason != nil || got[0].Agree != 32 {
		t.Errorf("all agree: %+v", got[0])
	}
	if got[1].Reason != nil || got[1].Agree != 31 || len(got[1].Ties) != 1 || got[1].Ties[0]["i"] != 2 {
		t.Errorf("a tie at 2: %+v", got[1])
	}
	for i, want := range map[int]string{2: "apart beyond ε", 3: "no ε", 4: "31 tokens"} {
		if got[i].Reason == nil || !strings.Contains(*got[i].Reason, want) {
			t.Errorf("case %d: %+v, want a reason with %q", i, got[i], want)
		}
	}
	// GPU makinesi ε'u HIGHRAM'in agreement.json'undan alır (TCB 15:19): yalnız orada ölçülmüş olanı, kaynağıyla.
	agreement := filepath.Join(t.TempDir(), "agreement.json")
	b, _ = json.Marshal(map[string]any{
		"S1/a": map[string]any{"ok": true, "epsilon": 2.5, "epsilon_source": "measured here, quantized and unquantized path"},
		"S1/b": map[string]any{"ok": true, "epsilon": 2.5, "epsilon_source": "HIGHRAM other.json S1/b"},
	})
	if err := os.WriteFile(agreement, b, 0o644); err != nil {
		t.Fatal(err)
	}
	var eps map[string][]any
	selfCheck(t, &eps, "epsilon", agreement)
	if a := eps["S1/a"]; a[0] != 2.5 || !strings.Contains(fmt.Sprint(a[1]), "S1/a") {
		t.Errorf("measured ε: %v, want 2.5 with its source", a)
	}
	if eps["S1/b"][0] != nil || eps["S1/c"][0] != nil {
		t.Errorf("ε taken from elsewhere or missing: %v, want none", eps)
	}
}

// MMS emsalinin girdisi (references.py mms_ids): VITS'in boşlukları her karakterin önünde ve sonunda; dolgu belirteci
// harf olan parçalayıcıda da ("k", mms-tts-eng; A-11 B13: "week" 8 değil 9 kimlik). Emsal paragrafı mms.tr gibi böler;
// koşturucu bozuk baytta durmaz.
func TestBaselineMMSIds(t *testing.T) {
	var got struct {
		Tokenizer    map[string][]int
		MmsIds       map[string][]int `json:"mms_ids"`
		AddBlankKept bool             `json:"add_blank_kept"`
		Sentences    []string
		StrayByte    string `json:"stray_byte"`
	}
	selfCheck(t, &got, "mms_ids", "baseline/texts.json")
	if len(got.Tokenizer["week"]) != 8 {
		t.Errorf("the stand-in tokenizer %v, want transformers' 8 ids for week", got.Tokenizer)
	}
	want := map[string][]int{"week": {0, 9, 0, 7, 0, 7, 0, 0, 0}, "kew": {0, 0, 0, 7, 0, 9, 0}, "a wa": {0, 26, 0, 19, 0, 9, 0, 26, 0}}
	for text, ids := range want {
		if fmt.Sprint(got.MmsIds[text]) != fmt.Sprint(ids) {
			t.Errorf("mms_ids(%q) = %v, want %v", text, got.MmsIds[text], ids)
		}
	}
	if !got.AddBlankKept || len(got.Sentences) != 5 || !strings.HasSuffix(got.Sentences[4], "work.") {
		t.Errorf("add_blank restored %v; sentences %q, want the paragraph's five", got.AddBlankKept, got.Sentences)
	}
	if got.StrayByte != "a�b" {
		t.Errorf("stray byte read as %q, want a, U+FFFD, b", got.StrayByte)
	}
}
