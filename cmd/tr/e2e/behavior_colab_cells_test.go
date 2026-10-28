package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Colab hücreleri GPU'suz sınanamaz, ama içlerindeki hatalı Python ya da kabuk sözdizimi
// Colab'a gitmeden yakalanabilir (bir hücrede tablo başlığı iki satıra bölünmüş, bir `cp`
// satırında satır devamı yerine düz "\n" yazılmıştı). Bu test tools/gpu/COLAB.md'deki
// her ```bash bloğunu `bash -n` ile, içindeki Python gömmelerini (heredoc ve `python -c`)
// `py_compile` ile denetler; tools/gpu/*.py betikleri de derlenir ve `--help` verir;
// phase3_tr.tr `tr kontrol`dan uyarısız geçer.

var (
	colabBashRe    = regexp.MustCompile("(?ms)^```bash[ \t]*\r?\n(.*?)^```")
	colabHeredocRe = regexp.MustCompile(`(?ms)^[^\n]*python[^\n]*<<'?(\w+)'?[^\n]*\n(.*?)^(\w+)[ \t]*$`)
	colabCmdPyRe   = regexp.MustCompile(`python -c "([^"]*)"`)
	// Tırnak içi olmayan, satır sonu olmayan düz "\n" (cp satırındaki hata).
	colabCommentRe = regexp.MustCompile(`(?m)^[ 	]*#.*$`)
	colabQuotedRe  = regexp.MustCompile(`"[^"\n]*"|'[^'\n]*'`)
)

func pythonOrSkip(t *testing.T) string {
	t.Helper()
	for _, ad := range []string{"python3", "python"} {
		// Windows'ta python3 Microsoft Store kısayolu olabilir: gerçekten çalıştığına bakılır.
		if p, err := exec.LookPath(ad); err == nil && exec.Command(p, "-c", "pass").Run() == nil {
			return p
		}
	}
	t.Skip("python yok")
	return ""
}

func pyCompile(t *testing.T, python, ad, kod string) {
	t.Helper()
	dosya := filepath.Join(t.TempDir(), ad+".py")
	if err := os.WriteFile(dosya, []byte(kod), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(python, "-m", "py_compile", dosya).CombinedOutput(); err != nil {
		t.Errorf("%s: Python derlenmedi:\n%s\n--- kod ---\n%s", ad, out, kod)
	}
}

func TestColabCellsPythonAndShellSyntax(t *testing.T) {
	python := pythonOrSkip(t)
	data, err := os.ReadFile(reporoot.Path("tools/gpu/COLAB.md"))
	if err != nil {
		t.Fatal(err)
	}
	bloklar := colabBashRe.FindAllStringSubmatch(strings.ReplaceAll(string(data), "\r\n", "\n"), -1)
	if len(bloklar) < 8 {
		t.Fatalf("COLAB.md'de yalnızca %d bash bloğu bulundu", len(bloklar))
	}
	gomme := 0
	for i, b := range bloklar {
		blok := b[1]
		ad := "blok" + string(rune('A'+i%26))
		// Hücre sihirleri (%%bash) kabuk söz diziminde yorumdur; bash -n da öyle okur.
		cmd := exec.Command("bash", "-n")
		cmd.Stdin = strings.NewReader(blok)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%d. bash bloğu bash -n'den geçmedi: %s\n%s", i+1, out, blok)
		}
		kabuk := blok
		for _, h := range colabHeredocRe.FindAllStringSubmatch(blok, -1) {
			gomme++
			pyCompile(t, python, ad+"_heredoc", h[2])
			kabuk = strings.Replace(kabuk, h[2], "", 1)
		}
		for _, c := range colabCmdPyRe.FindAllStringSubmatch(blok, -1) {
			gomme++
			pyCompile(t, python, ad+"_c", strings.ReplaceAll(c[1], `\"`, `"`))
		}
		duz := colabQuotedRe.ReplaceAllString(colabCommentRe.ReplaceAllString(kabuk, ""), "")
		if strings.Contains(duz, `\n`) {
			t.Errorf("%d. bash bloğunda tırnak dışı düz \\n var (satır devamı için \\ + gerçek satır sonu gerekir):\n%s", i+1, blok)
		}
	}
	if gomme < 3 {
		t.Errorf("yalnızca %d Python gömmesi bulundu; ayıklama bozulmuş olabilir", gomme)
	}
}

func TestGPUScriptsCompileAndHelp(t *testing.T) {
	python := pythonOrSkip(t)
	betikler, err := reporoot.Glob(reporoot.Path("tools/gpu/*.py"))
	if err != nil || len(betikler) == 0 {
		t.Fatalf("tools/gpu/*.py bulunamadı: %v", err)
	}
	for _, b := range betikler {
		ad := filepath.Base(b)
		if out, err := exec.Command(python, "-m", "py_compile", b).CombinedOutput(); err != nil {
			t.Errorf("%s derlenmedi: %s", ad, out)
		}
		_ = os.RemoveAll(filepath.Join(filepath.Dir(b), "__pycache__"))
		// --help yalnızca argparse kuran betiklerde var; torch gerektirenler argparse'tan önce
		// torch içe aktarmaz (phase3_torch.py torch'u kos() içinde yükler).
		if strings.HasPrefix(ad, "phase3_") {
			out, err := exec.Command(python, "-I", b, "--help").CombinedOutput()
			if err != nil || !strings.Contains(string(out), "usage") {
				t.Errorf("%s --help çalışmadı: %v\n%s", ad, err, out)
			}
		}
	}
}

func TestPhase3TRProgramChecksClean(t *testing.T) {
	out, err := exec.Command(trBinary, "kontrol", reporoot.Path("tools/gpu/phase3_tr.tr")).CombinedOutput()
	if err != nil {
		t.Fatalf("phase3_tr.tr kontrolden geçmedi: %s", out)
	}
	if strings.Contains(string(out), "Uyarılar") {
		t.Errorf("phase3_tr.tr uyarı veriyor:\n%s", out)
	}
}

// Kural (GPU araçları göçün kör noktasıydı: denetlenip koşulmuyordu): Evre 3 aracı küçük bir ayarla
// CPU'da uçtan uca koşar; ayar dosyasının değerleri tip çağrısıyla açılır (3.7a) ve sonuç JSON'u
// kayıp eğrisini taşır.
func TestPhase3TRProgramRunsOnCPU(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "k.json")
	small := `{"B": 2, "T": 8, "D": 16, "H": 2, "FF": 32, "LAYERS": 1, "V": 50, "precision": "float32", "warmup": 1, "measured": 2, "loss_steps": 2, "lr": 0.01}`
	if err := os.WriteFile(config, []byte(small), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(trBinary, reporoot.Path("tools/gpu/phase3_tr.tr"), "--device", "cpu", "--config", config).CombinedOutput()
	if err != nil || !regexp.MustCompile(`"loss_curve":\[[0-9.]+,[0-9.]+\]`).Match(out) {
		t.Fatalf("phase3_tr.tr CPU'da koşmadı: %v\n%s", err, out)
	}
}
