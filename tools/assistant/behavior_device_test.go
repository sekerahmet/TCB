package assistant_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Zincirin cihaz geçişleri (T1 tasarımının G7'si, dilim 4a): kesitleyiciye giden örnekler ve mikrofonun çerçevesi
// işlemcide, kesit yazıya çevrilmeden önce varsayılan cihazda. Kart benzeticide (TR_GPU_SIM) sınanır; varsayılan cihaz
// işlemciyken üçü de boş taşımadır. Program modelsizdir: zincir.tr'nin işlevlerini çağırır.
const deviceProgram = `# TR 4
TR_import "{zincir}"
TR_var ayar = TR_command_line({"cihaz": "cpu"})
TR_default_device(TR_String(ayar["cihaz"]))
TR_var örnekler = TR_zeros([1, 1024])
TR_print("KURULAN", örnekler.TR_device)
TR_print("G1", işlemcide(örnekler[0]).TR_device)
TR_var çerçeve = çerçeve_örnekleri(TR_Bytes([0, 128, 255, 127, 1, 0, 0, 0]))
TR_print("C1", çerçeve.TR_device, çerçeve.TR_dtype, çerçeve[0].TR_item(), çerçeve[2].TR_item(), çerçeve[3].TR_item())
TR_print("ÜST", çerçeve[1].TR_item() < 1.0, çerçeve[1].TR_item() > 0.9999)
TR_print("G2", kesit_cihazda(işlemcide(TR_zeros([1024]))).TR_device)
`

func TestChainDeviceTransitions(t *testing.T) {
	zincir := filepath.ToSlash(strings.TrimSuffix(reporoot.Path(chainModule), ".tr"))
	dir := t.TempDir()
	program := filepath.Join(dir, "cihaz.tr")
	if err := os.WriteFile(program, []byte(strings.ReplaceAll(deviceProgram, "{zincir}", zincir)), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, d := range []struct{ device, built, whisper string }{
		{"cpu", "cpu", "cpu"},
		{"gpu", "gpu:0", "gpu:0"},
	} {
		t.Run(d.device, func(t *testing.T) {
			cmd := exec.Command(trlangBinary(t), program, "--cihaz", d.device)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "TR_GPU_SIM=1")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("trlang: %v\n%s", err, out)
			}
			got := string(out)
			for _, want := range []string{
				"KURULAN " + d.built + "\n",
				"G1 cpu\n",
				"C1 cpu float32 -1.0 3.0517578e-05 0.0\n",
				"ÜST true true\n",
				"G2 " + d.whisper + "\n",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("--cihaz %s: çıktıda %q yok:\n%s", d.device, want, got)
				}
			}
		})
	}
}
