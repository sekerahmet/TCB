package tensor

import "runtime"

// Arka uç çağrılarının kapısının devamı (gate_calls.go 600 satır sınırında): aynı kural, ayrı
// dosya; yapısal testler gate_calls_*.go'yu da kapı sayar.

func gateRowLogSumExp(be RowLogSumExper, out Storage, x View, rows int, n int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{out}, []Storage{x.S})
	}
	be.RowLogSumExp(out, x, rows, n)
	runtime.KeepAlive(out)
	runtime.KeepAlive(x.S)
}

func gateRowLogSumExpGrad(be RowLogSumExper, gx Storage, g View, x View, rows int, n int) {
	if unlockedOps.Load() != 0 {
		gate([]Storage{gx}, []Storage{g.S, x.S})
	}
	be.RowLogSumExpGrad(gx, g, x, rows, n)
	runtime.KeepAlive(gx)
	runtime.KeepAlive(g.S)
	runtime.KeepAlive(x.S)
}
