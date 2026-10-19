package debugger

import "github.com/sekerahmet/TCB/internal/lang/interp"

// IzlemeOzetleyici, değeri satır içinde kısaca gösterilebilen motor değerlerinin (ör. skaler
// ya da birkaç elemanlı tensör) uyguladığı isteğe bağlı arayüzdür. IzlemeDegeri'nin etiketi
// ("Tensör (skaler)") değeri söylemez; özet söyler ("0.6074 (Tensör, skaler)").
type IzlemeOzetleyici interface {
	// IzlemeOzeti değer sütununda gösterilecek kısa metindir; değer satır içine sığmıyorsa
	// (ör. büyük tensör) boş metin döner ve yalnızca etiket gösterilir.
	IzlemeOzeti() string
}

// izlemeOzeti değerin satır içi özetini verir; yoksa boş metin.
func izlemeOzeti(val interp.Value) string {
	if o, ok := val.(IzlemeOzetleyici); ok {
		return o.IzlemeOzeti()
	}
	return ""
}

// izlemeGosterimi IzlemeDegeri uygulayan bir değerin değer sütunu metnidir: satır içi özet
// varsa o, yoksa etiket.
func izlemeGosterimi(val interp.Value, iz IzlemeDegeri) string {
	if o := izlemeOzeti(val); o != "" {
		return o
	}
	return iz.IzlemeEtiketi()
}
