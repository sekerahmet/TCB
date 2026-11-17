package tensor

import (
	"runtime"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// İşlemcide türevsiz bir boyutlu evrişim, açma tamponu olmadan. Teknik onnxruntime'ın MLAS'ındaki
// dilimli yoldur (microsoft/onnxruntime@f1b4861a, core/mlas/lib/convolve.cpp: :595-610, :644,
// :652-657, :668); kod bu motorun çarpım parçalarıyla yazıldı:
//   - Ağırlık [Cout / g, Cin / g · K] bir kez mr satırlık A panellerine paketlenir (packA, kcBlock'luk
//     K blokları); paketleme çekirdeğin gövdesindedir (izin alınmışken), hazırlıkta değil.
//   - İş = (toplu, grup, 128 sütunluk çıkış dilimi, satır bloğu). İş, dilimin girdi pencerelerini
//     K bloğu başına işçinin kendi tamponuna doğrudan nr sütunluk B panelleri olarak açar (adım 1'de
//     içerideki 16 öğe tek kopya) ve mikro çekirdekle çıkışa ldc = çıkış boyu ile ekler.
//   - Sapma dilimin bütün K bloklarından sonra eklenir.
//
// Eski yolla (unfold1d, gemm, devrik kopya, sapma toplaması) bit bit aynıdır: iki yolda da her çıkış
// öğesi sıfırdan başlayan biriktiriciyle, K'nın kcBlock bloklarıyla ve blok içinde aynı k sırasıyla
// (iç indis c·K + j) toplanır, bloklar sırayla çıkışa eklenir; mikro çekirdeğin sonucu satır ve sütun
// döşemesinden bağımsızdır (cpu_micro_amd64.go). Sapma iki yolda da toplamın üstüne bir kez eklenir.
// Tek ayrım: eski yolun çarpımı M = B · çıkış = 1'de gemv'e gider (useGemv) ve orada K tek nokta
// çarpımıdır; o çağrı bu yola girmez (Conv1dDirectOK).

// convTile çıkış diliminin sütun sayısıdır (MLAS_SGEMM_STRIDEN, mlasi.h:377).
const convTile = 128

// Conv1dDirectOK evrişimin bu yoldan yapılabileceğidir: x, w ve b işlemcide, float32, iz tensörü
// değil, türev istenmiyor ve eski yolun çarpımı gemv değil. Şekiller denetlenmiş olmalıdır.
// Karışık hassasiyet bloğunu çağıran denetler.
func Conv1dDirectOK(x, w, b *Tensor, o Conv1dOptions) bool {
	for _, t := range []*Tensor{x, w, b} {
		if t == nil {
			continue
		}
		if t.Device() != CPU || t.dtyp != Float32 || t.sym != nil || (GradEnabled() && t.RequiresGrad()) {
			return false
		}
	}
	g := o.Groups
	frames := o.Length(x.shape[2], w.shape[2])
	return !useGemv(x.shape[0]*frames, w.shape[0]/g, w.shape[1]*w.shape[2])
}

// Conv1dDirect Conv1dKernel'i kilit altında çalıştırır.
func Conv1dDirect(x, w, b *Tensor, o Conv1dOptions) *Tensor {
	r, k := Conv1dKernel(x, w, b, o)
	k.RunLocked()
	return r
}

// Conv1dKernel Conv1d'nin işlemcide türevsiz eşidir (Conv1dDirectOK doğru olmalı): hazırlık çıkışı
// ayırır, evrişim dönen çekirdektir. x, w ve b bitişik olmalıdır (çağıran Contiguous yapar).
func Conv1dKernel(x, w, b *Tensor, o Conv1dOptions) (*Tensor, *Kernel) {
	be := sameDevice("conv1d", x, w, b)
	for _, t := range []*Tensor{x, w, b} {
		if t != nil && !t.IsContiguous() {
			panic(messages.Internal("tensor: conv1d kernel needs contiguous inputs"))
		}
	}
	s := conv1dShape{batch: x.shape[0], cin: x.shape[1], l: x.shape[2], cout: w.shape[0], k: w.shape[2], o: o}
	s.frames = o.Length(s.l, s.k)
	sy := counterStart(be, OpKindMatMul).start()
	out := zerosOf(isMatMul, be, []int{s.batch, s.cout, s.frames}, Float32)
	// Gövde yalnız burada alınan depo ve konumlarla çalışır.
	xs, xo, xn := x.storage, x.offset, x.Numel()
	ws, wo, wn := w.storage, w.offset, w.Numel()
	reads, holds := []Storage{xs, ws}, []*Tensor{x, w, out}
	var bs Storage
	bo := 0
	if b != nil {
		bs, bo = b.storage, b.offset
		reads, holds = append(reads, bs), append(holds, b)
	}
	ys := out.storage
	k := newKernel(be, []Storage{ys}, reads, holds, func() {
		var bd []float32
		if bs != nil {
			bd = cd(bs)[bo : bo+s.cout]
		}
		conv1dCPU(s, cd(xs)[xo:xo+xn], cd(ws)[wo:wo+wn], bd, cd(ys)[:s.batch*s.cout*s.frames])
		// Depolar dilimleriyle okundu: sahipleri hesap bitene dek canlı (host_block.go).
		runtime.KeepAlive(xs)
		runtime.KeepAlive(ws)
		runtime.KeepAlive(bs)
		runtime.KeepAlive(ys)
		sy.finish()
	})
	return out, k
}

// conv1dShape evrişimin boyutlarıdır: girdi [batch, cin, l], ağırlık [cout, cin / g, k], çıkış
// [batch, cout, frames].
type conv1dShape struct {
	batch, cin, l, cout, k, frames int
	o                              Conv1dOptions
}

func (s conv1dShape) cgIn() int  { return s.cin / s.o.Groups }
func (s conv1dShape) cgOut() int { return s.cout / s.o.Groups }
func (s conv1dShape) inner() int { return s.cgIn() * s.k }
func (s conv1dShape) tiles() int { return (s.frames + convTile - 1) / convTile }

// conv1dCPU y'ye (sıfırla başlar) x ile w'nin evrişimini ve sapmayı yazar.
func conv1dCPU(s conv1dShape, x, w, b, y []float32) {
	g, kt, co := s.o.Groups, s.inner(), s.cgOut()
	if len(y) == 0 {
		return
	}
	if kt == 0 {
		// Boş iç boyut: toplam 0, çıkış yalnız sapmadır (eski yolda 0 + b).
		for r := 0; b != nil && r < s.batch*s.cout; r++ {
			seg := y[r*s.frames : (r+1)*s.frames]
			for i := range seg {
				seg[i] += b[r%s.cout]
			}
		}
		return
	}
	mPad := (co + mr - 1) / mr * mr
	apBuf := takePack(&packPoolA, g*mPad*kt)
	defer packPoolA.Put(apBuf)
	ap := *apBuf
	for gi := 0; gi < g; gi++ {
		wg, ag := w[gi*co*kt:(gi+1)*co*kt], ap[gi*mPad*kt:(gi+1)*mPad*kt]
		for p0 := 0; p0 < kt; p0 += kcBlock {
			kb := min(kcBlock, kt-p0)
			packA(wg, kt, 1, 0, co, p0, kb, ag[mPad*p0:mPad*(p0+kb)])
		}
	}
	// İş azsa satır blokları bölünür (bitler değişmez: bir öğenin K toplamı tek işin içindedir).
	units := s.batch * g * s.tiles()
	rb := mPad
	if p := workers(); units < 4*p {
		rb = max(mr, mPad/((4*p+units-1)/units)/mr*mr)
	}
	rowJobs := (mPad + rb - 1) / rb
	work := s.batch * s.frames * s.cout * kt
	W := workerCount(work, minPerWorker, gemmBigJob)
	if work < singleCoreThreshold {
		W = 1
	}
	bufs := make([]*[]float32, W)
	distribute(W, units*rowJobs, func(p, t int) {
		if bufs[p] == nil {
			bufs[p] = takePack(&packPoolB, kcBlock*convTile)
		}
		i0 := (t % rowJobs) * rb
		conv1dTile(s, x, ap, b, y, *bufs[p], mPad, t/rowJobs, i0, min(co, i0+rb))
	})
	for _, p := range bufs {
		if p != nil {
			packPoolB.Put(p)
		}
	}
}

// conv1dTile u. dilimin (toplu, grup, dilim sırasıyla) [i0, i1) çıkış kanallarını hesaplar; bp
// işçinin kcBlock · convTile'lık tamponudur.
func conv1dTile(s conv1dShape, x, ap, b, y, bp []float32, mPad, u, i0, i1 int) {
	if i0 >= i1 {
		return
	}
	g, tiles, co, ci, kt := s.o.Groups, s.tiles(), s.cgOut(), s.cgIn(), s.inner()
	bi, gi, tile := u/(g*tiles), u/tiles%g, u%tiles
	t0 := tile * convTile
	tn := min(convTile, s.frames-t0)
	np := (tn + nr - 1) / nr
	xg := x[(bi*s.cin+gi*ci)*s.l : (bi*s.cin+(gi+1)*ci)*s.l]
	yg := y[(bi*s.cout+gi*co)*s.frames : (bi*s.cout+(gi+1)*co)*s.frames]
	ag := ap[gi*mPad*kt : (gi+1)*mPad*kt]
	for p0 := 0; p0 < kt; p0 += kcBlock {
		kb := min(kcBlock, kt-p0)
		openPanels(s, xg, p0, kb, t0, np, bp)
		for ip := i0 / mr; ip*mr < i1; ip++ {
			mm := min(mr, i1-ip*mr)
			apan := ag[mPad*p0+ip*kb*mr : mPad*p0+(ip+1)*kb*mr]
			for jp := 0; jp < np; jp++ {
				col0 := t0 + jp*nr
				microKernel(kb, apan, bp[jp*kb*nr:(jp+1)*kb*nr], yg[ip*mr*s.frames+col0:], s.frames, mm, min(nr, s.frames-col0))
			}
		}
	}
	if b == nil {
		return
	}
	for r := i0; r < i1; r++ {
		br := b[gi*co+r]
		seg := yg[r*s.frames+t0 : r*s.frames+t0+tn]
		for i := range seg {
			seg[i] += br
		}
	}
}

// openPanels bir grubun girdisinin (xg [cin / g, l]) [p0, p0+kb) iç satırlarını, t0'dan başlayan np
// sütun panelinde bp'ye açar: panel jp'de (kk, cc) öğesi bp[jp·kb·nr + kk·nr + cc]'dedir; iç indis
// q = c·K + j, sütun t'de değeri xg[c·l + t·adım + j·genişleme − dolgu]'dur. Dolgu ve çıkışın
// dışındaki sütunlar sıfırdır (mikro çekirdek panelin bütün nr sütununu okur).
func openPanels(s conv1dShape, xg []float32, p0, kb, t0, np int, bp []float32) {
	st, l, k := s.o.Stride, s.l, s.k
	for kk := 0; kk < kb; kk++ {
		q := p0 + kk
		row := xg[(q/k)*l : (q/k+1)*l]
		off := (q%k)*s.o.Dilation - s.o.Padding
		for jp := 0; jp < np; jp++ {
			dst := bp[jp*kb*nr+kk*nr : jp*kb*nr+kk*nr+nr]
			col0 := t0 + jp*nr
			nn := min(nr, s.frames-col0)
			if s0 := col0*st + off; st == 1 && nn == nr && s0 >= 0 && s0+nr <= l {
				copy(dst, row[s0:s0+nr])
				continue
			}
			for cc := range dst {
				v := float32(0)
				if p := (col0+cc)*st + off; cc < nn && p >= 0 && p < l {
					v = row[p]
				}
				dst[cc] = v
			}
		}
	}
}
