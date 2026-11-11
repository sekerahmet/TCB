package ml

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// ----------------------------------------------------------------------------
// üretici_çarpımı bloğu (15.53b)
// ----------------------------------------------------------------------------

// vendorMatmulKey görev durumunda TR_vendor_matmul bloğunun anahtarıdır.
type vendorMatmulKey struct{}

// vendorMatmulOn görevin TR_vendor_matmul bloğunun içinde olup olmadığıdır. Çarpımlar bu durumu
// kayıt anında düğüme yazar (tensor.MatMulVendorKernel, AttentionOptions.Vendor): geri yön
// kaydedilen durumu kullanır.
func vendorMatmulOn(m *interp.VM) bool { return taskFlag(m, vendorMatmulKey{}) }

// vendorMatmul TR_vendor_matmul { ... } yerleşiğidir: bloğu, GPU'daki matris çarpımlarının
// üreticinin kitaplığına verildiği kipte çalıştırır ve bloğun son ifadesinin değerini döndürür
// (15.53b; durum kuralları 15.53a'nınkiler, flagBlock). İşlemcide blok etkisizdir.
func vendorMatmul(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	return flagBlock(m, vendorMatmulKey{}, codes.FnVendorMatmul, args, named)
}

// vendorMatmulOnBuiltin TR_vendor_matmul_on() yerleşiğidir: görev TR_vendor_matmul bloğunun
// içindeyse doğru (15.53b).
func vendorMatmulOnBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	return flagQuery(m, vendorMatmulKey{}, codes.FnVendorMatmulOn, args, named)
}
