package profiler

import (
	"fmt"
	"strings"
)

// Görüntünün metin biçimleri: canlı ekran (terminal), düz özet satırı (terminal değilken
// 10 sn'de bir) ve son rapor (tr profil, tr izle'nin sonu).

// Screen canlı ekranın görüntü dışındaki parçalarıdır.
type Screen struct {
	Width      int      // terminal sütunu
	OutputFile string   // programın çıktısının tamamı (boşsa gösterilmez)
	LastOutput []string // programın son çıktı satırları
}

// ScreenLines canlı ekranın satırlarıdır; her satır genişliğe kırpılır.
func ScreenLines(g *Snapshot, e Screen) []string {
	w := e.Width
	if w < 40 {
		w = 80
	}
	var s []string
	add := func(f string, a ...any) { s = append(s, trim(fmt.Sprintf(f, a...), w)) }
	add("⏱ izleniyor: %s · %s · durdurmak için Ctrl+C", g.Program, durationText(g.Seconds))
	p := g.Window
	if p == nil {
		p = &WindowInfo{}
	}
	switch {
	case g.Step == nil:
		add("Adım       %s", noStep())
	default:
		a := g.Step
		ad := full(a.Count)
		if a.Total > 0 {
			ad += "/" + full(a.Total)
		}
		ms, rate := a.MsPerStep, a.StepsPerSec
		if p.Step > 0 {
			ms, rate = p.MsPerStep, p.StepsPerSec
		}
		remaining := ""
		if a.Total > a.Count && rate > 0 {
			remaining = " · kalan ~" + durationText(float64(a.Total-a.Count)/rate)
		}
		add("Adım       %s · %s ms/adım · %s adım/sn%s", ad, count(ms), count(rate), remaining)
	}
	add("Zaman      tensör işlemleri %s · geri kalanı (TR kodu, bekleme) %s   (son %s)",
		percent(p.TensorShare), percent(max(0, 1-p.TensorShare)), durationText(p.Seconds))
	s = append(s, "")
	s = append(s, opTable(p.Ops, p.Step, p.Seconds, w, 10)...)
	if g.GPU.Used {
		add("GPU        %s", gpuWindowText(g.GPU, p))
	}
	if g.Sampling != nil {
		s = append(s, "")
		s = append(s, lineTable(p.Lines, w, 5)...)
	}
	s = append(s, "")
	add("Bellek     Go yığını %s · CPU tensör ayırması %s%s", nBytes(int64(g.Memory.GoHeap)),
		allocRate(p.CPUTensorAlloc, p.Step, p.Seconds), gpuMemory(g.GPU))
	add("Çöp top.   %d kez, toplam duraklama %s (son %s: %d kez, %s)", g.GC.Count, durationText(g.GC.PauseSec),
		durationText(p.Seconds), p.GCCount, durationText(p.PauseSec))
	if e.OutputFile != "" || len(e.LastOutput) > 0 {
		s = append(s, "")
		add("Programın çıktısı (tamamı: %s)", e.OutputFile)
		for _, c := range e.LastOutput {
			add("  │ %s", c)
		}
	}
	return s
}

// opTable en pahalı n işlemin tablosudur (pencere): adım varsa adım başına.
func opTable(is []OpInfo, step int64, dur float64, w, n int) []string {
	if len(is) == 0 {
		return []string{tensorOps() + ": yok"}
	}
	nameW := max(20, min(36, w-40))
	header := tensorOps() + " (en pahalı)"
	unit, callUnit := "ms", "çağrı/sn"
	if step > 0 {
		unit, callUnit = "ms/adım", "çağrı/adım"
	}
	s := []string{trim(left(header, nameW+2)+right("pay", 6)+right(unit, 10)+right(callUnit, 12)+right("cihaz", 7), w)}
	for i, b := range is {
		if i == n {
			break
		}
		ms := b.Seconds * 1000
		calls := float64(b.Calls) / max(dur, 1e-9)
		if step > 0 {
			ms /= float64(step)
			calls = float64(b.Calls) / float64(step)
		}
		s = append(s, trim("  "+left(b.Name, nameW)+right(percent(b.Share), 6)+right(msText(ms), 10)+right(count(calls), 12)+right(b.Device, 7), w))
	}
	return s
}

// lineTable en sıcak n satırdır.
func lineTable(ss []LineInfo, w, n int) []string {
	if len(ss) == 0 {
		return []string{"Sıcak satırlar: henüz örnek yok"}
	}
	s := []string{trim(left("Sıcak satırlar (öz süre)", 34)+right("pay", 6)+right("tensörde", 9), w)}
	for i, b := range ss {
		if i == n {
			break
		}
		pos := linePos(b)
		s = append(s, trim("  "+left(pos, 32)+right(percent(b.Share), 6)+right(percent(b.TensorShare), 9)+"  "+lineText(b), w))
	}
	return s
}

// gpuWindowText canlı ekranın GPU satırıdır: çekirdeklerin pencerede cihazda geçen
// süresinin payı (GPU'nun meşgul olduğu zaman), adım başına süresi ve en pahalı aileler.
func gpuWindowText(g GPUInfo, p *WindowInfo) string {
	if !g.Measured {
		return g.Note
	}
	if p.GPUSec <= 0 {
		return "çekirdekler: bu pencerede ölçüm yok"
	}
	m := "çekirdekler " + percent(p.GPUShare) + " meşgul"
	if p.Step > 0 {
		m += " · " + msText(p.GPUSec*1000/float64(p.Step)) + " ms/adım"
	}
	return m + " · " + familySummary(p.GPUFamilies, 3)
}

// familySummary en pahalı n çekirdek ailesinin kısa yazımıdır: "carpim %41 · dikkat %20".
func familySummary(families []KernelInfo, n int) string {
	var p []string
	for i, c := range families {
		if i == n {
			break
		}
		p = append(p, c.Name+" "+percent(c.Share))
	}
	return strings.Join(p, " · ")
}

// allocRate ayrılan baytı adım başına (adım varsa) ya da saniye başına yazar.
func allocRate(n, step int64, dur float64) string {
	if step > 0 {
		return nBytes(n/step) + "/adım"
	}
	if dur > 0 {
		return nBytes(int64(float64(n)/dur)) + "/sn"
	}
	return nBytes(n)
}

// gpuMemory GPU belleğinin kısa yazımıdır (GPU kullanılmadıysa boş).
func gpuMemory(g GPUInfo) string {
	var p []string
	for _, b := range g.Memory {
		ad := "GPU"
		if len(g.Memory) > 1 {
			ad = fmt.Sprintf("GPU %d", b.Device)
		}
		p = append(p, fmt.Sprintf("%s canlı %s (ayrılmış %s, tepe %s)", ad, nBytes(int64(b.Live)),
			nBytes(int64(b.Allocated)), nBytes(int64(b.PeakLive))))
	}
	if len(p) == 0 {
		return ""
	}
	return " · " + strings.Join(p, " · ")
}

// SummaryLine terminal değilken (10 sn'de bir) yazılan tek satırlık özettir.
func SummaryLine(g *Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "⏱ %s", durationText(g.Seconds))
	if g.Step != nil {
		fmt.Fprintf(&b, " · adım %s", full(g.Step.Count))
		if g.Step.Total > 0 {
			fmt.Fprintf(&b, "/%s", full(g.Step.Total))
		}
		ms := g.Step.MsPerStep
		if g.Window != nil && g.Window.Step > 0 {
			ms = g.Window.MsPerStep
		}
		fmt.Fprintf(&b, " · %s ms/adım", count(ms))
	}
	share, is := g.Tensor.Share, g.Tensor.Ops
	if g.Window != nil {
		share, is = g.Window.TensorShare, g.Window.Ops
	}
	fmt.Fprintf(&b, " · tensör %s", percent(share))
	var p []string
	for i, x := range is {
		if i == 3 {
			break
		}
		p = append(p, x.Name+" "+percent(x.Share))
	}
	if len(p) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(p, ", "))
	}
	if g.Window != nil && len(g.Window.Lines) > 0 {
		fmt.Fprintf(&b, " · satır %s %s", linePos(g.Window.Lines[0]), percent(g.Window.Lines[0].Share))
	}
	fmt.Fprintf(&b, " · Go yığını %s · çöp toplama %d (%s)", nBytes(int64(g.Memory.GoHeap)), g.GC.Count, durationText(g.GC.PauseSec))
	return b.String()
}
