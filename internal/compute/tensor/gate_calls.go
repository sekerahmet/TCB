package tensor

// Arka uç çağrılarının kapısı (access.go). Tensör paketi arka uca veri okuyan ya da yazan her
// çağrıyı buradaki gateX ile yapar: kilitsiz bir işlemin tuttuğu bir depoyla çakışan çağrı, o işlem
// bitene dek bekler. Yöntem başına yazılan parametreler elle sınıflandırıldı (çıktılar, gradyanlar,
// iyileştiricinin durumu); öbür depo ve görünüm parametreleri okunur. Yapısal test
// (gate_calls_test.go) paketin başka yerinden arka uca doğrudan veri çağrısı yapılmadığını sınar.

// viewStorage isteğe bağlı görünümün deposudur (yoksa nil).
func viewStorage(v *View) Storage {
	if v == nil {
		return nil
	}
	return v.S
}

// viewStorages görünümlerin depolarıdır.
func viewStorages(vs []View) []Storage {
	out := make([]Storage, len(vs))
	for i, v := range vs {
		out[i] = v.S
	}
	return out
}

func gateAttentionGradSet(be AttentionGradSetter, gq Storage, gk Storage, gv Storage, g Storage, q View, k View, v View, mask Storage, o Storage, lse Storage, p AttentionParams, fresh [3]bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gq, gk, gv}, []Storage{g, q.S, k.S, v.S, mask, o, lse})
	}
	be.AttentionGradSet(gq, gk, gv, g, q, k, v, mask, o, lse, p, fresh)
}

func gateAdamWStep(be Backend, p Storage, g Storage, m Storage, v Storage, lr float32, b1 float32, b2 float32, eps float32, wd float32, bc1 float32, bc2 float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{p, m, v}, []Storage{g})
	}
	be.AdamWStep(p, g, m, v, lr, b1, b2, eps, wd, bc1, bc2)
}

func gateAddTo(be Backend, dst Storage, src Storage, scale float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{src})
	}
	be.AddTo(dst, src, scale)
}

func gateArgMax(be Backend, x View, axis int) []int {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{x.S})
	}
	return be.ArgMax(x, axis)
}

func gateAtan2Grad(be Backend, ga Storage, gb Storage, aShape []int, bShape []int, outShape []int, g Storage, a View, b View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{ga, gb}, []Storage{g, a.S, b.S})
	}
	be.Atan2Grad(ga, gb, aShape, bShape, outShape, g, a, b)
}

func gateAttention(be Backend, out Storage, q View, k View, v View, mask Storage, p AttentionParams) (lse Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{q.S, k.S, v.S, mask})
	}
	return be.Attention(out, q, k, v, mask, p)
}

func gateAttentionGrad(be Backend, gq Storage, gk Storage, gv Storage, g Storage, q View, k View, v View, mask Storage, o Storage, lse Storage, p AttentionParams) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gq, gk, gv}, []Storage{g, q.S, k.S, v.S, mask, o, lse})
	}
	be.AttentionGrad(gq, gk, gv, g, q, k, v, mask, o, lse, p)
}

func gateBinary(be Backend, op BinOp, out Storage, outShape []int, a View, b View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{a.S, b.S})
	}
	be.Binary(op, out, outShape, a, b)
}

func gateBinaryGrad(be Backend, mode GradMode, dst Storage, dstShape []int, outShape []int, g Storage, o View, y Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{g, o.S, y})
	}
	be.BinaryGrad(mode, dst, dstShape, outShape, g, o, y)
}

func gateClamp(be Backend, out Storage, x View, lo float32, hi float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.Clamp(out, x, lo, hi)
}

func gateClampGrad(be Backend, gx Storage, g Storage, x View, lo float32, hi float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S})
	}
	be.ClampGrad(gx, g, x, lo, hi)
}

func gateCopy(be Backend, dst Storage, dstOff int, src View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{src.S})
	}
	be.Copy(dst, dstOff, src)
}

func gateCopyStrided(be Backend, dst View, src View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst.S}, []Storage{src.S})
	}
	be.CopyStrided(dst, src)
}

func gateCrossEntropy(be Backend, x View, targets []int, C int) (loss Storage, lse Storage) {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{x.S})
	}
	return be.CrossEntropy(x, targets, C)
}

func gateCrossEntropyGrad(be Backend, gx Storage, g Storage, x View, lse Storage, targets []int, C int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S, lse})
	}
	be.CrossEntropyGrad(gx, g, x, lse, targets, C)
}

func gateCumSum(be Backend, out Storage, x View, axis int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.CumSum(out, x, axis)
}

func gateCumSumGrad(be Backend, gx Storage, g Storage, shape []int, axis int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g})
	}
	be.CumSumGrad(gx, g, shape, axis)
}

func gateEmbedding(be Backend, out Storage, w View, idx []int, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{w.S})
	}
	be.Embedding(out, w, idx, D)
}

func gateEmbeddingGrad(be Backend, gw Storage, g Storage, idx []int, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gw}, []Storage{g})
	}
	be.EmbeddingGrad(gw, g, idx, D)
}

func gateExpandAdd(be Backend, dst Storage, dstShape []int, src Storage, srcShape []int, scale float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{src})
	}
	be.ExpandAdd(dst, dstShape, src, srcShape, scale)
}

func gateFill(be Backend, dst Storage, v float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, nil)
	}
	be.Fill(dst, v)
}

func gateGatherGrad(be Backend, gx Storage, g Storage, idx Storage, shape []int, axis int, k int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, idx})
	}
	be.GatherGrad(gx, g, idx, shape, axis, k)
}

func gateGemm(be Backend, batch int, M int, N int, K int, a View, transA bool, b View, transB bool, c Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{c}, []Storage{a.S, b.S})
	}
	be.Gemm(batch, M, N, K, a, transA, b, transB, c)
}

func gateLayerNorm(be Backend, out Storage, x View, gamma *View, beta *View, D int, eps float32) (mean Storage, rstd Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S, viewStorage(gamma), viewStorage(beta)})
	}
	return be.LayerNorm(out, x, gamma, beta, D, eps)
}

func gateLayerNormGrad(be Backend, gx Storage, ggamma Storage, gbeta Storage, g Storage, x View, gamma *View, mean Storage, rstd Storage, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx, ggamma, gbeta}, []Storage{g, x.S, viewStorage(gamma), mean, rstd})
	}
	be.LayerNormGrad(gx, ggamma, gbeta, g, x, gamma, mean, rstd, D)
}

func gateMax(be Backend, out Storage, x View, axis int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.Max(out, x, axis)
}

func gateMaxGrad(be Backend, gx Storage, g Storage, x View, axis int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S})
	}
	be.MaxGrad(gx, g, x, axis)
}

func gateRMSNorm(be Backend, out Storage, x View, gamma *View, D int, eps float32) (rinv Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S, viewStorage(gamma)})
	}
	return be.RMSNorm(out, x, gamma, D, eps)
}

func gateRMSNormGrad(be Backend, gx Storage, ggamma Storage, g Storage, x View, gamma *View, rinv Storage, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx, ggamma}, []Storage{g, x.S, viewStorage(gamma), rinv})
	}
	be.RMSNormGrad(gx, ggamma, g, x, gamma, rinv, D)
}

func gateSGDStep(be Backend, p Storage, g Storage, vel Storage, lr float32, momentum float32, wd float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{p, vel}, []Storage{g})
	}
	be.SGDStep(p, g, vel, lr, momentum, wd)
}

func gateScale(be Backend, s Storage, k float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{s}, nil)
	}
	be.Scale(s, k)
}

func gateScatterAdd(be Backend, dst Storage, dstOff int, dstStrides []int, src Storage, shape []int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{src})
	}
	be.ScatterAdd(dst, dstOff, dstStrides, src, shape)
}

func gateSoftmax(be Backend, out Storage, x View, axis int, log bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.Softmax(out, x, axis, log)
}

func gateSoftmaxGrad(be Backend, gx Storage, g Storage, y Storage, shape []int, axis int, log bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, y})
	}
	be.SoftmaxGrad(gx, g, y, shape, axis, log)
}

func gateSort(be Backend, vals Storage, idx Storage, x View, axis int, k int, desc bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{vals, idx}, []Storage{x.S})
	}
	be.Sort(vals, idx, x, axis, k, desc)
}

func gateSum(be Backend, out Storage, x View, axis int, scale float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.Sum(out, x, axis, scale)
}

func gateSumAll(be Backend, out Storage, x View, scale float32) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.SumAll(out, x, scale)
}

func gateSumSquares(be Backend, s Storage) float64 {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{s})
	}
	return be.SumSquares(s)
}

func gateToHost(be Backend, src View) []float32 {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{src.S})
	}
	return be.ToHost(src)
}

func gateUnary(be Backend, op UnOp, p float32, out Storage, x View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.Unary(op, p, out, x)
}

func gateUnaryGrad(be Backend, op UnOp, p float32, gx Storage, g Storage, x View, y Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S, y})
	}
	be.UnaryGrad(op, p, gx, g, x, y)
}

func gateWhere(be Backend, out Storage, outShape []int, c View, a View, b View) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{c.S, a.S, b.S})
	}
	be.Where(out, outShape, c, a, b)
}

func gateWhereGrad(be Backend, dst Storage, dstShape []int, outShape []int, g Storage, c View, pick bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst}, []Storage{g, c.S})
	}
	be.WhereGrad(dst, dstShape, outShape, g, c, pick)
}

func gateCrossEntropyGradSet(be CrossEntropyGradSetter, gx Storage, g Storage, x View, lse Storage, targets []int, C int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S, lse})
	}
	be.CrossEntropyGradSet(gx, g, x, lse, targets, C)
}

func gateRunFused(be Fuser, node *FuseNode, out Storage, ins []View, add bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, append([]Storage{}, viewStorages(ins)...))
	}
	be.RunFused(node, out, ins, add)
}

func gateGemmSet(be GemmSetter, batch int, M int, N int, K int, a View, transA bool, b View, transB bool, c Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{c}, []Storage{a.S, b.S})
	}
	be.GemmSet(batch, M, N, K, a, transA, b, transB, c)
}

func gateClipGradAll(be GradClipper, grads []Storage, maxNorm float32) (norm Storage, ok bool) {
	if unlockedOps.Load() != 0 {
		gate(append([]Storage{}, grads...), nil)
	}
	return be.ClipGradAll(grads, maxNorm)
}

func gateEmbeddingAt(be IndexReader, out Storage, w View, idx View, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{w.S, idx.S})
	}
	be.EmbeddingAt(out, w, idx, D)
}

func gateUpdateSliceAt(be IndexReader, dst View, src View, start View, outer int, Td int, Ts int, inner int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{dst.S}, []Storage{src.S, start.S})
	}
	be.UpdateSliceAt(dst, src, start, outer, Td, Ts, inner)
}

func gateAdamWStepAll(be MultiTensorOps, p []Storage, g []Storage, m []Storage, v []Storage, lr float32, b1 float32, b2 float32, eps float32, wd float32, bc1 []float32, bc2 []float32) {
	if unlockedOps.Load() != 0 {
		gate(append(append(append([]Storage{}, p...), m...), v...), append([]Storage{}, g...))
	}
	be.AdamWStepAll(p, g, m, v, lr, b1, b2, eps, wd, bc1, bc2)
}

func gateSGDStepAll(be MultiTensorOps, p []Storage, g []Storage, vel []Storage, lr float32, momentum float32, wd float32) {
	if unlockedOps.Load() != 0 {
		gate(append(append([]Storage{}, p...), vel...), append([]Storage{}, g...))
	}
	be.SGDStepAll(p, g, vel, lr, momentum, wd)
}

func gateScaleAll(be MultiTensorOps, s []Storage, k float32) {
	if unlockedOps.Load() != 0 {
		gate(append([]Storage{}, s...), nil)
	}
	be.ScaleAll(s, k)
}

func gateLayerNormGradSet(be NormGradSetter, gx Storage, ggamma Storage, gbeta Storage, g Storage, x View, gamma *View, mean Storage, rstd Storage, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx, ggamma, gbeta}, []Storage{g, x.S, viewStorage(gamma), mean, rstd})
	}
	be.LayerNormGradSet(gx, ggamma, gbeta, g, x, gamma, mean, rstd, D)
}

func gateRMSNormGradSet(be NormGradSetter, gx Storage, ggamma Storage, g Storage, x View, gamma *View, rinv Storage, D int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx, ggamma}, []Storage{g, x.S, viewStorage(gamma), rinv})
	}
	be.RMSNormGradSet(gx, ggamma, g, x, gamma, rinv, D)
}

func gateToRaw(be RawBackend, src View) []byte {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{src.S})
	}
	return be.ToRaw(src)
}

func gateRelease(be Releaser, s Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{s}, nil)
	}
	be.Release(s)
}

func gateAttentionSeqMajor(be SeqMajorAttender, q View, k View, v View, mask bool, p AttentionParams) bool {
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{q.S, k.S, v.S})
	}
	return be.AttentionSeqMajor(q, k, v, mask, p)
}

func gateSumSquaresAll(be SquareSummer, s []Storage) float64 {
	if unlockedOps.Load() != 0 {
		gate(nil, append([]Storage{}, s...))
	}
	return be.SumSquaresAll(s)
}

func gateTyped(be TypedBackend, op TypedOp, out Storage, outShape []int, ins []View) (fault *ElemFault, ok bool) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, append([]Storage{}, viewStorages(ins)...))
	}
	return be.Typed(op, out, outShape, ins)
}

func gateUnaryGradSet(be UnaryGradSetter, op UnOp, p float32, gx Storage, g Storage, x View, y Storage) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g, x.S, y})
	}
	be.UnaryGradSet(op, p, gx, g, x, y)
}
