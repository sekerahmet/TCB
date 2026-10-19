package debugger

import "github.com/sekerahmet/TCB/internal/lang/interp"

// WatchSummarizer, değeri satır içinde kısaca gösterilebilen motor değerlerinin (ör. skaler
// ya da birkaç elemanlı tensör) uyguladığı isteğe bağlı arayüzdür. WatchValue'nin etiketi
// ("Tensör (skaler)") değeri söylemez; özet söyler ("0.6074 (Tensör, skaler)").
type WatchSummarizer interface {
	// WatchSummary değer sütununda gösterilecek kısa metindir; değer satır içine sığmıyorsa
	// (ör. büyük tensör) boş metin döner ve yalnızca etiket gösterilir.
	WatchSummary() string
}

// watchSummary değerin satır içi özetini verir; yoksa boş metin.
func watchSummary(val interp.Value) string {
	if o, ok := val.(WatchSummarizer); ok {
		return o.WatchSummary()
	}
	return ""
}

// watchDisplay WatchValue uygulayan bir değerin değer sütunu metnidir: satır içi özet
// varsa o, yoksa etiket.
func watchDisplay(val interp.Value, iz WatchValue) string {
	if o := watchSummary(val); o != "" {
		return o
	}
	return iz.WatchLabel()
}
