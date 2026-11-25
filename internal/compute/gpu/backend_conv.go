package gpu

import "github.com/sekerahmet/TCB/internal/compute/tensor"

// Arka uç: bir boyutlu evrişimin açması ve katlaması (tensor.Conv1dFolder; çekirdekler
// kernel_conv.go'da). Girdi float32 olarak okunur (16 bitlik girdi açılır, bitler değişmez);
// çıktı float32'dir, 16 bitlik hassasiyete tensör paketi işlemcideki yerde çevirir.

// Derleme anı denetimi (gpu.go'daki listenin eşi): yöntem adı değişirse tensör paketi evrişimi
// işlemcide açmaya düşerdi.
var _ tensor.Conv1dFolder = (*backend)(nil)

// Unfold1d (tensor.Conv1dFolder) x'in pencerelerini cols'a açar.
func (g *backend) Unfold1d(cols tensor.Storage, x tensor.View, f tensor.Conv1dLayout) {
	n := f.Groups * f.Batch * f.Frames * (f.Channels / f.Groups) * f.Kernel
	g.convLaunch("acma_1b", unfoldKernel, cols, x, n, f)
}

// Fold1d (tensor.Conv1dFolder) açmanın türevidir: dx[b, c, p] cols'un p'ye düşen öğelerinin
// çıkış sırasıyla toplamıdır.
func (g *backend) Fold1d(dx tensor.Storage, cols tensor.View, f tensor.Conv1dLayout) {
	g.convLaunch("katlama_1b", foldKernel, dx, cols, f.Batch*f.Channels*f.Length, f)
}

// convWide düzenin 64 bitlik çekirdeğe gittiğidir: dolgulu uzunluk 2³¹ ve üstüdür. Altında 32 bit
// yeter: adım ve açıklık dolgulu uzunluğu geçmez (foldShape.layout, 15.23c), konumlar 2³¹'in, katlamanın
// `lo + adım − 1`'i işaretsiz 2³²'nin altındadır.
func convWide(f tensor.Conv1dLayout) bool { return f.Length+2*f.Padding >= 1<<31 }

// convLaunch çekirdeği (dar ya da geniş eşini) eleman başına iş parçacığıyla başlatır (n = yazılan öğe
// sayısı).
func (g *backend) convLaunch(key string, gen func(wide bool) *emitter, out tensor.Storage, in tensor.View, n int, f tensor.Conv1dLayout) {
	g.enter()
	defer g.exit()
	checkSize(n)
	if n == 0 {
		return
	}
	gt := g.temp()
	defer gt.finish()
	iv := gt.read(in)
	o, last := gt.write(out, false)
	wide := convWide(f)
	if wide {
		key += "_64"
	}
	c := g.takeKernel(key, func() *emitter { return gen(wide) })
	p := g.param().readV(iv).write(o, 0).u32(n).u32(f.Batch).u32(f.Channels).u32(f.Length).u32(f.Kernel).u32(f.Frames)
	for _, v := range []int{f.Stride, f.Padding, f.Dilation} {
		if wide {
			p.u64(v)
		} else {
			p.u32(v)
		}
	}
	p.u32(f.Groups)
	g.launch(c, [3]uint32{blocks(n, convBlock), 1, 1}, [3]uint32{convBlock, 1, 1}, p)
	last()
}
