package profiler

import (
	"fmt"
	"strings"
)

// ReportText son raporun metin tablosudur (tr profil, tr izle'nin sonu).
func ReportText(g *Snapshot) string {
	var b strings.Builder
	write := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	exit := ""
	if g.ExitCode != nil {
		exit = fmt.Sprintf(" · çıkış kodu %d", *g.ExitCode)
	}
	write("⏱ Profil: %s — %s%s", g.Program, durationText(g.Seconds), exit)
	if g.Step == nil {
		write("Adımlar: %s", noStep())
	} else {
		write("Adımlar: %s adım · %s ms/adım · %s adım/sn", full(g.Step.Count), count(g.Step.MsPerStep), count(g.Step.StepsPerSec))
	}
	t := g.Tensor
	write("Zaman: tensör işlemleri %s (%s, %s çağrı, %s ayrıldı) · geri kalanı %s (TR kodu, bekleme, çöp toplama)",
		percent(t.Share), durationText(t.Seconds), full(t.Calls), nBytes(t.Bytes), percent(max(0, 1-t.Share)))
	if o := g.Sampling; o != nil {
		write("Örnekleme: %s aralıkla %s örnek; tensör işlemindeki payı %s", durationText(o.IntervalMs/1000), full(o.Samples), percent(o.TensorShare))
	}
	b.WriteString("\n")
	reportOps(&b, t)
	reportGPU(&b, g)
	if o := g.Sampling; o != nil {
		b.WriteString("\n")
		reportLines(&b, o)
	}
	b.WriteString("\n")
	write("Bellek: Go yığını %s (Go'nun sistemden aldığı %s) · CPU tensör ayırması %s toplam%s",
		nBytes(int64(g.Memory.GoHeap)), nBytes(int64(g.Memory.GoSys)), nBytes(g.Memory.CPUTensorAlloc), gpuMemory(g.GPU))
	write("Çöp toplama: %d kez, toplam duraklama %s", g.GC.Count, durationText(g.GC.PauseSec))
	return b.String()
}

// reportOps tensör işlemleri tablosudur (en pahalı 15).
func reportOps(b *strings.Builder, t TensorInfo) {
	if len(t.Ops) == 0 {
		b.WriteString(tensorOps() + ": yok\n")
		return
	}
	hasStep := t.Ops[0].MsPerStep != nil
	header := left(tensorOps(), 30) + right("süre", 10) + right("pay", 6) + right("çağrı", 12)
	if hasStep {
		header += right("ms/adım", 10)
	}
	b.WriteString(header + right("ayrılan", 10) + right("cihaz", 7) + "\n")
	for i, x := range t.Ops {
		if i == 15 {
			fmt.Fprintf(b, "  … %d işlem daha\n", len(t.Ops)-i)
			break
		}
		line := "  " + left(x.Name, 28) + right(durationText(x.Seconds), 10) + right(percent(x.Share), 6) + right(full(x.Calls), 12)
		if hasStep && x.MsPerStep != nil {
			line += right(msText(*x.MsPerStep), 10)
		}
		b.WriteString(line + right(nBytes(x.Bytes), 10) + right(x.Device, 7) + "\n")
	}
}

// reportLines en sıcak satırlar ve fonksiyonlardır.
func reportLines(b *strings.Builder, o *SamplingInfo) {
	b.WriteString("En çok zaman alan satırlar (öz süre; tensör: satırın tensör işleminde geçen payı):\n")
	for i, s := range o.Lines {
		if i == 10 {
			break
		}
		fmt.Fprintf(b, "  %s  %s  %s  %s\n", right(percent(s.Share), 5), left(linePos(s), 30), right("tensör "+percent(s.TensorShare), 11), trim(lineText(s), 70))
	}
	b.WriteString("Fonksiyonlar (öz süre):\n")
	for i, f := range o.Functions {
		if i == 8 {
			break
		}
		fmt.Fprintf(b, "  %s  %s  %s\n", right(percent(f.Share), 5), left(functionName(f.Name), 30), durationText(f.Seconds))
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
		fmt.Fprintf(b, "GPU çekirdekleri: %s\n", gp.Note)
		return
	}
	fmt.Fprintf(b, "GPU çekirdekleri: cihazda %s (program süresine oranı %s)", durationText(gp.Seconds), percent(ratio(gp.Seconds, g.Seconds)))
	if g.Step != nil && g.Step.Count > 0 {
		fmt.Fprintf(b, " · %s ms/adım", msText(gp.Seconds*1000/float64(g.Step.Count)))
	}
	fmt.Fprintf(b, " · %s\n", familySummary(gp.Families, 5))
	if gp.summary != "" {
		b.WriteString("\n" + gp.summary + "\n")
	}
}
