package e2e

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Hedef modelin TR karşılığı (tools/gpu/rgn512.tr) PyTorch sürümüyle (rgn512.py) sayısal olarak
// aynıdır: aynı başlangıç ağırlıkları (.trt ile aktarılır) ve aynı toplu işlerle, float32'de ve
// CPU'da her adımın kaybı ve gradyan normu float32 yuvarlama düzeyinde eşittir. Mimari (yerel/genel
// katmanlar, GQA, QK-norm, birleşik KV ve [G|U], PLE) ve eğitim (Muon, iki AdamW grubu, satır
// seyrek RMS, WSD, kırpma, birikim) birlikte sınanır. Küçük boyutlarla birkaç saniye sürer;
// PyTorch yoksa atlanır.
func TestTargetModelMatchesPyTorch(t *testing.T) {
	python := pythonOrSkip(t)
	if exec.Command(python, "-c", "import torch, numpy").Run() != nil {
		t.Skip("torch ya da numpy yok")
	}
	dir := t.TempDir()
	tools := reporoot.Path("tools/gpu")
	size := []string{"--micro", "2", "--accum", "2", "--seq", "64", "--steps", "5"}
	ref := append([]string{"-I", filepath.Join(tools, "rgn_reference.py"), "--out", dir,
		"--vocab", "256", "--d", "64", "--heads", "4", "--kv-heads", "2", "--ff", "96"}, size...)
	if out, err := exec.Command(python, ref...).CombinedOutput(); err != nil {
		t.Fatalf("referans koşusu: %v\n%s", err, out)
	}
	s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(tools, "rgn512.tr"),
		"--veri", "data", "--ağırlık", "init.trt", "--sıralı", "--hassasiyet", "fp32", "--cihaz", "cpu",
		"--adım", "5", "--toplu", "4", "--mikro", "2", "--bağlam", "64", "--günlük", "5", "--kayıplar", "tr.json",
		"--dağarcık", "256", "--boyut", "64", "--bakış_sayısı", "4", "--kv_bakış_sayısı", "2", "--ara", "96")
	if s.Code != 0 {
		t.Fatalf("TR koşusu çıkış %d:\n%s\n%s", s.Code, s.Stdout, s.Stderr)
	}
	type steps struct {
		Steps []struct {
			Loss     float64 `json:"loss"`
			GradNorm float64 `json:"grad_norm"`
		} `json:"steps"`
	}
	var py, tr steps
	for name, v := range map[string]*steps{"reference.json": &py, "tr.json": &tr} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if len(py.Steps) != 5 || len(tr.Steps) != 5 {
		t.Fatalf("adım sayısı: PyTorch %d, TR %d", len(py.Steps), len(tr.Steps))
	}
	rel := func(a, b float64) float64 { return math.Abs(a-b) / math.Max(math.Abs(a), 1e-12) }
	for i := range py.Steps {
		p, q := py.Steps[i], tr.Steps[i]
		if rel(p.Loss, q.Loss) > 1e-5 || rel(p.GradNorm, q.GradNorm) > 1e-4 {
			t.Errorf("adım %d: kayıp PyTorch %s TR %s; gradyan normu PyTorch %s TR %s", i+1,
				strconv.FormatFloat(p.Loss, 'g', 10, 64), strconv.FormatFloat(q.Loss, 'g', 10, 64),
				strconv.FormatFloat(p.GradNorm, 'g', 8, 64), strconv.FormatFloat(q.GradNorm, 'g', 8, 64))
		}
	}
}
