package testkit

import (
	"os"
	"strings"
)

// RuntimeVariables Go'nun çalışma zamanının ortam değişkenleridir (go1.27.4 src/runtime/extern.go):
// dilin ayarı değildirler; trlang onları süreç başında bir satırla bildirir (19.12a). trlang'ı başlatan
// testler onları alt sürecin ortamından çıkarır: test sürecine verilen değer (zorlama koşusunun
// GOGC=1'i) alt süreçte uyarı satırı olur ve çıktıyı tam karşılaştıran testleri düşürürdü.
var RuntimeVariables = []string{"GOGC", "GOMEMLIMIT", "GODEBUG", "GOMAXPROCS", "GORACE", "GOTRACEBACK"}

// WithoutRuntimeVariables ortamdan çalışma zamanının değişkenlerini çıkarır.
func WithoutRuntimeVariables(env []string) []string {
	var out []string
	for _, e := range env {
		name, _, _ := strings.Cut(e, "=")
		if !isRuntimeVariable(name) {
			out = append(out, e)
		}
	}
	return out
}

// ClearRuntimeVariables çalışma zamanının değişkenlerini test sürecinin ortamından siler; TestMain
// çağırır. Go değeri süreç başında bir kez okuduğundan test süreci verilen değerle sürer, başlattığı
// alt süreçler değişkensiz başlar. GODEBUG'ın silinmesi test sürecinin hata ayıklama değişkenlerini
// yeniden okutur (go1.27.4 src/runtime/runtime.go, godebugNotify).
func ClearRuntimeVariables() {
	for _, e := range os.Environ() {
		if name, _, _ := strings.Cut(e, "="); isRuntimeVariable(name) {
			os.Unsetenv(name)
		}
	}
}

// isRuntimeVariable name çalışma zamanının değişkeni mi; Windows'ta ortam adları harf büyüklüğüne
// duyarsızdır.
func isRuntimeVariable(name string) bool {
	for _, v := range RuntimeVariables {
		if strings.EqualFold(name, v) {
			return true
		}
	}
	return false
}
