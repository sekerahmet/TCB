package tensor

import "math"

// CPU arka ucu: sinir ağı işlemleri ve iyileştirici adımları.

func (cpuBackend) f32Embedding(out Storage, w View, idx []int, D int) {
	wd := gatherCPU(w)
	od := cd(out)
	parallelFor(len(idx), D, func(lo, hi int) {
		for r := lo; r < hi; r++ {
			copy(od[r*D:(r+1)*D], wd[idx[r]*D:(idx[r]+1)*D])
		}
	})
}

func (cpuBackend) f32EmbeddingGrad(gw, g Storage, idx []int, D int) {
	wg, gd := cd(gw), cd(g)
	V := len(wg) / max(D, 1)
	// Tablo satırları işçilere bölünür; her işçi tüm indeksleri tarar ve yalnızca
	// kendi aralığındaki satırlara, indeks sırasıyla ekler (yinelenen indeksler
	// çakışmaz, sonuç belirlenimcidir).
	parallelFor(V, len(idx)*D/max(V, 1)+1, func(lo, hi int) {
		for r, k := range idx {
			if k < lo || k >= hi {
				continue
			}
			dst := wg[k*D : (k+1)*D]
			src := gd[r*D : (r+1)*D]
			for j, v := range src {
				dst[j] += v
			}
		}
	})
}

func (cpuBackend) f32CrossEntropy(x View, targets []int, C int) (Storage, Storage) {
	xd := gatherCPU(x)
	R := len(targets)
	lse := make([]float32, R)
	kayip := make([]float64, R)
	parallelFor(R, C*8, func(lo, hi int) {
		tmp := make([]float32, C)
		for r := lo; r < hi; r++ {
			t := targets[r]
			if t < 0 {
				continue
			}
			row := xd[r*C : (r+1)*C]
			m := math.Inf(-1)
			for _, v := range row {
				if float64(v) > m {
					m = float64(v)
				}
			}
			var s float64
			m32 := float32(m)
			for c, v := range row {
				tmp[c] = v - m32
			}
			expVek(tmp, tmp)
			s = toplam64(tmp)
			l := m + math.Log(s)
			lse[r] = float32(l)
			kayip[r] = l - float64(row[t])
		}
	})
	var toplam float64
	gecerli := 0
	for r, t := range targets {
		if t >= 0 {
			toplam += kayip[r]
			gecerli++
		}
	}
	ort := 0.0
	if gecerli > 0 {
		ort = toplam / float64(gecerli)
	}
	return &cpuStorage{d: []float32{float32(ort)}}, &cpuStorage{d: lse}
}

func (cpuBackend) f32CrossEntropyGrad(gx, g Storage, x View, lse Storage, targets []int, C int) {
	gecerli := 0
	for _, t := range targets {
		if t >= 0 {
			gecerli++
		}
	}
	if gecerli == 0 {
		return
	}
	xd := gatherCPU(x)
	pg, ld := cd(gx), cd(lse)
	k := float64(cd(g)[0]) / float64(gecerli)
	parallelFor(len(targets), C*4, func(lo, hi int) {
		tmp := make([]float32, C)
		for r := lo; r < hi; r++ {
			t := targets[r]
			if t < 0 {
				continue
			}
			row := xd[r*C : (r+1)*C]
			dst := pg[r*C : (r+1)*C]
			l := ld[r]
			k32 := float32(k)
			for c, v := range row {
				tmp[c] = v - l
			}
			expVek(tmp, tmp)
			for c, e := range tmp {
				dst[c] += k32 * e
			}
			dst[t] -= float32(k)
		}
	})
}

// parcaliToplam norm katmanlarının ölçek/sapma gradyanlarını belirlenimci
// biriktirir: satırlar uzunluğu yalnızca D'ye bağlı parçalara bölünür, her
// parçanın kısmi toplamı ayrı tutulur ve sonunda parça sırasıyla birleştirilir.
// Sonuç işçi sayısından ve zamanlamadan bağımsızdır (bit düzeyinde aynı).
type parcaliToplam struct {
	R, D, rpc, nc int
	g, b          []float64
}

func yeniParcaliToplam(R, D int, gVar, bVar bool) *parcaliToplam {
	rpc := max(1, indirgemeParca/max(D, 1))
	p := &parcaliToplam{R: R, D: D, rpc: rpc, nc: (R + rpc - 1) / rpc}
	if gVar {
		p.g = make([]float64, p.nc*D)
	}
	if bVar {
		p.b = make([]float64, p.nc*D)
	}
	return p
}

// aralik c. parçanın satır aralığı.
func (p *parcaliToplam) aralik(c int) (int, int) { return c * p.rpc, min((c+1)*p.rpc, p.R) }

// kismi c. parçanın kısmi toplam dilimleri (gerekmeyenler nil).
func (p *parcaliToplam) kismi(c int) (g, b []float64) {
	if p.g != nil {
		g = p.g[c*p.D : (c+1)*p.D]
	}
	if p.b != nil {
		b = p.b[c*p.D : (c+1)*p.D]
	}
	return
}

// birlestir kısmi toplamları parça sırasıyla toplayıp hedeflere ekler.
func (p *parcaliToplam) birlestir(gg, bg []float32) {
	topla := func(dst []float32, k []float64) {
		if dst == nil || k == nil {
			return
		}
		acc := make([]float64, p.D)
		for c := 0; c < p.nc; c++ {
			for j, v := range k[c*p.D : (c+1)*p.D] {
				acc[j] += v
			}
		}
		for j, v := range acc {
			dst[j] += float32(v)
		}
	}
	topla(gg, p.g)
	topla(bg, p.b)
}

func optGather(v *View) []float32 {
	if v == nil {
		return nil
	}
	return gatherCPU(*v)
}

// Katman ve RMS normu çekirdekleri. Satır işleri lnIleriBlok / lnGeriBlok'tadır
// (GOAMD64=v3'te AVX2 çekirdekleri, aksi halde aşağıdaki Go eşleri). Toplamlar
// float64'te, eleman konumuna göre 8 kulvarda (j mod 8) biriktirilir ve sabit
// sırayla birleştirilir; kaynaştırılmış çarpma-toplama kullanılmaz. Böylece
// vektör ve Go yolları bit düzeyinde aynı sonucu verir (testler: norm_test.go).

// topla8 8 kulvarlık float64 kısmi toplamları sabit sırayla birleştirir
// (AVX2 çekirdeklerindeki TOPLA8 ile aynı sıra).
func topla8(a *[8]float64) float64 {
	return ((a[0] + a[4]) + (a[2] + a[6])) + ((a[1] + a[5]) + (a[3] + a[7]))
}

// lnDenetle norm çekirdeklerinin dilim uzunluklarını denetler ve satır sayısını
// döner (D <= 0 ya da boş girdide 0). a, x ile aynı uzunlukta olmalıdır (ileri
// yönde çıktı, geri yönde gelen gradyan); g, b, pg, pb nil ya da en az D,
// mean nil ya da en az R, rstd en az R, xg nil ya da x uzunluğunda olmalıdır.
func lnDenetle(D int, x, a, g, b, mean, rstd, xg []float32, pg, pb []float64) int {
	if D <= 0 || len(x) == 0 {
		return 0
	}
	R := len(x) / D
	kisa := func(s int, n int) bool { return s != 0 && s < n }
	if R*D != len(x) || len(a) < R*D || kisa(len(g), D) || kisa(len(b), D) || kisa(len(pg), D) ||
		kisa(len(pb), D) || kisa(len(mean), R) || len(rstd) < R || kisa(len(xg), R*D) {
		panic("tensor: norm çekirdeğine uyumsuz dilim uzunlukları (iç hata)")
	}
	return R
}

// lnIleriBlokGo katman normunun (mean nil ise RMS normunun) taşınabilir ileri
// yönü; x'in her D'lik satırı için:
//
//	mu = Σx / D (RMS: 0),  v = Σ(x − mu)² / D,  rs = 1 / √(v + eps),
//	y  = ((x − mu)·rs)·g + b   (float32; g ve b nil olabilir).
//
// mean (varsa) ve rstd satır başına yazılır.
func lnIleriBlokGo(y, x, g, b, mean, rstd []float32, D int, eps float32) {
	R := len(x) / D
	dD := float64(D)
	for r := 0; r < R; r++ {
		row := x[r*D : (r+1)*D]
		var mu float64
		if mean != nil {
			var acc [8]float64
			for j, v := range row {
				acc[j&7] += float64(v)
			}
			mu = topla8(&acc) / dD
		}
		var acc [8]float64
		for j, v := range row {
			d := float64(v) - mu
			acc[j&7] += float64(d * d)
		}
		rs := 1 / math.Sqrt(topla8(&acc)/dD+float64(eps))
		m32, r32 := float32(mu), float32(rs)
		if mean != nil {
			mean[r] = m32
		}
		rstd[r] = r32
		yr := y[r*D : (r+1)*D]
		for j, v := range row {
			h := float32((v - m32) * r32)
			if g != nil {
				h = float32(h * g[j])
			}
			if b != nil {
				h += b[j]
			}
			yr[j] = h
		}
	}
}

// lnGeriBlokGo katman normunun (mean nil ise RMS normunun) taşınabilir geri
// yönü; h = (x − m)·rs, dh = gr·gam (gam nil ise gr) ile her satır için:
//
//	pg[j] += gr·h,  pb[j] += gr                     (float64; nil olabilir),
//	xg    += rs·((dh − m1) − h·m2),  m1 = Σdh / D (RMS: 0),  m2 = Σdh·h / D.
//
// xg nil ise yalnızca pg ve pb biriktirilir.
func lnGeriBlokGo(xg []float32, pg, pb []float64, x, gr, gam, mean, rstd []float32, D int) {
	R := len(x) / D
	dD := float64(D)
	for r := 0; r < R; r++ {
		row := x[r*D : (r+1)*D]
		g := gr[r*D : (r+1)*D]
		var m float32
		if mean != nil {
			m = mean[r]
		}
		rs := rstd[r]
		var a1, a2 [8]float64
		for j, v := range row {
			h := (v - m) * rs
			if pg != nil {
				pg[j] += float64(float64(g[j]) * float64(h))
			}
			if pb != nil {
				pb[j] += float64(g[j])
			}
			if xg != nil {
				dh := g[j]
				if gam != nil {
					dh *= gam[j]
				}
				a1[j&7] += float64(dh)
				a2[j&7] += float64(float64(dh) * float64(h))
			}
		}
		if xg == nil {
			continue
		}
		var m1 float32
		if mean != nil {
			m1 = float32(topla8(&a1) / dD)
		}
		m2 := float32(topla8(&a2) / dD)
		xr := xg[r*D : (r+1)*D]
		for j, v := range row {
			h := (v - m) * rs
			dh := g[j]
			if gam != nil {
				dh *= gam[j]
			}
			t := (dh - m1) - float32(h*m2)
			xr[j] += float32(t * rs)
		}
	}
}

func (cpuBackend) f32LayerNorm(out Storage, x View, gamma, beta *View, D int, eps float32) (Storage, Storage) {
	xd := gatherCPU(x)
	gd, bd := optGather(gamma), optGather(beta)
	R := len(xd) / max(D, 1)
	mean := make([]float32, R)
	rstd := make([]float32, R)
	y := cd(out)
	parallelFor(R, D*2, func(lo, hi int) {
		lnIleriBlok(y[lo*D:hi*D], xd[lo*D:hi*D], gd, bd, mean[lo:hi], rstd[lo:hi], D, eps)
	})
	return &cpuStorage{d: mean}, &cpuStorage{d: rstd}
}

func (cpuBackend) f32LayerNormGrad(gx, ggamma, gbeta, g Storage, x View, gamma *View, mean, rstd Storage, D int) {
	xd := gatherCPU(x)
	gd := optGather(gamma)
	gr0, md, rd := cd(g), cd(mean), cd(rstd)
	xg, gg, bg := cd(gx), cd(ggamma), cd(gbeta)
	R := len(xd) / max(D, 1)
	pc := yeniParcaliToplam(R, D, gg != nil, bg != nil)
	parallelFor(pc.nc, pc.rpc*D*4, func(clo, chi int) {
		for c := clo; c < chi; c++ {
			lo, hi := pc.aralik(c)
			pg, pb := pc.kismi(c)
			var xr []float32
			if xg != nil {
				xr = xg[lo*D : hi*D]
			}
			lnGeriBlok(xr, pg, pb, xd[lo*D:hi*D], gr0[lo*D:hi*D], gd, md[lo:hi], rd[lo:hi], D)
		}
	})
	pc.birlestir(gg, bg)
}

func (cpuBackend) f32RMSNorm(out Storage, x View, gamma *View, D int, eps float32) Storage {
	xd := gatherCPU(x)
	gd := optGather(gamma)
	R := len(xd) / max(D, 1)
	rinv := make([]float32, R)
	y := cd(out)
	parallelFor(R, D*2, func(lo, hi int) {
		lnIleriBlok(y[lo*D:hi*D], xd[lo*D:hi*D], gd, nil, nil, rinv[lo:hi], D, eps)
	})
	return &cpuStorage{d: rinv}
}

func (cpuBackend) f32RMSNormGrad(gx, ggamma, g Storage, x View, gamma *View, rinv Storage, D int) {
	xd := gatherCPU(x)
	gd := optGather(gamma)
	gr0, rd := cd(g), cd(rinv)
	xg, gg := cd(gx), cd(ggamma)
	R := len(xd) / max(D, 1)
	pc := yeniParcaliToplam(R, D, gg != nil, false)
	parallelFor(pc.nc, pc.rpc*D*4, func(clo, chi int) {
		for c := clo; c < chi; c++ {
			lo, hi := pc.aralik(c)
			pg, _ := pc.kismi(c)
			var xr []float32
			if xg != nil {
				xr = xg[lo*D : hi*D]
			}
			lnGeriBlok(xr, pg, nil, xd[lo*D:hi*D], gr0[lo*D:hi*D], gd, nil, rd[lo:hi], D)
		}
	})
	pc.birlestir(gg, nil)
}

// ----------------------------------------------------------------------------
// İyileştirici adımları
// ----------------------------------------------------------------------------

func (cpuBackend) f32SGDStep(p, g, vel Storage, lr, momentum, wd float32) {
	d, gd, v := cd(p), cd(g), cd(vel)
	parallelFor(len(d), 4, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			gi := gd[i] + wd*d[i]
			if v != nil {
				v[i] = momentum*v[i] + gi
				gi = v[i]
			}
			d[i] -= lr * gi
		}
	})
}

func (cpuBackend) f32AdamWStep(p, g, m, v Storage, lr, b1, b2, eps, wd, bc1, bc2 float32) {
	d, gd, md, vd := cd(p), cd(g), cd(m), cd(v)
	adimLR := lr / bc1
	bc2s := sqrt32(bc2)
	cur := 1 - lr*wd
	parallelFor(len(d), 8, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			gi := gd[i]
			md[i] = b1*md[i] + (1-b1)*gi
			vd[i] = b2*vd[i] + (1-b2)*gi*gi
			d[i] = d[i]*cur - adimLR*md[i]/(sqrt32(vd[i])/bc2s+eps)
		}
	})
}

func (cpuBackend) f32SumSquares(s Storage) float64 {
	d := cd(s)
	return parallelSum(len(d), 2, func(lo, hi int) float64 {
		var t0, t1 float64
		p := d[lo:hi]
		for len(p) >= 2 {
			t0 += float64(p[0]) * float64(p[0])
			t1 += float64(p[1]) * float64(p[1])
			p = p[2:]
		}
		for _, v := range p {
			t0 += float64(v) * float64(v)
		}
		return t0 + t1
	})
}

func (cpuBackend) f32Scale(s Storage, k float32) {
	d := cd(s)
	parallelFor(len(d), 1, func(lo, hi int) {
		p := d[lo:hi]
		for i := range p {
			p[i] *= k
		}
	})
}
