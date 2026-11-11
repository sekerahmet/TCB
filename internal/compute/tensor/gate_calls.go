package tensor

import "runtime"

// Arka uç çağrılarının kapısı (access.go). Tensör paketi arka uca veri okuyan ya da yazan her
// çağrıyı buradaki gateX ile yapar: kilitsiz bir işlemin tuttuğu bir depoyla çakışan çağrı, o işlem
// bitene dek bekler. Yöntem başına yazılan parametreler elle sınıflandırıldı (çıktılar, gradyanlar,
// iyileştiricinin durumu); öbür depo ve görünüm parametreleri okunur. Yapısal test
// (gate_calls_test.go) paketin başka yerinden arka uca doğrudan veri çağrısı yapılmadığını sınar.
// Kapı depolarını çağrı bitene dek canlı tutar: öbek dışı blokta duran depoyu çekirdek dilimiyle
// okurken derleyici ölü sayabilir (host_block.go).

func gateAttentionGradSet(be AttentionGradSetter, gq Storage, gk Storage, gv Storage, g Storage, q View, k View, v View, mask Storage, o Storage, lse Storage, p AttentionParams, fresh [3]bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gq, gk, gv}, []Storage{g, q.S, k.S, v.S, mask, o, lse})
	}
	be.AttentionGradSet(gq, gk, gv, g, q, k, v, mask, o, lse, p, fresh)
	runtime.KeepAlive(gq)
	runtime.KeepAlive(gk)
	runtime.KeepAlive(gv)
	runtime.KeepAlive(g)
	runtime.KeepAlive(q.S)
	runtime.KeepAlive(k.S)
	runtime.KeepAlive(v.S)
	runtime.KeepAlive(mask)
	runtime.KeepAlive(o)
	runtime.KeepAlive(lse)
}

func gateAddTo(be Backend, dst Storage, src Storage, scale float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{src})
	}
	be.AddTo(dst, src, scale)
	runtime.KeepAlive(dst)
	runtime.KeepAlive(src)
}

func gateArgMax(be Backend, x View, axis int) []int {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{x.S})
	}
	r := be.ArgMax(x, axis)
	runtime.KeepAlive(x.S)
	return r
}

func gateAtan2Grad(be Backend, ga Storage, gb Storage, aShape []int, bShape []int, outShape []int, g Storage, a View, b View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{ga, gb}, []Storage{g, a.S, b.S})
	}
	be.Atan2Grad(ga, gb, aShape, bShape, outShape, g, a, b)
	runtime.KeepAlive(ga)
	runtime.KeepAlive(gb)
	runtime.KeepAlive(g)
	runtime.KeepAlive(a.S)
	runtime.KeepAlive(b.S)
}

func gateAttention(be Backend, out Storage, q View, k View, v View, mask Storage, p AttentionParams) (lse Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{q.S, k.S, v.S, mask})
	}
	lse = be.Attention(out, q, k, v, mask, p)
	runtime.KeepAlive(out)
	runtime.KeepAlive(q.S)
	runtime.KeepAlive(k.S)
	runtime.KeepAlive(v.S)
	runtime.KeepAlive(mask)
	return lse
}

func gateAttentionGrad(be Backend, gq Storage, gk Storage, gv Storage, g Storage, q View, k View, v View, mask Storage, o Storage, lse Storage, p AttentionParams) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gq, gk, gv}, []Storage{g, q.S, k.S, v.S, mask, o, lse})
	}
	be.AttentionGrad(gq, gk, gv, g, q, k, v, mask, o, lse, p)
	runtime.KeepAlive(gq)
	runtime.KeepAlive(gk)
	runtime.KeepAlive(gv)
	runtime.KeepAlive(g)
	runtime.KeepAlive(q.S)
	runtime.KeepAlive(k.S)
	runtime.KeepAlive(v.S)
	runtime.KeepAlive(mask)
	runtime.KeepAlive(o)
	runtime.KeepAlive(lse)
}

func gateBinary(be Backend, op BinOp, out Storage, outShape []int, a View, b View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{a.S, b.S})
	}
	be.Binary(op, out, outShape, a, b)
	runtime.KeepAlive(out)
	runtime.KeepAlive(a.S)
	runtime.KeepAlive(b.S)
}

func gateBinaryGrad(be Backend, mode GradMode, dst Storage, dstShape []int, outShape []int, g Storage, o View, y Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{g, o.S, y})
	}
	be.BinaryGrad(mode, dst, dstShape, outShape, g, o, y)
	runtime.KeepAlive(dst)
	runtime.KeepAlive(g)
	runtime.KeepAlive(o.S)
	runtime.KeepAlive(y)
}

func gateClamp(be Backend, out Storage, x View, lo float32, hi float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.Clamp(out, x, lo, hi)
	runtime.KeepAlive(out)
	runtime.KeepAlive(x.S)
}

func gateClampGrad(be Backend, gx Storage, g Storage, x View, lo float32, hi float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S})
	}
	be.ClampGrad(gx, g, x, lo, hi)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
}

func gateCopy(be Backend, dst Storage, dstOff int, src View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{src.S})
	}
	be.Copy(dst, dstOff, src)
	runtime.KeepAlive(dst)
	runtime.KeepAlive(src.S)
}

func gateCopyStrided(be Backend, dst View, src View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst.S}, []Storage{src.S})
	}
	be.CopyStrided(dst, src)
	runtime.KeepAlive(dst.S)
	runtime.KeepAlive(src.S)
}

func gateCrossEntropy(be Backend, x View, targets []int, C int) (loss Storage, lse Storage) {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{x.S})
	}
	loss, lse = be.CrossEntropy(x, targets, C)
	runtime.KeepAlive(x.S)
	return loss, lse
}

func gateCrossEntropyGrad(be Backend, gx Storage, g Storage, x View, lse Storage, targets []int, C int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S, lse})
	}
	be.CrossEntropyGrad(gx, g, x, lse, targets, C)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(lse)
}

func gateCumSum(be Backend, out Storage, x View, axis int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.CumSum(out, x, axis)
	runtime.KeepAlive(out)
	runtime.KeepAlive(x.S)
}

func gateCumSumGrad(be Backend, gx Storage, g Storage, shape []int, axis int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g})
	}
	be.CumSumGrad(gx, g, shape, axis)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g)
}

func gateEmbedding(be Backend, out Storage, w View, idx []int, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{w.S})
	}
	be.Embedding(out, w, idx, D)
	runtime.KeepAlive(out)
	runtime.KeepAlive(w.S)
}

func gateEmbeddingGrad(be Backend, gw Storage, g Storage, idx []int, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gw}, []Storage{g})
	}
	be.EmbeddingGrad(gw, g, idx, D)
	runtime.KeepAlive(gw)
	runtime.KeepAlive(g)
}

func gateExpandAdd(be Backend, dst Storage, dstShape []int, src Storage, srcShape []int, scale float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{src})
	}
	be.ExpandAdd(dst, dstShape, src, srcShape, scale)
	runtime.KeepAlive(dst)
	runtime.KeepAlive(src)
}

func gateFill(be Backend, dst Storage, v float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, nil)
	}
	be.Fill(dst, v)
	runtime.KeepAlive(dst)
}

func gateGatherGrad(be Backend, gx Storage, g Storage, idx Storage, shape []int, axis int, k int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, idx})
	}
	be.GatherGrad(gx, g, idx, shape, axis, k)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g)
	runtime.KeepAlive(idx)
}

func gateGemm(be Backend, batch int, M int, N int, K int, a View, transA bool, b View, transB bool, c Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{c}, []Storage{a.S, b.S})
	}
	be.Gemm(batch, M, N, K, a, transA, b, transB, c)
	runtime.KeepAlive(a.S)
	runtime.KeepAlive(b.S)
	runtime.KeepAlive(c)
}

func gateLayerNorm(be Backend, out Storage, x View, gamma *View, beta *View, D int, eps float32) (mean Storage, rstd Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S, viewStorage(gamma), viewStorage(beta)})
	}
	mean, rstd = be.LayerNorm(out, x, gamma, beta, D, eps)
	runtime.KeepAlive(out)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(gamma)
	runtime.KeepAlive(beta)
	return mean, rstd
}

func gateLayerNormGrad(be Backend, gx Storage, ggamma Storage, gbeta Storage, g Storage, x View, gamma *View, mean Storage, rstd Storage, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx, ggamma, gbeta}, []Storage{g, x.S, viewStorage(gamma), mean, rstd})
	}
	be.LayerNormGrad(gx, ggamma, gbeta, g, x, gamma, mean, rstd, D)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(ggamma)
	runtime.KeepAlive(gbeta)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(gamma)
	runtime.KeepAlive(mean)
	runtime.KeepAlive(rstd)
}

func gateMaxGrad(be Backend, gx Storage, g Storage, x View, axis int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S})
	}
	be.MaxGrad(gx, g, x, axis)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
}

func gateRMSNorm(be Backend, out Storage, x View, gamma *View, D int, eps float32) (rinv Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S, viewStorage(gamma)})
	}
	rinv = be.RMSNorm(out, x, gamma, D, eps)
	runtime.KeepAlive(out)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(gamma)
	return rinv
}

func gateRMSNormGrad(be Backend, gx Storage, ggamma Storage, g Storage, x View, gamma *View, rinv Storage, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx, ggamma}, []Storage{g, x.S, viewStorage(gamma), rinv})
	}
	be.RMSNormGrad(gx, ggamma, g, x, gamma, rinv, D)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(ggamma)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(gamma)
	runtime.KeepAlive(rinv)
}

func gateScale(be Backend, s Storage, k float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{s}, nil)
	}
	be.Scale(s, k)
	runtime.KeepAlive(s)
}

func gateScatterAdd(be Backend, dst Storage, dstOff int, dstStrides []int, src Storage, shape []int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{src})
	}
	be.ScatterAdd(dst, dstOff, dstStrides, src, shape)
	runtime.KeepAlive(dst)
	runtime.KeepAlive(src)
}

func gateSoftmax(be Backend, out Storage, x View, axis int, log bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.Softmax(out, x, axis, log)
	runtime.KeepAlive(out)
	runtime.KeepAlive(x.S)
}

func gateSoftmaxGrad(be Backend, gx Storage, g Storage, y Storage, shape []int, axis int, log bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, y})
	}
	be.SoftmaxGrad(gx, g, y, shape, axis, log)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g)
	runtime.KeepAlive(y)
}

func gateSort(be Backend, vals Storage, idx Storage, x View, axis int, k int, desc bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{vals, idx}, []Storage{x.S})
	}
	be.Sort(vals, idx, x, axis, k, desc)
	runtime.KeepAlive(vals)
	runtime.KeepAlive(idx)
	runtime.KeepAlive(x.S)
}

func gateSumSquares(be Backend, s Storage) float64 {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{s})
	}
	r := be.SumSquares(s)
	runtime.KeepAlive(s)
	return r
}

func gateToHost(be Backend, src View) []float32 {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{src.S})
	}
	r := be.ToHost(src)
	runtime.KeepAlive(src.S)
	return r
}

func gateUnaryGrad(be Backend, op UnOp, p float32, gx Storage, g Storage, x View, y Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S, y})
	}
	be.UnaryGrad(op, p, gx, g, x, y)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(y)
}

func gateWhere(be Backend, out Storage, outShape []int, c View, a View, b View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{c.S, a.S, b.S})
	}
	be.Where(out, outShape, c, a, b)
	runtime.KeepAlive(out)
	runtime.KeepAlive(c.S)
	runtime.KeepAlive(a.S)
	runtime.KeepAlive(b.S)
}

func gateWhereGrad(be Backend, dst Storage, dstShape []int, outShape []int, g Storage, c View, pick bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{g, c.S})
	}
	be.WhereGrad(dst, dstShape, outShape, g, c, pick)
	runtime.KeepAlive(dst)
	runtime.KeepAlive(g)
	runtime.KeepAlive(c.S)
}

func gateCrossEntropyGradSet(be CrossEntropyGradSetter, gx Storage, g Storage, x View, lse Storage, targets []int, C int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S, lse})
	}
	be.CrossEntropyGradSet(gx, g, x, lse, targets, C)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(lse)
}

func gateRunFused(be Fuser, node *FuseNode, out Storage, ins []View, add bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, append([]Storage{}, viewStorages(ins)...))
	}
	be.RunFused(node, out, ins, add)
	runtime.KeepAlive(out)
	runtime.KeepAlive(ins)
}

func gateGemmSet(be GemmSetter, batch int, M int, N int, K int, a View, transA bool, b View, transB bool, c Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{c}, []Storage{a.S, b.S})
	}
	be.GemmSet(batch, M, N, K, a, transA, b, transB, c)
	runtime.KeepAlive(a.S)
	runtime.KeepAlive(b.S)
	runtime.KeepAlive(c)
}

func gateGemmVendor(be VendorGemmer, batch int, M int, N int, K int, a View, transA bool, b View, transB bool, c Storage, set bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{c}, []Storage{a.S, b.S})
	}
	be.GemmVendor(batch, M, N, K, a, transA, b, transB, c, set)
	runtime.KeepAlive(a.S)
	runtime.KeepAlive(b.S)
	runtime.KeepAlive(c)
}

func gateClipGradAll(be GradClipper, grads []Storage, maxNorm float32) (norm Storage, ok bool) {
	if unlockedOps.Load() != 0 {
		gate(append([]Storage{}, grads...), nil)
	}
	norm, ok = be.ClipGradAll(grads, maxNorm)
	runtime.KeepAlive(grads)
	return norm, ok
}

func gateEmbeddingAt(be IndexReader, out Storage, w View, idx View, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{w.S, idx.S})
	}
	be.EmbeddingAt(out, w, idx, D)
	runtime.KeepAlive(out)
	runtime.KeepAlive(w.S)
	runtime.KeepAlive(idx.S)
}

func gateOpenQuant(be QuantOpener, out Storage, src View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{src.S})
	}
	be.OpenQuant(out, src)
	runtime.KeepAlive(out)
	runtime.KeepAlive(src.S)
}

func gateUpdateSliceAt(be IndexReader, dst View, src View, start View, outer int, Td int, Ts int, inner int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst.S}, []Storage{src.S, start.S})
	}
	be.UpdateSliceAt(dst, src, start, outer, Td, Ts, inner)
	runtime.KeepAlive(dst.S)
	runtime.KeepAlive(src.S)
	runtime.KeepAlive(start.S)
}

func gateScaleAll(be MultiTensorOps, s []Storage, k float32) {
	if unlockedOps.Load() != 0 {
		gate(append([]Storage{}, s...), nil)
	}
	be.ScaleAll(s, k)
	runtime.KeepAlive(s)
}

func gateLayerNormGradSet(be NormGradSetter, gx Storage, ggamma Storage, gbeta Storage, g Storage, x View, gamma *View, mean Storage, rstd Storage, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx, ggamma, gbeta}, []Storage{g, x.S, viewStorage(gamma), mean, rstd})
	}
	be.LayerNormGradSet(gx, ggamma, gbeta, g, x, gamma, mean, rstd, D)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(ggamma)
	runtime.KeepAlive(gbeta)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(gamma)
	runtime.KeepAlive(mean)
	runtime.KeepAlive(rstd)
}

func gateRMSNormGradSet(be NormGradSetter, gx Storage, ggamma Storage, g Storage, x View, gamma *View, rinv Storage, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx, ggamma}, []Storage{g, x.S, viewStorage(gamma), rinv})
	}
	be.RMSNormGradSet(gx, ggamma, g, x, gamma, rinv, D)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(ggamma)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(gamma)
	runtime.KeepAlive(rinv)
}

func gateToRaw(be RawBackend, src View) []byte {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{src.S})
	}
	r := be.ToRaw(src)
	runtime.KeepAlive(src.S)
	return r
}

func gateRelease(be Releaser, s Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{s}, nil)
	}
	be.Release(s)
	runtime.KeepAlive(s)
}

func gateAttentionSeqMajor(be SeqMajorAttender, q View, k View, v View, mask bool, p AttentionParams) bool {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{q.S, k.S, v.S})
	}
	r := be.AttentionSeqMajor(q, k, v, mask, p)
	runtime.KeepAlive(q.S)
	runtime.KeepAlive(k.S)
	runtime.KeepAlive(v.S)
	return r
}

func gateSumSquaresAll(be SquareSummer, s []Storage) float64 {
	if unlockedOps.Load() != 0 {
		gate(nil, append([]Storage{}, s...))
	}
	r := be.SumSquaresAll(s)
	runtime.KeepAlive(s)
	return r
}

func gateTyped(be TypedBackend, op TypedOp, out Storage, outShape []int, ins []View) (fault *ElemFault, ok bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, append([]Storage{}, viewStorages(ins)...))
	}
	fault, ok = be.Typed(op, out, outShape, ins)
	runtime.KeepAlive(out)
	runtime.KeepAlive(ins)
	return fault, ok
}

func gateUnaryGradSet(be UnaryGradSetter, op UnOp, p float32, gx Storage, g Storage, x View, y Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S, y})
	}
	be.UnaryGradSet(op, p, gx, g, x, y)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g)
	runtime.KeepAlive(x.S)
	runtime.KeepAlive(y)
}
