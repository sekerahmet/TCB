package profiler

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Görüntünün metin biçimleri: canlı ekran (terminal), düz özet satırı (terminal değilken
// 10 sn'de bir) ve son rapor (tr profile, tr watch'nin sonu).

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
	add := func(line string) { s = append(s, trim(line, w)) }
	add("⏱ " + messages.Text(messages.H8260, g.Program, durationText(g.Seconds)))
	p := g.Window
	if p == nil {
		p = &WindowInfo{}
	}
	switch {
	case g.Step == nil:
		add(messages.Text(messages.H8261, noStep()))
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
			remaining = " · " + messages.Text(messages.H8262, durationText(float64(a.Total-a.Count)/rate))
		}
		add(messages.Text(messages.H8263, ad, count(ms), count(rate), remaining))
	}
	add(messages.Text(messages.H8264,
		percent(p.TensorShare), percent(max(0, 1-p.TensorShare)), durationText(p.Seconds)))
	s = append(s, "")
	s = append(s, opTable(p.Ops, p.Step, p.Seconds, w, 10)...)
	if g.GPU.Used {
		add(messages.Text(messages.H8265, gpuWindowText(g.GPU, p)))
	}
	if g.Sampling != nil {
		s = append(s, "")
		s = append(s, lineTable(p.Lines, w, 5)...)
	}
	s = append(s, "")
	add(messages.Text(messages.H8266, nBytes(int64(g.Memory.GoHeap)),
		allocRate(p.CPUTensorAlloc, p.Step, p.Seconds), gpuMemory(g.GPU)))
	add(messages.Text(messages.H8267, g.GC.Count, durationText(g.GC.PauseSec),
		durationText(p.Seconds), p.GCCount, durationText(p.PauseSec)))
	if e.OutputFile != "" || len(e.LastOutput) > 0 {
		s = append(s, "")
		add(messages.Text(messages.H8268, e.OutputFile))
		for _, c := range e.LastOutput {
			add("  │ " + c)
		}
	}
	return s
}

// opTable en pahalı n işlemin tablosudur (pencere): adım varsa adım başına.
func opTable(is []OpInfo, step int64, dur float64, w, n int) []string {
	if len(is) == 0 {
		return []string{messages.Text(messages.H8238, tensorOps())}
	}
	nameW := max(20, min(36, w-40))
	header := messages.Text(messages.H8269, tensorOps())
	unit, callUnit := "ms", messages.Text(messages.H8270)
	if step > 0 {
		unit, callUnit = messages.Text(messages.H8242), messages.Text(messages.H8271)
	}
	s := []string{trim(left(header, nameW+2)+right(messages.Text(messages.H8240), 6)+right(unit, 10)+right(callUnit, 12)+right(messages.Text(messages.H8244), 7), w)}
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
		return []string{messages.Text(messages.H8272)}
	}
	s := []string{trim(left(messages.Text(messages.H8273), 34)+right(messages.Text(messages.H8240), 6)+right(messages.Text(messages.H8274), 9), w)}
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
		return messages.Text(messages.H8275)
	}
	m := messages.Text(messages.H8276, percent(p.GPUShare))
	if p.Step > 0 {
		m += " · " + messages.Text(messages.H8251, msText(p.GPUSec*1000/float64(p.Step)))
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
		return messages.Text(messages.H8277, nBytes(n/step))
	}
	if dur > 0 {
		return messages.Text(messages.H8278, nBytes(int64(float64(n)/dur)))
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
		p = append(p, messages.Text(messages.H8279, ad, nBytes(int64(b.Live)),
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
		b.WriteString(" · " + messages.Text(messages.H8280, full(g.Step.Count)))
		if g.Step.Total > 0 {
			fmt.Fprintf(&b, "/%s", full(g.Step.Total))
		}
		ms := g.Step.MsPerStep
		if g.Window != nil && g.Window.Step > 0 {
			ms = g.Window.MsPerStep
		}
		b.WriteString(" · " + messages.Text(messages.H8251, count(ms)))
	}
	share, is := g.Tensor.Share, g.Tensor.Ops
	if g.Window != nil {
		share, is = g.Window.TensorShare, g.Window.Ops
	}
	b.WriteString(" · " + messages.Text(messages.H8247, percent(share)))
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
		b.WriteString(" · " + messages.Text(messages.H8281, linePos(g.Window.Lines[0]), percent(g.Window.Lines[0].Share)))
	}
	b.WriteString(" · " + messages.Text(messages.H8282, nBytes(int64(g.Memory.GoHeap)), g.GC.Count, durationText(g.GC.PauseSec)))
	return b.String()
}
