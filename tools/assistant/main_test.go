package assistant_test

import (
	"os"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// TestMain çalışma zamanının değişkenlerini ortamdan siler: testler trlang'ı alt süreç olarak başlatır,
// zorlama koşusunun GOGC=1'i orada uyarı satırı olur ve standart hatayı tam karşılaştıran testleri
// düşürürdü (19.12a; testkit.ClearRuntimeVariables).
func TestMain(m *testing.M) {
	testkit.ClearRuntimeVariables()
	os.Exit(m.Run())
}
