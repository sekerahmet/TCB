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

// convLaunch çekirdeği eleman başına iş parçacığıyla başlatır (n = yazılan öğe sayısı).
func (g *backend) convLaunch(key string, gen func() *emitter, out tensor.Storage, in tensor.View, n int, f tensor.Conv1dLayout) {
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
	c := g.takeKernel(key, gen)
	g.launch(c, [3]uint32{blocks(n, convBlock), 1, 1}, [3]uint32{convBlock, 1, 1},
		g.param().readV(iv).write(o, 0).u32(n).u32(f.Batch).u32(f.Channels).u32(f.Length).u32(f.Kernel).
			u32(f.Frames).u32(f.Stride).u32(f.Padding).u32(f.Dilation).u32(f.Groups))
	last()
}
