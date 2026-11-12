package tensor

// Kapının yardımcıları (gate_calls.go): görünümlerin depoları. Kapı dosyası yalnız arka uç
// çağrılarını taşır; yapısal testler (gate_calls_test.go, host_block_test.go) onu adıyla tarar.

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
