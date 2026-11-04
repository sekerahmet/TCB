package tensor

// sgemv64bf sgemv64'ün bfloat16 ağırlıklı eşi. Assembly: cpu_gemv16_amd64.s.
//
//go:noescape
func sgemv64bf(k int, x *float32, w *uint16, ldw int, y *float32)

// sgemv64h sgemv64'ün float16 ağırlıklı eşi (F16C ister). Assembly: cpu_gemv16_amd64.s.
//
//go:noescape
func sgemv64h(k int, x *float32, w *uint16, ldw int, y *float32)

// sdot32bf sdot32'nin bfloat16 eşi (n 32'nin katı). Assembly: cpu_gemv16_amd64.s.
//
//go:noescape
func sdot32bf(n int, x *float32, y *uint16) float32

// sdot32h sdot32'nin float16 eşi (n 32'nin katı, F16C ister). Assembly: cpu_gemv16_amd64.s.
//
//go:noescape
func sdot32h(n int, x *float32, y *uint16) float32

// direct16 gemv16 bu işlemcide "aç + gemv" ile aynı bitleri verir mi. AVX2'li ama F16C'siz
// işlemcide float16'nın çekirdeği yoktur; Go eşi de AVX2'li gemv'nin bitlerini vermez (FMA yok),
// o işlemcide float16 açılarak çarpılır.
func direct16(dt DType) bool { return !hasAVX2 || dt == BFloat16 || hasF16C }

// gemvStrip16 gemvStrip'in 16 bitlik eşi; AVX2 yoksa Go eşi.
func gemvStrip16(a []float32, b []uint16, ldb int, y []float32, dt DType) {
	if !hasAVX2 || len(a) == 0 {
		gemvStripGo16(a, b, ldb, y[:64], dt)
		return
	}
	_ = b[(len(a)-1)*ldb+63]
	_ = y[63]
	if dt == BFloat16 {
		sgemv64bf(len(a), &a[0], &b[0], ldb, &y[0])
		return
	}
	sgemv64h(len(a), &a[0], &b[0], ldb, &y[0])
}

// dot16 x·float32(y), dotF32 sırasıyla; 32'lik bloklar assembly ile, kalan row'a açılıp Go ile.
func dot16(x []float32, y []uint16, dt DType, row []float32) float32 {
	y = y[:len(x)]
	if !hasAVX2 || len(x) < 32 {
		widenSpan(row[:len(x)], y, dt)
		return dot32(x, row[:len(x)])
	}
	n := len(x) &^ 31
	var s float32
	if dt == BFloat16 {
		s = sdot32bf(n, &x[0], &y[0])
	} else {
		s = sdot32h(n, &x[0], &y[0])
	}
	if n < len(x) {
		tail := row[:len(x)-n]
		widenSpan(tail, y[n:], dt)
		s += dot32(x[n:], tail)
	}
	return s
}
