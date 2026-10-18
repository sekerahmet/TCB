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

func (cpuBackend) f32LayerNorm(out Storage, x View, gamma, beta *View, D int, eps float32) (Storage, Storage) {
	xd := gatherCPU(x)
	gd, bd := optGather(gamma), optGather(beta)
	R := len(xd) / max(D, 1)
	mean := make([]float32, R)
	rstd := make([]float32, R)
	y := cd(out)
	parallelFor(R, D*2, func(lo, hi int) {
		for r := lo; r < hi; r++ {
			row := xd[r*D : (r+1)*D]
			var s float64
			for _, v := range row {
				s += float64(v)
			}
			mu := s / float64(D)
			var v2 float64
			for _, v := range row {
				d := float64(v) - mu
				v2 += d * d
			}
			rs := 1 / math.Sqrt(v2/float64(D)+float64(eps))
			m32, r32 := float32(mu), float32(rs)
			mean[r], rstd[r] = m32, r32
			yr := y[r*D : (r+1)*D]
			for j, v := range row {
				h := (v - m32) * r32
				if gd != nil {
					h *= gd[j]
				}
				if bd != nil {
					h += bd[j]
				}
				yr[j] = h
			}
		}
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
			for r := lo; r < hi; r++ {
				row := xd[r*D : (r+1)*D]
				gr := gr0[r*D : (r+1)*D]
				m, rs := md[r], rd[r]
				for j, v := range gr {
					if pg != nil {
						pg[j] += float64(v) * float64((row[j]-m)*rs)
					}
					if pb != nil {
						pb[j] += float64(v)
					}
				}
				if xg == nil {
					continue
				}
				// dx = rstd·(dx̂ - ort(dx̂) - x̂·ort(dx̂·x̂))
				var s1, s2 float64
				for j, v := range gr {
					dh := v
					if gd != nil {
						dh *= gd[j]
					}
					h := (row[j] - m) * rs
					s1 += float64(dh)
					s2 += float64(dh) * float64(h)
				}
				m1 := float32(s1 / float64(D))
				m2 := float32(s2 / float64(D))
				xr := xg[r*D : (r+1)*D]
				for j, v := range gr {
					dh := v
					if gd != nil {
						dh *= gd[j]
					}
					h := (row[j] - m) * rs
					xr[j] += rs * (dh - m1 - h*m2)
				}
			}
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
		for r := lo; r < hi; r++ {
			row := xd[r*D : (r+1)*D]
			var s float64
			for _, v := range row {
				s += float64(v) * float64(v)
			}
			ri := float32(1 / math.Sqrt(s/float64(D)+float64(eps)))
			rinv[r] = ri
			yr := y[r*D : (r+1)*D]
			for j, v := range row {
				h := v * ri
				if gd != nil {
					h *= gd[j]
				}
				yr[j] = h
			}
		}
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
			for r := lo; r < hi; r++ {
				row := xd[r*D : (r+1)*D]
				gr := gr0[r*D : (r+1)*D]
				ri := rd[r]
				if pg != nil {
					for j, v := range gr {
						pg[j] += float64(v) * float64(row[j]*ri)
					}
				}
				if xg == nil {
					continue
				}
				// dx = r·dn - x·r³·ort(dn·x),  dn = g·γ
				var s float64
				for j, v := range gr {
					dn := v
					if gd != nil {
						dn *= gd[j]
					}
					s += float64(dn) * float64(row[j])
				}
				k := float32(s/float64(D)) * ri * ri * ri
				xr := xg[r*D : (r+1)*D]
				for j, v := range gr {
					dn := v
					if gd != nil {
						dn *= gd[j]
					}
					xr[j] += ri*dn - row[j]*k
				}
			}
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
