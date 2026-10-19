package tensor

import (
	"fmt"
	"math/bits"
	"sync"
)

// CPU arka ucu: bloklu, paketlemeli ve çok çekirdekli matris çarpımı.
//
// C += op(A)·op(B)   (op(A): M×K, op(B): K×N, C: M×N satır öncelikli)
//
// Düzen:
//   - B, nr sütun genişliğinde dikey panellere paketlenir; her panel K×nr
//     boyutunda ve bitişiktir. Böylece iç döngü B'yi düz dilim olarak okur.
//     Devrik B paketleme sırasında adımlarla okunur; ayrıca kopya gerekmez.
//   - Her iş parçası C'nin bir satır bloğudur (iş sayısı az ise satır ×
//     sütun karosu). İşçi, A'nın o satırlarını
//     kc derinliğinde mr yükseklikli panellere paketler (L2'de kalır).
//   - En içte mr×nr'lik mikro çekirdek C bloğunu yazmaçlarda biriktirir;
//     her k adımında mr+nr yükleme ile mr·nr çarpma-toplama yapar. Çekirdek
//     ve boyutu mimariye göre seçilir: amd64'te 6×16 (cpu_micro_amd64.go; AVX2/FMA'lı
//     işlemcide assembly, öbürlerinde Go), diğer mimarilerde taşınabilir Go ile 4×2
//     (cpu_micro_other.go).
//   - İş parçaları işçi havuzuna (paralel.go) atomik bir sayaçla dağıtılır;
//     katılımcı sayısı çarpımın büyüklüğüne göre seçilir (gemmBuyukIs).
const (
	kcBlock = 256     // K ekseni blok derinliği
	rowB    = 16 * mr // bir iş parçasındaki en fazla satır sayısı (mr katı)
	// minPanel sütun yönünde bölünürken bir iş parçasına düşen en az B paneli
	// (en az 32 sütun).
	minPanel = (32 + nr - 1) / nr
	// singleCoreThreshold bu kadar çarpma-toplamadan küçük işler tek çekirdekte yapılır.
	singleCoreThreshold = 1 << 18
	// minPerWorker bir işçiye düşen en az çarpma-toplama; işçi sayısı buna
	// göre sınırlanır (eşiğin hemen üstündeki işlerde gereksiz işçi olmasın).
	minPerWorker = 1 << 17
	// gemmBigJob bu kadar çarpma-toplamadan büyük çarpımlar bütün işçileri
	// kullanabilir; daha küçükleri en çok donenIsci+1 katılımcıyla yürütülür
	// (bkz. paralel.go). 2^25 çarpma-toplama tek çekirdekte ~0,5 ms sürer.
	// Ölçüm (i7-13700H, 20 iş parçacığı): modeldeki 1024×64×64 ve 1024×64×256
	// çarpımlarında 4-6 katılımcıdan sonrası kazandırmıyor, uyuyan işçileri
	// uyandırma ve GC duraklamaları yüzünden kaybettiriyordu; 512³'ten
	// büyüklerde ise bütün çekirdekler kazandırıyor (1024³: 8 işçi 390, 20 işçi
	// 470 GFLOPS; 2048³: 420 ve 500 GFLOPS).
	gemmBigJob = 1 << 25
)

// directBMaxM bu kadar satıra kadar (M) tam B panelleri paketlenmeden okunur (bkz.
// gemmParca). Ölçüm (19.10.2026, i7-13700H, satır öncelikli B, N 64-256, K 64-1024):
// M ≤ 64'te tek çekirdekte %14-83, 2 ve 20 çekirdekte %13-94 hızlı; M = 256'da tek
// çekirdekte ±%5, çok çekirdekte %5-45 hızlı; M = 512'de tek çekirdekte %8'e varan kayıp,
// M ≥ 1024'te paketli B ile aynı ya da biraz hızlı. Testler değiştirebilir.
var directBMaxM = 256

// Paketleme tamponları çağrılar arasında yeniden kullanılır (eğitimde aynı
// boyutlar her adımda yinelenir). Paketleme tamponun gereken her elemanını
// yazdığından sıfırlama gerekmez. A ve B tamponları boyutları çok farklı
// olduğu için ayrı yığınlardadır.
//
// sync.Pool yerine sınırlı bir yığın kullanılır: sync.Pool her GC'de
// boşalıyordu (modelde adım başına ~1,4 GC) ve boyutlar karışık geldiğinde
// her uyuşmazlıkta yeni tampon ayrılıyordu; 100 adımda ~390 MB gereksiz
// ayırma (GC sıklığını artırır) ölçüldü.
var (
	packPoolA = bufStack{largest: bufMax}
	packPoolB = bufStack{largest: bufMaxB}
)

const (
	// bufMax bundan büyük tamponlar saklanmaz (büyük çarpımlarda
	// ayırma maliyeti önemsizdir): 1M eleman, 4 MB.
	bufMax = 1 << 20
	// bufMaxB paketlenmiş B tamponunun en büyük boyu (4M eleman, 16 MB): toplu
	// çarpımda B bütün toplu için birlikte paketlenir (ör. 64×(1024×64)·(64×1024) ileri
	// yönde tam 4M eleman); daha büyüğü gerektiğinde toplu parçalara bölünür.
	bufMaxB = 1 << 22
	// bufTotalMax bir yığında tutulan tamponların toplam kapasitesi
	// (eleman): 8M eleman, 32 MB. Karakter modelinde gereken toplam ~2 MB'tır.
	bufTotalMax = 1 << 23
)

// bufStack paketleme tamponlarının kilitli, sınırlı bir yığını.
type bufStack struct {
	largest int // bundan büyük tamponlar saklanmaz
	mu      sync.Mutex
	t       []*[]float32
	total   int // t'deki tamponların toplam kapasitesi
}

// takePack yığından en az n elemanlı en küçük tamponu alır (yoksa yenisini
// ayırır). Yeni tamponun kapasitesi ikinin kuvvetine yuvarlanır: yakın
// boyutlar aynı tamponu paylaşır.
func takePack(y *bufStack, n int) *[]float32 {
	y.mu.Lock()
	k := -1
	for i, p := range y.t {
		if c := cap(*p); c >= n && (k < 0 || c < cap(*y.t[k])) {
			k = i
		}
	}
	if k >= 0 {
		p := y.t[k]
		last := len(y.t) - 1
		y.t[k], y.t[last] = y.t[last], nil
		y.t = y.t[:last]
		y.total -= cap(*p)
		y.mu.Unlock()
		*p = (*p)[:n]
		return p
	}
	y.mu.Unlock()
	s := make([]float32, n, 1<<bits.Len(uint(n-1)))
	return &s
}

// Put tamponu yığına geri koyar; çok büyükse ya da yığının toplam sınırı
// aşılacaksa bırakır (çöp toplayıcıya kalır).
func (y *bufStack) Put(p *[]float32) {
	c := cap(*p)
	if c > y.largest {
		return
	}
	y.mu.Lock()
	if y.total+c <= bufTotalMax {
		y.t = append(y.t, p)
		y.total += c
	}
	y.mu.Unlock()
}

// packBPanel op(B)'nin jp. nr genişlikli panelini dst'ye (K×nr) yazar.
// op(B)(k, j) = b[k*bks + j*bjs]. N'yi aşan sütunlar sıfırla doldurulur.
func packBPanel(b []float32, bks, bjs, K, N, jp int, dst []float32) {
	j0 := jp * nr
	nn := N - j0
	switch {
	case nn >= nr && bjs == 1:
		for k := 0; k < K; k++ {
			o := k*bks + j0
			copy(dst[k*nr:k*nr+nr], b[o:o+nr])
		}
	case nn >= nr && bks == 1:
		// Devrik B: her sütun bellekte bitişik.
		for c := 0; c < nr; c++ {
			o := (j0 + c) * bjs
			for k, v := range b[o : o+K] {
				dst[k*nr+c] = v
			}
		}
	default:
		for k := 0; k < K; k++ {
			for c := 0; c < nr; c++ {
				if c < nn {
					dst[k*nr+c] = b[k*bks+(j0+c)*bjs]
				} else {
					dst[k*nr+c] = 0
				}
			}
		}
	}
}

// packA op(A)'nın [i0, i0+m) satırlarını ve [p0, p0+kb) sütunlarını mr
// yükseklikli panellere yazar: panel ip içinde (k, r) elemanı
// ap[ip*kb*mr + k*mr + r]'dedir. op(A)(i, k) = a[i*ais + k*aks]. m'yi aşan
// satırlar sıfırla doldurulur.
func packA(a []float32, ais, axisN, i0, m, p0, kb int, ap []float32) {
	for ip := 0; ip*mr < m; ip++ {
		pan := ap[ip*kb*mr : (ip+1)*kb*mr]
		rows := m - ip*mr
		ib := i0 + ip*mr
		switch {
		case rows >= mr && axisN == 1:
			for r := 0; r < mr; r++ {
				o := (ib+r)*ais + p0
				for k, v := range a[o : o+kb] {
					pan[k*mr+r] = v
				}
			}
		case rows >= mr && ais == 1:
			// Devrik A: ardışık satırlar bellekte yan yana.
			for k := 0; k < kb; k++ {
				o := (p0+k)*axisN + ib
				copy(pan[k*mr:k*mr+mr], a[o:o+mr])
			}
		default:
			for k := 0; k < kb; k++ {
				for r := 0; r < mr; r++ {
					if r < rows {
						pan[k*mr+r] = a[(ib+r)*ais+(p0+k)*axisN]
					} else {
						pan[k*mr+r] = 0
					}
				}
			}
		}
	}
}

// gemm toplu çarpım: her b için C_b += op(A_b)·op(B_b). a, b, c ilgili toplu
// elemanın başından başlayan dilimlerdir; toplu adımları M*K, K*N, M*N'dir.
//
// B bütün toplu için birlikte paketlenir; paket tamponEnBuyukB elemanı aşacaksa toplu
// parçalara bölünür. Böylece paketleme tamponu sınırlı kalır ve havuzdan gelir (önce
// 64×(1024×64) gibi toplu çarpımlarda her çağrıda ~16 MB yeni ayırma oluyordu; havuz
// yalnızca 4 MB'a kadar tutuyordu). Parçalar paralelliği korumak için olabildiğince büyük
// seçilir. Her toplu elemanın hesabı parçalamadan bağımsızdır (sonuç bit düzeyinde aynı).
func gemm(batch, M, N, K int, a []float32, transA bool, b []float32, transB bool, c []float32) {
	if batch == 0 || M == 0 || N == 0 || K == 0 {
		return
	}
	bpLen := (N + nr - 1) / nr * K * nr
	part := batch
	if batch*bpLen > bufMaxB {
		part = max(1, bufMaxB/bpLen)
	}
	for b0 := 0; b0 < batch; b0 += part {
		n := min(part, batch-b0)
		gemmParca(n, M, N, K, a[b0*M*K:(b0+n)*M*K], transA, b[b0*K*N:(b0+n)*K*N], transB, c[b0*M*N:(b0+n)*M*N])
	}
}

func gemmParca(batch, M, N, K int, a []float32, transA bool, b []float32, transB bool, c []float32) {
	ais, axisN := K, 1
	if transA {
		ais, axisN = 1, M
	}
	bks, bjs := N, 1
	if transB {
		bks, bjs = 1, K
	}
	np := (N + nr - 1) / nr
	bpLen := np * K * nr
	bpBuf := takePack(&packPoolB, batch*bpLen)
	defer packPoolB.Put(bpBuf)
	bp := *bpBuf
	// B satır öncelikliyse ve az satırla çarpılacaksa tam paneller paketlenmeden okunur:
	// her B elemanı yalnızca M/mr kez kullanılırken paketleme onu bir kez okuyup yazar.
	unpackedB := directB && bjs == 1 && M <= directBMaxM
	full := N / nr // tam panel sayısı (yalnızca bunlar paketlenmeden okunabilir)
	first := 0
	if unpackedB {
		first = full
	}
	if packN := np - first; packN > 0 {
		parallelFor(batch*packN, K*nr, func(lo, hi int) {
			for t := lo; t < hi; t++ {
				bi, jp := t/packN, first+t%packN
				packBPanel(b[bi*K*N:(bi+1)*K*N], bks, bjs, K, N, jp, bp[bi*bpLen+jp*K*nr:bi*bpLen+(jp+1)*K*nr])
			}
		})
	}

	is := batch * M * N * K
	W := workerCount(is, minPerWorker, gemmBigJob)
	mPad := (M + mr - 1) / mr * mr
	rb := mPad // iş parçası başına satır
	npb := np  // iş parçası başına B paneli (sütun bloğu)
	if is < singleCoreThreshold || W == 1 {
		W = 1
	} else {
		// Başarım ve verimlilik çekirdekleri karışık işlemcilerde yük dengesi
		// belirleyici olduğundan işçi başına ~8 iş parçası hedeflenir; satır
		// bloğu bu yüzden küçük tutulur. İş yine az kalırsa (küçük M) sütun
		// yönünde de bölünür.
		rb = (batch*M + W*8 - 1) / (W * 8)
		rb = (rb + mr - 1) / mr * mr
		if rb > rowB {
			rb = rowB
		}
		if rb > mPad {
			rb = mPad
		}
		rowJob := batch * ((M + rb - 1) / rb)
		if target := W * 4; rowJob < target {
			divFn := (target + rowJob - 1) / rowJob
			npb = (np + divFn - 1) / divFn
			if npb < minPanel {
				npb = minPanel
			}
			if npb > np {
				npb = np
			}
		}
	}
	tpr := (M + rb - 1) / rb    // toplu eleman başına satır bloğu
	tpc := (np + npb - 1) / npb // sütun bloğu sayısı
	nt := batch * tpr * tpc
	if W > nt {
		W = nt
	}
	kcMax := kcBlock
	if K < kcMax {
		kcMax = K
	}

	// A paneli çok sayıda B paneliyle kullanılacaksa (geniş N) paketlemek,
	// adımlı okumadan biraz daha hızlıdır (ölçüm: N=256'da %10).
	direct := directA && np < 12
	run := func(t int, ap []float32) {
		bi := t / (tpr * tpc)
		r := t % (tpr * tpc)
		i0, jp0 := (r/tpc)*rb, (r%tpc)*npb
		jp1 := jp0 + npb
		if jp1 > np {
			jp1 = np
		}
		m := M - i0
		if m > rb {
			m = rb
		}
		aB := a[bi*M*K : (bi+1)*M*K]
		cB := c[bi*M*N : (bi+1)*M*N]
		bB := bp[bi*bpLen : (bi+1)*bpLen]
		for p0 := 0; p0 < K; p0 += kcBlock {
			kb := K - p0
			if kb > kcBlock {
				kb = kcBlock
			}
			whole := 0 // doğrudan okunacak tam A paneli sayısı
			if direct {
				whole = m / mr
				if m > whole*mr {
					// Yalnızca kenar paneli (sıfır dolgulu) paketlenir.
					packA(aB, ais, axisN, i0+whole*mr, m-whole*mr, p0, kb, ap)
				}
			} else {
				packA(aB, ais, axisN, i0, m, p0, kb, ap)
			}
			for jp := jp0; jp < jp1; jp++ {
				j0 := jp * nr
				nn := N - j0
				if nn > nr {
					nn = nr
				}
				bpan := bB[jp*K*nr+p0*nr : jp*K*nr+(p0+kb)*nr]
				if unpackedB && jp < full {
					bpan = b[bi*K*N+p0*N+j0:]
				}
				for ip := 0; ip*mr < m; ip++ {
					mm := m - ip*mr
					if mm > mr {
						mm = mr
					}
					cBlock := cB[(i0+ip*mr)*N+j0:]
					if unpackedB && jp < full {
						switch {
						case ip < whole:
							microAB(kb, aB[(i0+ip*mr)*ais+p0*axisN:], ais, axisN, bpan, N, cBlock, N, mm, nn)
						case direct:
							microAB(kb, ap[:kb*mr], 1, mr, bpan, N, cBlock, N, mm, nn)
						default:
							microAB(kb, ap[ip*kb*mr:(ip+1)*kb*mr], 1, mr, bpan, N, cBlock, N, mm, nn)
						}
						continue
					}
					switch {
					case ip < whole:
						microA(kb, aB[(i0+ip*mr)*ais+p0*axisN:], ais, axisN, bpan, cBlock, N, mm, nn)
					case direct:
						microKernel(kb, ap[:kb*mr], bpan, cBlock, N, mm, nn)
					default:
						microKernel(kb, ap[ip*kb*mr:(ip+1)*kb*mr], bpan, cBlock, N, mm, nn)
					}
				}
			}
		}
	}

	apSize := rb * kcMax
	if W == 1 {
		apBuf := takePack(&packPoolA, apSize)
		for t := 0; t < nt; t++ {
			run(t, *apBuf)
		}
		packPoolA.Put(apBuf)
		return
	}
	// Her katılımcının kendi A paketleme tamponu (ilk parçasında alınır).
	buffers := make([]*[]float32, W)
	distribute(W, nt, func(k, t int) {
		if buffers[k] == nil {
			buffers[k] = takePack(&packPoolA, apSize)
		}
		run(t, *buffers[k])
	})
	for _, p := range buffers {
		if p != nil {
			packPoolA.Put(p)
		}
	}
}

func (cpuBackend) f32Gemm(batch, M, N, K int, a View, transA bool, b View, transB bool, c Storage) {
	if !a.IsContiguous() || !b.IsContiguous() {
		panic(fmt.Sprintf("tensor: Gemm bitişik girdiler ister (%v, %v)", a.Shape, b.Shape))
	}
	ad := cd(a.S)[a.Offset : a.Offset+batch*M*K]
	bd := cd(b.S)[b.Offset : b.Offset+batch*K*N]
	gemm(batch, M, N, K, ad, transA, bd, transB, cd(c)[:batch*M*N])
}
