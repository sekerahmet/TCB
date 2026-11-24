package gpu

// mma.sync s8'li çok satırlı çarpım (15.53a, sm_80+): blok 4 warp, warp 32 × 32 çıktı; ortak düzen
// kernel_quant_act.go'da.

// actMM çok satırlı çarpımın ortak kuruluşudur: şeridin satırları ve sütunları.
type actMM struct {
	k               *emitter
	pq, pd, pbs, pw reg
	pc, pfin        reg
	M, N, nb, K     reg
	rowBytes        reg
	g, t            reg
	rows            [2][2]reg // [mt][üst/alt]: g ve g+8 (M−1'e kırpılmış)
	rowOK           [2][2]reg // satır M'nin içinde ve sonlu mu
	colB            [4]reg    // B parçasının sütunu (g), N−1'e kırpılmış
	colC            [4][2]reg // C'nin sütunları (2t, 2t+1), N−1'e kırpılmış
	colOK           [4][2]reg
}

func newActMM(name string) *actMM {
	k := newEmitter(name)
	k.target = 80
	a := &actMM{k: k}
	a.pq, a.pd, a.pbs, a.pw = k.pointer("q"), k.pointer("d"), k.pointer("bs"), k.pointer("w")
	a.pc, a.pfin = k.written("c"), k.pointer("fin")
	a.M, a.N, a.nb = k.u32p("m"), k.u32p("n"), k.u32p("bloklar")
	a.K = k.r()
	k.e("shl.b32", a.K, a.nb, 8)
	tid := k.special("%tid.x")
	lane, warp := k.r(), k.r()
	k.e("and.b32", lane, tid, 31)
	k.e("shr.u32", warp, tid, 5)
	a.g, a.t = k.r(), k.r()
	k.e("shr.u32", a.g, lane, 2)
	k.e("and.b32", a.t, lane, 3)
	wm, wn := k.r(), k.r()
	k.e("shr.u32", wm, warp, 1)
	k.e("and.b32", wn, warp, 1)
	row0, col0 := k.r(), k.r()
	k.e("mul.lo.s32", row0, k.special("%ctaid.y"), actTile)
	k.e("mad.lo.s32", row0, wm, 32, row0)
	k.e("mul.lo.s32", col0, k.special("%ctaid.x"), actTile)
	k.e("mad.lo.s32", col0, wn, 32, col0)
	last := func(n reg) reg {
		r := k.r()
		k.e("sub.s32", r, n, 1)
		return r
	}
	mLast, nLast := last(a.M), last(a.N)
	for mt := range 2 {
		for h := range 2 {
			r := k.r()
			k.e("add.s32", r, row0, mt*16+h*8)
			k.e("mad.lo.s32", r, a.g, 1, r)
			a.rowOK[mt][h] = k.p()
			k.e("setp.lt.u32", a.rowOK[mt][h], r, a.M)
			k.e("min.u32", r, r, mLast)
			a.rows[mt][h] = r
			// Sonlu olmayan satır float yoluyla yazılır (kernel_quant_act_float.go): burada atlanır.
			f, finite := k.r(), k.p()
			k.e("ld.global.u32", f, mem(k.address(a.pfin, r, 4), 0))
			k.e("setp.eq.u32", finite, f, 0)
			k.e("and.pred", a.rowOK[mt][h], a.rowOK[mt][h], finite)
		}
	}
	for nt := range 4 {
		r := k.r()
		k.e("add.s32", r, col0, nt*8)
		k.e("add.s32", r, r, a.g)
		k.e("min.u32", r, r, nLast)
		a.colB[nt] = r
		for j := range 2 {
			c := k.r()
			k.e("add.s32", c, col0, nt*8+j)
			k.e("mad.lo.s32", c, a.t, 2, c)
			a.colOK[nt][j] = k.p()
			k.e("setp.lt.u32", a.colOK[nt][j], c, a.N)
			k.e("min.u32", c, c, nLast)
			a.colC[nt][j] = c
		}
	}
	return a
}

// blockAddr sütunun i. ağırlık bloğunun adresidir.
func (a *actMM) blockAddr(col, i reg, size int) reg {
	k := a.k
	r := k.rd()
	k.e("mul.wide.u32", r, col, a.rowBytes)
	k.e("add.s64", r, r, a.pw)
	o := k.rd()
	k.e("mul.wide.u32", o, i, size)
	k.e("add.s64", r, r, o)
	return r
}

// actAddr satırın i. bloğunun kodlarının adresidir (+ ofset bayt).
func (a *actMM) actAddr(row, i reg) reg {
	k := a.k
	at := k.r()
	k.e("mul.lo.s32", at, row, a.K)
	k.e("mad.lo.s32", at, i, actBlk, at)
	k.e("mad.lo.s32", at, a.t, 4, at)
	return k.address(a.pq, at, 1)
}

// mmaS8 d = a·b + c'dir (int8 girdi, int32 birikim): k32 için a dört, b iki; k16 için a iki, b bir yazmaç.
func (k *emitter) mmaS8(d [4]reg, a []reg, b []reg, c [4]reg) {
	shape := "m16n8k32"
	if len(a) == 2 {
		shape = "m16n8k16"
	}
	k.e("mma.sync.aligned."+shape+".row.col.s32.s8.s8.s32", vlist(d[:]), vlist(a), vlist(b), vlist(c[:]))
}

// store c'ye toplamı ekler: c[row, col] = c + acc (CPU'nun c += dot'u; sıfırla başlayan c'de −0 da aynı).
func (a *actMM) store(acc [2][4][4]reg) {
	k := a.k
	for mt := range 2 {
		for nt := range 4 {
			for e := range 4 {
				h, j := e/2, e%2
				skip := k.label()
				p := k.p()
				k.e("and.pred", p, a.rowOK[mt][h], a.colOK[nt][j])
				k.jumpUnless(p, skip)
				idx := k.r()
				k.e("mad.lo.s32", idx, a.rows[mt][h], a.N, a.colC[nt][j])
				ad := k.address(a.pc, idx, 4)
				old := k.f()
				k.e("ld.global.f32", old, mem(ad, 0))
				k.e("add.rn.f32", old, old, acc[mt][nt][e])
				k.e("st.global.f32", mem(ad, 0), old)
				k.put(skip)
			}
		}
	}
}

// zeros4 sıfır int32 dörtlüsüdür.
func (k *emitter) zeros4() [4]reg { return [4]reg{k.whole(0), k.whole(0), k.whole(0), k.whole(0)} }

// mmAcc warp'ın 32 × 32 çıktısıdır: [mt][nt][c parçası].
type mmAcc [2][4][4]reg

func (a *actMM) zeroAcc() (acc mmAcc) {
	for mt := range acc {
		for nt := range acc[mt] {
			for e := range acc[mt][nt] {
				acc[mt][nt][e] = a.k.constant(0)
			}
		}
	}
	return acc
}

func (a *actMM) zeroInts() (v mmAcc) {
	for mt := range v {
		for nt := range v[mt] {
			v[mt][nt] = a.k.zeros4()
		}
	}
	return v
}

// actAddrs şeridin satırlarının i. blok adresleri; d8s ölçekleri.
func (a *actMM) actAddrs(i reg) (aA [2][2]reg) {
	for mt := range 2 {
		for h := range 2 {
			aA[mt][h] = a.actAddr(a.rows[mt][h], i)
		}
	}
	return aA
}

func (a *actMM) d8s(i reg) (d8 [2][2]reg) {
	k := a.k
	for mt := range 2 {
		for h := range 2 {
			at := k.r()
			k.e("mul.lo.s32", at, a.rows[mt][h], a.nb)
			k.e("add.s32", at, at, i)
			d8[mt][h] = k.f()
			k.e("ld.global.f32", d8[mt][h], mem(k.address(a.pd, at, 4), 0))
		}
	}
	return d8
}

// quantActQ4KKernel Q4_K ağırlıklı çok satırlı çarpım: alt blok j (32 öğe) bir m16n8k32'dir;
// isum = Σ_j sc_j·s_j, msum = Σ_j m_j·(bs[2j] + bs[2j+1]); blokta t = d·isum − dmin·msum, acc += t·d8
// (dotQ4KQ8Go'nun sırası).
func quantActQ4KKernel() *emitter {
	a := newActMM("nicem_etk_mma_q4_k")
	k := a.k
	a.rowBytes = k.r()
	k.e("mul.lo.s32", a.rowBytes, a.nb, q4kBlockBytes)
	acc := a.zeroAcc()
	i := k.whole(0)
	k.whileLess("u32", i, a.nb, func() {
		dC, mC, scW, mW := a.q4kScales(i)
		isum := a.q4kDots(i, scW)
		a.q4kAdd(i, &isum, dC, mC, mW, &acc)
		k.e("add.s32", i, i, 1)
	})
	a.store(acc)
	return k
}

// q4kScales sütunların ölçekleridir: d, dmin ve paketli 6 bitlik sc/m (her sözcükte dört alt blok).
func (a *actMM) q4kScales(i reg) (dC, mC [4][2]reg, scW, mW [4][2][2]reg) {
	k := a.k
	for nt := range 4 {
		for j := range 2 {
			blk := a.blockAddr(a.colC[nt][j], i, q4kBlockBytes)
			dC[nt][j], mC[nt][j] = k.half(blk, 0), k.half(blk, 2)
			s := [3]reg{k.r(), k.r(), k.r()}
			for w := range s {
				k.e("ld.global.u32", s[w], mem(blk, 4+4*w))
			}
			lo, ml, hi, mh, t := k.r(), k.r(), k.r(), k.r(), k.r()
			k.e("and.b32", lo, s[0], 0x3F3F3F3F)
			k.e("and.b32", ml, s[1], 0x3F3F3F3F)
			k.e("and.b32", hi, s[2], 0x0F0F0F0F)
			k.e("shr.u32", t, s[0], 2)
			k.e("and.b32", t, t, 0x30303030)
			k.e("or.b32", hi, hi, t)
			k.e("shr.u32", mh, s[2], 4)
			k.e("and.b32", mh, mh, 0x0F0F0F0F)
			k.e("shr.u32", t, s[1], 2)
			k.e("and.b32", t, t, 0x30303030)
			k.e("or.b32", mh, mh, t)
			scW[nt][j] = [2]reg{lo, hi}
			mW[nt][j] = [2]reg{ml, mh}
		}
	}
	return dC, mC, scW, mW
}

// q4kDots isum = Σ_j sc_j·s_j'dir (alt blok j bir m16n8k32).
func (a *actMM) q4kDots(i reg, scW [4][2][2]reg) mmAcc {
	k := a.k
	isum := a.zeroInts()
	var wB [4]reg
	for nt := range 4 {
		wB[nt] = a.blockAddr(a.colB[nt], i, q4kBlockBytes)
	}
	aA := a.actAddrs(i)
	for g4 := range 4 {
		var bq [4][2][2]reg // [nt][alt/üst yarım bayt][b0/b1]
		for nt := range 4 {
			lo, hi := k.r(), k.r()
			// q[g4·32 + 4t] ve q[g4·32 + 16 + 4t]; şeridin 4t'si adrese eklenir.
			ad := k.rd()
			t4 := k.r()
			k.e("shl.b32", t4, a.t, 2)
			k.e("cvt.u64.u32", ad, t4)
			k.e("add.s64", ad, ad, wB[nt])
			k.e("ld.global.u32", lo, mem(ad, 16+32*g4))
			k.e("ld.global.u32", hi, mem(ad, 16+32*g4+16))
			for half := range 2 {
				for b, w := range [2]reg{lo, hi} {
					r := k.r()
					if half == 0 {
						k.e("and.b32", r, w, 0x0F0F0F0F)
					} else {
						k.e("shr.u32", r, w, 4)
						k.e("and.b32", r, r, 0x0F0F0F0F)
					}
					bq[nt][half][b] = r
				}
			}
		}
		for half := range 2 {
			j := 2*g4 + half
			var frag [2][4]reg
			for mt := range 2 {
				for h := range 2 {
					lo, hi := k.r(), k.r()
					k.e("ld.global.u32", lo, mem(aA[mt][h], 32*j))
					k.e("ld.global.u32", hi, mem(aA[mt][h], 32*j+16))
					frag[mt][h], frag[mt][2+h] = lo, hi
				}
			}
			var sc [4][2]reg
			for nt := range 4 {
				for jj := range 2 {
					sc[nt][jj] = k.byteU(scW[nt][jj][j/4], j%4)
				}
			}
			for mt := range 2 {
				for nt := range 4 {
					s := k.zeros4()
					k.mmaS8(s, frag[mt][:], bq[nt][half][:], s)
					for e := range 4 {
						k.e("mad.lo.s32", isum[mt][nt][e], sc[nt][e%2], s[e], isum[mt][nt][e])
					}
				}
			}
		}
	}
	return isum
}

// q4kAdd msum'u (bs'nin 16'lık çiftleri satır başına bir kez okunur) ve bloğun float katkısını ekler.
func (a *actMM) q4kAdd(i reg, isum *mmAcc, dC, mC [4][2]reg, mW [4][2][2]reg, acc *mmAcc) {
	k := a.k
	var bs [2][2][8]reg
	var d8 [2][2]reg
	for mt := range 2 {
		for h := range 2 {
			at := k.r()
			k.e("mul.lo.s32", at, a.rows[mt][h], a.nb)
			k.e("add.s32", at, at, i)
			d8[mt][h] = k.f()
			k.e("ld.global.f32", d8[mt][h], mem(k.address(a.pd, at, 4), 0))
			k.e("shl.b32", at, at, 4)
			ad := k.address(a.pbs, at, 4)
			for q := range 4 {
				v := []reg{k.r(), k.r(), k.r(), k.r()}
				k.e("ld.global.v4.u32", vlist(v), mem(ad, 16*q))
				for p := range 2 {
					s := k.r()
					k.e("add.s32", s, v[2*p], v[2*p+1])
					bs[mt][h][2*q+p] = s
				}
			}
		}
	}
	for nt := range 4 {
		for jj := range 2 {
			var m [8]reg
			for j := range 8 {
				m[j] = k.byteU(mW[nt][jj][j/4], j%4)
			}
			for mt := range 2 {
				for h := range 2 {
					e := 2*h + jj
					ms := k.whole(0)
					for j := range 8 {
						k.e("mad.lo.s32", ms, m[j], bs[mt][h][j], ms)
					}
					x, y, t := k.f(), k.f(), k.f()
					k.e("mul.rn.f32", x, dC[nt][jj], k.toFloat(isum[mt][nt][e]))
					k.e("mul.rn.f32", y, mC[nt][jj], k.toFloat(ms))
					k.e("sub.rn.f32", t, x, y)
					k.e("mul.rn.f32", t, t, d8[mt][h])
					k.e("add.rn.f32", acc[mt][nt][e], acc[mt][nt][e], t)
				}
			}
		}
	}
}

// quantActQ6KKernel Q6_K ağırlıklı çok satırlı çarpım: 16'lık alt blok s bir m16n8k16'dır, kod
// q6 − 32 (işaretli); I = Σ_s sc_s·Σ (q6 − 32)·q8 = isum − 32·msum (tamsayıda kesin); blokta
// t = d·I, acc += t·d8 (dotQ6KQ8Go'nun sırası).
func quantActQ6KKernel() *emitter {
	a := newActMM("nicem_etk_mma_q6_k")
	k := a.k
	a.rowBytes = k.r()
	k.e("mul.lo.s32", a.rowBytes, a.nb, q6kBlockBytes)
	acc := a.zeroAcc()
	i := k.whole(0)
	k.whileLess("u32", i, a.nb, func() {
		var dC [4][2]reg
		var scW [4][2][4]reg // 16 işaretli ölçek, dört sözcük
		for nt := range 4 {
			for j := range 2 {
				blk := a.blockAddr(a.colC[nt][j], i, q6kBlockBytes)
				dC[nt][j] = k.half(blk, 208)
				for w := range 4 {
					scW[nt][j][w] = k.words16(blk, 192+4*w)
				}
			}
		}
		isum := a.q6kDots(i, scW)
		d8 := a.d8s(i)
		for mt := range 2 {
			for nt := range 4 {
				for e := range 4 {
					t := k.f()
					k.e("mul.rn.f32", t, dC[nt][e%2], k.toFloat(isum[mt][nt][e]))
					k.e("mul.rn.f32", t, t, d8[mt][e/2])
					k.e("add.rn.f32", acc[mt][nt][e], acc[mt][nt][e], t)
				}
			}
		}
		k.e("add.s32", i, i, 1)
	})
	a.store(acc)
	return k
}

// q6kDots I = Σ_s sc_s·Σ (q6 − 32)·q8'dir (alt blok s bir m16n8k16).
func (a *actMM) q6kDots(i reg, scW [4][2][4]reg) mmAcc {
	k := a.k
	isum := a.zeroInts()
	var wB [4]reg
	t4 := k.r()
	k.e("shl.b32", t4, a.t, 2)
	for nt := range 4 {
		ad := k.rd()
		k.e("cvt.u64.u32", ad, t4)
		k.e("add.s64", ad, ad, a.blockAddr(a.colB[nt], i, q6kBlockBytes))
		wB[nt] = ad
	}
	aA := a.actAddrs(i)
	for n := range 2 {
		for half := range 2 {
			// l0 = half·16 + 4t: ql[n·64 + l0], ql[n·64 + 32 + l0], qh[n·32 + l0]; dört çeyrek.
			var ql0, ql1, qh [4]reg
			for nt := range 4 {
				ql0[nt] = k.words16(wB[nt], n*64+half*16)
				ql1[nt] = k.words16(wB[nt], n*64+32+half*16)
				qh[nt] = k.words16(wB[nt], 128+n*32+half*16)
			}
			for quarter := range 4 {
				s := n*8 + quarter*2 + half
				var frag [2][2]reg
				for mt := range 2 {
					for h := range 2 {
						r := k.r()
						k.e("ld.global.u32", r, mem(aA[mt][h], 16*s))
						frag[mt][h] = r
					}
				}
				var sc [4][2]reg
				for nt := range 4 {
					for jj := range 2 {
						sc[nt][jj] = k.byteS(scW[nt][jj][s/4], s%4)
					}
				}
				for nt := range 4 {
					src := ql0[nt]
					if quarter%2 == 1 {
						src = ql1[nt]
					}
					q, t := k.r(), k.r()
					k.e("shr.u32", q, src, 4*(quarter/2))
					k.e("and.b32", q, q, 0x0F0F0F0F)
					k.e("shr.u32", t, qh[nt], 2*quarter)
					k.e("and.b32", t, t, 0x03030303)
					k.e("shl.b32", t, t, 4)
					k.e("or.b32", q, q, t)
					// Bayt bayt q − 32: ((q | 0x80) − 0x20) ^ 0x80 ödünç almaz.
					k.e("or.b32", q, q, -2139062144)
					k.e("sub.s32", q, q, 0x20202020)
					k.e("xor.b32", q, q, -2139062144)
					for mt := range 2 {
						d := k.zeros4()
						k.mmaS8(d, frag[mt][:], []reg{q}, d)
						for e := range 4 {
							k.e("mad.lo.s32", isum[mt][nt][e], sc[nt][e%2], d[e], isum[mt][nt][e])
						}
					}
				}
			}
		}
	}
	return isum
}
