package e2e

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// tools/gpu/trt_io.py TR'nin tensör dosyasını (.trt) Python'da yazar ve okur: hedef modelin
// PyTorch başlangıç ağırlıkları TR'ye bu yolla yüklenir. Python'un yazdığı dosyayı TR
// tensör_yükle ile okur (float32 ve bf16), TR'nin yazdığını Python geri okur; iki yönde de
// değerler ve türler aynıdır.
func TestTrtFilesBetweenPythonAndTR(t *testing.T) {
	python := pythonOrSkip(t)
	if !hasPythonModules(python, "numpy") {
		t.Skip("numpy yok")
	}
	dir := t.TempDir()
	tools := reporoot.Path("tools/gpu")
	fromPy := filepath.Join(dir, "py.trt")
	write := `import sys, numpy as np
sys.path.insert(0, sys.argv[1])
from trt_io import write_trt
write_trt(sys.argv[2], {"w": np.array([[1.5, -2.0], [0.25, 3.0]], dtype=np.float32), "b": np.array([1.0, 0.333], dtype=np.float32)}, {"b": "bf16"})
`
	if out, err := exec.Command(python, "-I", "-c", write, tools, fromPy).CombinedOutput(); err != nil {
		t.Fatalf("python yazamadı: %v\n%s", err, out)
	}
	writeTR(t, dir, "oku.tr", `TR_var d = TR_load_tensors("py.trt")
TR_print(d["w"].TR_tolist)
TR_print(d["b"].TR_tolist)
TR_save_tensors("tr.trt", {"x": TR_Tensor([[0.5, 1.0, -4.0]])})
`)
	s := tr(t, dir, "oku.tr")
	// bf16'da 0.333 en yakın değere (0.33203125) yuvarlanır.
	testkit.Wait(t, s, 0, []string{"[[1.5, -2.0], [0.25, 3.0]]", "[1.0, 0.33203125]"}, nil)

	read := `import sys
sys.path.insert(0, sys.argv[1])
from trt_io import read_trt
for name, (a, dt) in read_trt(sys.argv[2]).items():
    print(name, dt, a.tolist())
`
	out, err := exec.Command(python, "-I", "-c", read, tools, filepath.Join(dir, "tr.trt")).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "x float32 [[0.5, 1.0, -4.0]]") {
		t.Fatalf("python TR dosyasını okuyamadı: %v\n%s", err, out)
	}
}
