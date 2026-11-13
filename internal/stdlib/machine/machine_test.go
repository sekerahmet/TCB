package machine

import (
	"runtime"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestMachineCPUFeaturesTable .TR_cpu_features tablonun öğelerinden, tablonun sırasıyla ve
// tekrarsızdır; bu dizüstünde (13th Gen Intel i7-13700H: AVX2'li, AVX-512'siz) ve G4'te
// bilinen işlemcide avx2 vardır.
func TestMachineCPUFeaturesTable(t *testing.T) {
	table := map[string][]string{
		"amd64": {"sse4_2", "avx", "avx2", "fma", "f16c", "avx512f"},
		"arm64": {"neon"},
	}[runtime.GOARCH]
	v, err := read().GetProperty(codes.MemCpuFeatures, "")
	if err != nil {
		t.Fatal(err)
	}
	l, ok := v.(*interp.ListVal)
	if !ok {
		t.Fatalf("liste bekleniyordu: %T", v)
	}
	last := -1
	var got []string
	for _, e := range l.Elements {
		s := e.(*interp.StringVal).Val
		got = append(got, s)
		i := slices.Index(table, s)
		if i <= last {
			t.Fatalf("liste tablonun sırasında, tekrarsız ve tablodan değil: %v", got)
		}
		last = i
	}
	if runtime.GOARCH == "amd64" && !slices.Contains(got, "avx2") {
		t.Logf("bu işlemcide avx2 yok: %v", got)
	}
	t.Logf("%s: %v", runtime.GOARCH, got)
}

// TestMachineMembers her üyenin tipi ve akla yatkınlığı; okunamayan üye kodlu hata verir.
func TestMachineMembers(t *testing.T) {
	m := read()
	get := func(kd codes.Code) (interp.Value, error) { return m.GetProperty(kd, "") }
	total, err := get(codes.MemMemoryTotalBytes)
	if err != nil {
		t.Fatal(err)
	}
	if total.(*interp.IntVal).Val < 256<<20 {
		t.Fatalf("toplam bellek akla yatkın değil: %v", total)
	}
	avail, err := get(codes.MemMemoryAvailableBytes)
	switch runtime.GOOS {
	case "windows", "linux":
		if err != nil {
			t.Fatal(err)
		}
		if a := avail.(*interp.IntVal).Val; a <= 0 || a > total.(*interp.IntVal).Val {
			t.Fatalf("kullanılabilir bellek %d, toplam %v", a, total)
		}
	default:
		if e, ok := messages.As(err); !ok || e.Code != messages.H7735 {
			t.Fatalf("H7735 bekleniyordu: %v", err)
		}
	}
	limit, err := get(codes.MemMemoryLimitBytes)
	if err == nil {
		if iv, ok := limit.(*interp.IntVal); ok && iv.Val <= 0 {
			t.Fatalf("sınır pozitif ya da yok olmalı: %v", limit)
		}
		t.Logf("sınır: %v", limit.Inspect())
	} else if runtime.GOOS == "windows" {
		t.Fatalf("Windows'ta sınır okunmalı: %v", err)
	}
	cpus, _ := get(codes.MemUsableCpus)
	if cpus.(*interp.IntVal).Val < 1 {
		t.Fatalf("çekirdek sayısı %v", cpus)
	}
	if _, err := m.GetProperty(codes.MemSize, "bayt"); err == nil {
		t.Fatalf("makinenin .TR_size'ı olmamalı")
	}
}
