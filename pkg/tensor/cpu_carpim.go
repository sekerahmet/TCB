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
//     her k adımında mr+nr yükleme ile mr·nr çarpma-toplama yapar.
//   - İş parçaları GOMAXPROCS kadar işçiye atomik bir sayaçla dağıtılır.
const (
	mr     = 4   // mikro çekirdek satır sayısı
	nr     = 2   // mikro çekirdek sütun sayısı
	kcBlok = 256 // K ekseni blok derinliği
	satirB = 64  // bir iş parçasındaki en fazla satır sayısı
	// enAzPanel sütun yönünde bölünürken bir iş parçasına düşen en az B paneli.
	enAzPanel = 16
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
	if nn >= nr && bjs == 1 {
		for k := 0; k < K; k++ {
			o := k*bks + j0
			src := b[o : o+nr : o+nr]
			d := dst[k*nr : k*nr+nr : k*nr+nr]
			d[0], d[1] = src[0], src[1]
		}
		return
	}
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

// packA op(A)'nın [i0, i0+m) satırlarını ve [p0, p0+kb) sütunlarını mr
// yükseklikli panellere yazar: panel ip içinde (k, r) elemanı
// ap[ip*kb*mr + k*mr + r]'dedir. op(A)(i, k) = a[i*ais + k*aks].
func packA(a []float32, ais, aks, i0, m, p0, kb int, ap []float32) {
	for ip := 0; ip*mr < m; ip++ {
		pan := ap[ip*kb*mr : (ip+1)*kb*mr]
		rows := m - ip*mr
		ib := i0 + ip*mr
		switch {
		case rows >= mr && aks == 1:
			r0 := a[ib*ais+p0:]
			r1 := a[(ib+1)*ais+p0:]
			r2 := a[(ib+2)*ais+p0:]
			r3 := a[(ib+3)*ais+p0:]
			r0, r1, r2, r3 = r0[:kb], r1[:kb], r2[:kb], r3[:kb]
			for k := 0; k < kb; k++ {
				d := pan[k*mr : k*mr+mr : k*mr+mr]
				d[0], d[1], d[2], d[3] = r0[k], r1[k], r2[k], r3[k]
			}
		case rows >= mr && ais == 1:
			// Devrik A: ardışık satırlar bellekte yan yana.
			for k := 0; k < kb; k++ {
				o := (p0+k)*aks + ib
				s := a[o : o+mr : o+mr]
				d := pan[k*mr : k*mr+mr : k*mr+mr]
				d[0], d[1], d[2], d[3] = s[0], s[1], s[2], s[3]
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

// microKernel C[0:mm, 0:nn] += Ap·Bp (Ap: kb×mr, Bp: kb×nr paketli).
// 4×2 boyutu, 8 biriktirici ve yükleme yazmaçlarının yazmaç dosyasına
// taşmadan sığması için seçilmiştir (daha büyük bloklar yığına taşma
// nedeniyle ölçümlerde daha yavaştı).
func microKernel(kb int, ap, bp []float32, c []float32, ldc, mm, nn int) {
	var c00, c01, c10, c11, c20, c21, c30, c31 float32
	ap = ap[:kb*mr]
	bp = bp[:kb*nr]
	for len(ap) >= 4 && len(bp) >= 2 {
		b0, b1 := bp[0], bp[1]
		a0 := ap[0]
		c00 += a0 * b0
		c01 += a0 * b1
		a1 := ap[1]
		c10 += a1 * b0
		c11 += a1 * b1
		a2 := ap[2]
		c20 += a2 * b0
		c21 += a2 * b1
		a3 := ap[3]
		c30 += a3 * b0
		c31 += a3 * b1
		ap = ap[4:]
		bp = bp[2:]
	}
	if mm == mr && nn == nr {
		r := c[0:2:2]
		r[0] += c00
		r[1] += c01
		r = c[ldc : ldc+2 : ldc+2]
		r[0] += c10
		r[1] += c11
		r = c[2*ldc : 2*ldc+2 : 2*ldc+2]
		r[0] += c20
		r[1] += c21
		r = c[3*ldc : 3*ldc+2 : 3*ldc+2]
		r[0] += c30
		r[1] += c31
		return
	}
	t := [mr][nr]float32{{c00, c01}, {c10, c11}, {c20, c21}, {c30, c31}}
	for i := 0; i < mm; i++ {
		for j := 0; j < nn; j++ {
			c[i*ldc+j] += t[i][j]
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
			packA(aB, ais, aks, i0, m, p0, kb, ap)
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
					microKernel(kb, ap[ip*kb*mr:(ip+1)*kb*mr], bpan, cB[(i0+ip*mr)*N+j0:], N, mm, nn)
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
