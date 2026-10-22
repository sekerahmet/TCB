package profiler

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ReportText son raporun metin tablosudur (tr profil, tr izle'nin sonu).
func ReportText(g *Snapshot) string {
	var b strings.Builder
	write := func(s string) { b.WriteString(s + "\n") }
	exit := ""
	if g.ExitCode != nil {
		exit = " · " + messages.Text(messages.H8230, *g.ExitCode)
	}
	write("⏱ " + messages.Text(messages.H8231, g.Program, durationText(g.Seconds), exit))
	if g.Step == nil {
		write(messages.Text(messages.H8232, noStep()))
	} else {
		write(messages.Text(messages.H8233, full(g.Step.Count), count(g.Step.MsPerStep), count(g.Step.StepsPerSec)))
	}
	t := g.Tensor
	write(messages.Text(messages.H8234,
		percent(t.Share), durationText(t.Seconds), full(t.Calls), nBytes(t.Bytes), percent(max(0, 1-t.Share))))
	if o := g.Sampling; o != nil {
		write(messages.Text(messages.H8235, durationText(o.IntervalMs/1000), full(o.Samples), percent(o.TensorShare)))
	}
	b.WriteString("\n")
	reportOps(&b, t)
	reportImplicit(&b, t.Implicit)
	reportGPU(&b, g)
	if o := g.Sampling; o != nil {
		b.WriteString("\n")
		reportLines(&b, o)
	}
	reportGPUPeak(&b, g)
	b.WriteString("\n")
	write(messages.Text(messages.H8236,
		nBytes(int64(g.Memory.GoHeap)), nBytes(int64(g.Memory.GoSys)), nBytes(g.Memory.CPUTensorAlloc), gpuMemory(g.GPU)))
	write(messages.Text(messages.H8237, g.GC.Count, durationText(g.GC.PauseSec)))
	return b.String()
}

// reportOps tensör işlemleri tablosudur (en pahalı 15).
func reportOps(b *strings.Builder, t TensorInfo) {
	if len(t.Ops) == 0 {
		b.WriteString(messages.Text(messages.H8238, tensorOps()) + "\n")
		return
	}
	hasStep := t.Ops[0].MsPerStep != nil
	header := left(tensorOps(), 30) + right(messages.Text(messages.H8239), 10) + right(messages.Text(messages.H8240), 6) + right(messages.Text(messages.H8241), 12)
	if hasStep {
		header += right(messages.Text(messages.H8242), 10)
	}
	b.WriteString(header + right(messages.Text(messages.H8243), 10) + right(messages.Text(messages.H8244), 7) + "\n")
	for i, x := range t.Ops {
		if i == 15 {
			b.WriteString("  " + messages.Text(messages.H8245, len(t.Ops)-i) + "\n")
			break
		}
		line := "  " + left(x.Name, 28) + right(durationText(x.Seconds), 10) + right(percent(x.Share), 6) + right(full(x.Calls), 12)
		if hasStep && x.MsPerStep != nil {
			line += right(msText(*x.MsPerStep), 10)
		}
		b.WriteString(line + right(nBytes(x.Bytes), 10) + right(x.Device, 7) + "\n")
	}
}

// reportImplicit örtük kopya ve dönüşüm tablosudur (en büyük 12, elemana göre).
func reportImplicit(b *strings.Builder, list []ImplicitInfo) {
	if len(list) == 0 {
		return
	}
	b.WriteString("\n" + messages.Text(messages.H8287) + "\n")
	b.WriteString(left("", 64) + right(messages.Text(messages.H8241), 10) + right(messages.Text(messages.H8288), 16) +
		right(messages.Text(messages.H8239), 10) + right(messages.Text(messages.H8244), 7) + "\n")
	for i, x := range list {
		if i == 12 {
			b.WriteString("  " + messages.Text(messages.H8245, len(list)-i) + "\n")
			break
		}
		t := ""
		if x.Measured > 0 {
			t = durationText(x.Seconds)
		}
		b.WriteString("  " + left(x.Name, 22) + left(x.ParentName, 40) + right(full(x.Calls), 10) +
			right(full(x.Elements), 16) + right(t, 10) + right(x.Device, 7) + "\n")
	}
}

// reportLines en sıcak satırlar ve fonksiyonlardır.
func reportLines(b *strings.Builder, o *SamplingInfo) {
	b.WriteString(messages.Text(messages.H8246) + "\n")
	for i, s := range o.Lines {
		if i == 10 {
			break
		}
		back := ""
		if s.BackwardShare > 0 {
			back = messages.Text(messages.H8289, percent(s.BackwardShare))
		}
		fmt.Fprintf(b, "  %s  %s  %s  %s  %s\n", right(percent(s.Share), 5), left(linePos(s), 30), right(messages.Text(messages.H8247, percent(s.TensorShare)), 11), left(back, 9), trim(lineText(s), 70))
	}
	b.WriteString(messages.Text(messages.H8248) + "\n")
	for i, f := range o.Functions {
		if i == 8 {
			break
		}
		fmt.Fprintf(b, "  %s  %s  %s\n", right(percent(f.Share), 5), left(functionName(f.Name), 30), durationText(f.Seconds))
	}
}

// reportGPUPeak tepe GPU belleği anında canlı tensörleri ayıran satırlardır (en büyük 15):
// pay, bayt, tensör deposu sayısı × tür, konum ve satırın metni. Satır örneklemesi kapalıysa ya da GPU yoksa yazılmaz.
func reportGPUPeak(b *strings.Builder, g *Snapshot) {
	if len(g.gpuPeak) == 0 {
		return
	}
	b.WriteString("\n" + messages.Text(messages.H8283, nBytes(int64(g.gpuPeakTotal))) + "\n")
	for _, s := range g.gpuPeak {
		kind := full(int64(s.Blocks)) + " × " + s.DType
		fmt.Fprintf(b, "  %s  %s  %s  %s  %s\n", right(percent(s.Share), 5), right(nBytes(int64(s.Bytes)), 9), left(kind, 14), left(linePos(s.Line), 22), trim(lineText(s.Line), 64))
	}
}

// reportGPU GPU çekirdeklerinin cihazda geçen süresidir (aileler) ve ölçümü profil açtıysa
// GPU arka ucunun kendi raporu.
func reportGPU(b *strings.Builder, g *Snapshot) {
	gp := g.GPU
	if !gp.Used {
		return
	}
	if !gp.Measured || len(gp.Families) == 0 {
		b.WriteString(messages.Text(messages.H8249, gp.Note) + "\n")
		return
	}
	b.WriteString(messages.Text(messages.H8250, durationText(gp.Seconds), percent(ratio(gp.Seconds, g.Seconds))))
	if g.Step != nil && g.Step.All > 0 {
		b.WriteString(" · " + messages.Text(messages.H8251, msText(gp.Seconds*1000/float64(g.Step.All))))
	}
	fmt.Fprintf(b, " · %s\n", familySummary(gp.Families, 5))
	if gp.summary != "" {
		b.WriteString("\n" + gp.summary + "\n")
	}
}
