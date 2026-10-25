package profiler

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// sampleSnapshot canlı ekranın sınanması için elle kurulmuş bir görüntüdür: 50 adımlık
// pencerede 2 sn, GPU'nun 1,5 sn'si meşgul.
func sampleSnapshot() *Snapshot {
	ms := 20.0
	return &Snapshot{
		Program: "egit.tr", Seconds: 65, Step: &StepInfo{Count: 300, Total: 1000, MsPerStep: 41, StepsPerSec: 24},
		Tensor:   TensorInfo{Share: 0.8},
		GPU:      GPUInfo{Used: true, Measured: true, Memory: []GPUMemory{{Live: 3 << 30, Allocated: 4 << 30, PeakLive: 3 << 30}}},
		Sampling: &SamplingInfo{}, Memory: MemoryInfo{GoHeap: 200 << 20},
		Window: &WindowInfo{
			Seconds: 2, Step: 50, MsPerStep: 40, StepsPerSec: 25, TensorShare: 0.75, CPUTensorAlloc: 50 << 20,
			Ops:    []OpInfo{{Name: "matris çarpımı (@)", Share: 0.5, Seconds: 1, Calls: 100, Device: "gpu", MsPerStep: &ms}},
			Lines:  []LineInfo{{File: "egit.tr", Line: 12, Share: 0.6, TensorShare: 0.9, Text: "geri_yayılım(kayıp)"}},
			GPUSec: 1.5, GPUShare: 0.75, GPUFamilies: []KernelInfo{{Name: "carpim", Share: 0.6}, {Name: "dikkat", Share: 0.3}},
		},
	}
}

// Canlı ekran: adım hızı penceredendir; GPU satırı meşguliyet, adım başına GPU süresi ve
// aileler; bellek satırı Go yığını, CPU tensör ayırması (adım başına) ve GPU belleği.
func TestScreenLines(t *testing.T) {
	lines := ScreenLines(sampleSnapshot(), Screen{Width: 200, OutputFile: "c.txt", LastOutput: []string{"adım 300"}})
	text := strings.Join(lines, "\n")
	for _, p := range []string{
		"300/1 000 · 40 ms/adım · 25 adım/sn · kalan ~28 sn",
		"tensör işlemleri %75",
		"matris çarpımı (@)", "ms/adım", "çağrı/adım",
		"kerneller %75 meşgul · 30,0 ms/adım · carpim %60 · dikkat %30",
		"egit.tr:12", "geri_yayılım(kayıp)",
		"Go yığını 200,0 MB", "CPU tensör ayırması 1,0 MB/adım", "GPU canlı 3,0 GB (ayrılmış 4,0 GB, tepe 3,0 GB)",
		"Programın çıktısı (tamamı: c.txt)", "│ adım 300",
	} {
		if !strings.Contains(text, p) {
			t.Errorf("ekranda %q yok:\n%s", p, text)
		}
	}
	// Dar terminalde satırlar genişliği aşmaz.
	for _, s := range ScreenLines(sampleSnapshot(), Screen{Width: 60}) {
		if n := utf8.RuneCountInString(s); n > 60 {
			t.Errorf("%d karakterlik satır 60 sütuna sığmıyor: %q", n, s)
		}
	}
}

// Ölçüm kapalıyken GPU satırı nedenini söyler; GPU kullanılmadıysa GPU satırı yoktur.
func TestScreenGPUStates(t *testing.T) {
	g := sampleSnapshot()
	g.GPU.Measured, g.GPU.Note = false, "kernel süreleri ölçülmüyor"
	if m := strings.Join(ScreenLines(g, Screen{Width: 200}), "\n"); !strings.Contains(m, "GPU        kernel süreleri ölçülmüyor") {
		t.Errorf("ölçüm kapalı notu yok:\n%s", m)
	}
	g.GPU = GPUInfo{Note: "GPU kullanılmadı"}
	for _, s := range ScreenLines(g, Screen{Width: 200}) {
		if strings.HasPrefix(s, "GPU ") || strings.Contains(s, "GPU canlı") {
			t.Errorf("GPU kullanılmadığı halde GPU satırı: %q", s)
		}
	}
}
