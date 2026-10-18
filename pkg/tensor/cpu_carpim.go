package tensor

import (
	"fmt"
	"sync"
	"sync/atomic"
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
//     ve boyutu derlemeye göre seçilir: GOAMD64=v3'te AVX2/FMA ile 6×16
//     (cpu_mikro_avx2.go), diğer derlemelerde taşınabilir Go ile 4×2
//     (cpu_mikro_genel.go).
//   - İş parçaları GOMAXPROCS kadar işçiye atomik bir sayaçla dağıtılır.
const (
	kcBlok = 256     // K ekseni blok derinliği
	satirB = 16 * mr // bir iş parçasındaki en fazla satır sayısı (mr katı)
	// enAzPanel sütun yönünde bölünürken bir iş parçasına düşen en az B paneli
	// (en az 32 sütun).
	enAzPanel = (32 + nr - 1) / nr
	// tekCekirdekEsik bu kadar çarpma-toplamadan küçük işler tek çekirdekte yapılır.
	tekCekirdekEsik = 1 << 18
	// isciBasinaEnAz bir işçiye düşen en az çarpma-toplama; işçi sayısı buna
	// göre sınırlanır (eşiğin hemen üstündeki işlerde gereksiz goroutine olmasın).
	isciBasinaEnAz = 1 << 17
)

// Paketleme tamponları çağrılar arasında yeniden kullanılır (eğitimde aynı
// boyutlar her adımda yinelenir). Paketleme tamponun gereken her elemanını
// yazdığından sıfırlama gerekmez. A ve B tamponları boyutları çok farklı
// olduğu için ayrı havuzlardadır.
var (
	paketHavuzA sync.Pool // *[]float32
	paketHavuzB sync.Pool // *[]float32
)

// paketAl havuzdan en az n elemanlı bir tampon alır (yetmezse yenisini ayırır).
func paketAl(h *sync.Pool, n int) *[]float32 {
	if p, ok := h.Get().(*[]float32); ok {
		if cap(*p) >= n {
			*p = (*p)[:n]
			return p
		}
		h.Put(p)
	}
	s := make([]float32, n)
	return &s
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
func packA(a []float32, ais, aks, i0, m, p0, kb int, ap []float32) {
	for ip := 0; ip*mr < m; ip++ {
		pan := ap[ip*kb*mr : (ip+1)*kb*mr]
		rows := m - ip*mr
		ib := i0 + ip*mr
		switch {
		case rows >= mr && aks == 1:
			for r := 0; r < mr; r++ {
				o := (ib+r)*ais + p0
				for k, v := range a[o : o+kb] {
					pan[k*mr+r] = v
				}
			}
		case rows >= mr && ais == 1:
			// Devrik A: ardışık satırlar bellekte yan yana.
			for k := 0; k < kb; k++ {
				o := (p0+k)*aks + ib
				copy(pan[k*mr:k*mr+mr], a[o:o+mr])
			}
		default:
			for k := 0; k < kb; k++ {
				for r := 0; r < mr; r++ {
					if r < rows {
						pan[k*mr+r] = a[(ib+r)*ais+(p0+k)*aks]
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
func gemm(batch, M, N, K int, a []float32, transA bool, b []float32, transB bool, c []float32) {
	if batch == 0 || M == 0 || N == 0 || K == 0 {
		return
	}
	ais, aks := K, 1
	if transA {
		ais, aks = 1, M
	}
	bks, bjs := N, 1
	if transB {
		bks, bjs = 1, K
	}
	np := (N + nr - 1) / nr
	bpLen := np * K * nr
	bpTampon := paketAl(&paketHavuzB, batch*bpLen)
	defer paketHavuzB.Put(bpTampon)
	bp := *bpTampon
	parallelFor(batch*np, K*nr, func(lo, hi int) {
		for t := lo; t < hi; t++ {
			bi, jp := t/np, t%np
			packBPanel(b[bi*K*N:(bi+1)*K*N], bks, bjs, K, N, jp, bp[bi*bpLen+jp*K*nr:bi*bpLen+(jp+1)*K*nr])
		}
	})

	W := workers()
	is := batch * M * N * K
	if lim := is / isciBasinaEnAz; W > lim {
		W = max(lim, 1)
	}
	mPad := (M + mr - 1) / mr * mr
	rb := mPad // iş parçası başına satır
	npb := np  // iş parçası başına B paneli (sütun bloğu)
	if is < tekCekirdekEsik || W == 1 {
		W = 1
	} else {
		// Başarım ve verimlilik çekirdekleri karışık işlemcilerde yük dengesi
		// belirleyici olduğundan işçi başına ~8 iş parçası hedeflenir; satır
		// bloğu bu yüzden küçük tutulur. İş yine az kalırsa (küçük M) sütun
		// yönünde de bölünür.
		rb = (batch*M + W*8 - 1) / (W * 8)
		rb = (rb + mr - 1) / mr * mr
		if rb > satirB {
			rb = satirB
		}
		if rb > mPad {
			rb = mPad
		}
		satirIs := batch * ((M + rb - 1) / rb)
		if hedef := W * 4; satirIs < hedef {
			bol := (hedef + satirIs - 1) / satirIs
			npb = (np + bol - 1) / bol
			if npb < enAzPanel {
				npb = enAzPanel
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
	kcMax := kcBlok
	if K < kcMax {
		kcMax = K
	}

	calis := func(t int, ap []float32) {
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
		for p0 := 0; p0 < K; p0 += kcBlok {
			kb := K - p0
			if kb > kcBlok {
				kb = kcBlok
			}
			tam := 0 // doğrudan okunacak tam A paneli sayısı
			if dogrudanA {
				tam = m / mr
				if m > tam*mr {
					// Yalnızca kenar paneli (sıfır dolgulu) paketlenir.
					packA(aB, ais, aks, i0+tam*mr, m-tam*mr, p0, kb, ap)
				}
			} else {
				packA(aB, ais, aks, i0, m, p0, kb, ap)
			}
			for jp := jp0; jp < jp1; jp++ {
				j0 := jp * nr
				nn := N - j0
				if nn > nr {
					nn = nr
				}
				bpan := bB[jp*K*nr+p0*nr : jp*K*nr+(p0+kb)*nr]
				for ip := 0; ip*mr < m; ip++ {
					mm := m - ip*mr
					if mm > mr {
						mm = mr
					}
					cBlok := cB[(i0+ip*mr)*N+j0:]
					switch {
					case ip < tam:
						mikroA(kb, aB[(i0+ip*mr)*ais+p0*aks:], ais, aks, bpan, cBlok, N, mm, nn)
					case dogrudanA:
						microKernel(kb, ap[:kb*mr], bpan, cBlok, N, mm, nn)
					default:
						microKernel(kb, ap[ip*kb*mr:(ip+1)*kb*mr], bpan, cBlok, N, mm, nn)
					}
				}
			}
		}
	}

	apBoy := rb * kcMax
	if W == 1 {
		apTampon := paketAl(&paketHavuzA, apBoy)
		for t := 0; t < nt; t++ {
			calis(t, *apTampon)
		}
		paketHavuzA.Put(apTampon)
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	wg.Add(W)
	for w := 0; w < W; w++ {
		go func() {
			defer wg.Done()
			apTampon := paketAl(&paketHavuzA, apBoy)
			defer paketHavuzA.Put(apTampon)
			ap := *apTampon
			for {
				t := int(next.Add(1) - 1)
				if t >= nt {
					return
				}
				calis(t, ap)
			}
		}()
	}
	wg.Wait()
}

func (cpuBackend) Gemm(batch, M, N, K int, a View, transA bool, b View, transB bool, c Storage) {
	if !a.IsContiguous() || !b.IsContiguous() {
		panic(fmt.Sprintf("tensor: Gemm bitişik girdiler ister (%v, %v)", a.Shape, b.Shape))
	}
	ad := cd(a.S)[a.Offset : a.Offset+batch*M*K]
	bd := cd(b.S)[b.Offset : b.Offset+batch*K*N]
	gemm(batch, M, N, K, ad, transA, bd, transB, cd(c)[:batch*M*N])
}
