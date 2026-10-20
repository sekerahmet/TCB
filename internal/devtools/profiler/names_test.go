package profiler

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestOpNameCoversEveryOp: her tensör işleminin raporda görünen adı bir ad koduna (opCode),
// bir ileti koduna (opLabel) ya da opName'in işleç/metot dalına bağlıdır; hiçbiri değilse
// rapor tensör paketinin ASCII anahtarını yazardı.
func TestOpNameCoversEveryOp(t *testing.T) {
	special := map[tensor.OpKind]bool{
		tensor.OpKindAdd: true, tensor.OpKindSub: true, tensor.OpKindMul: true, tensor.OpKindDiv: true,
		tensor.OpKindLess: true, tensor.OpKindGreater: true, tensor.OpKindLessEq: true,
		tensor.OpKindGreaterEq: true, tensor.OpKindMatMul: true, tensor.OpKindToDevice: true,
		tensor.OpKindCast: true, tensor.OpKindAdamW: true, tensor.OpKindSGD: true,
	}
	for i := tensor.OpKind(0); i < tensor.OpKindCount; i++ {
		_, a := opCode[i]
		_, b := opLabel[i]
		if !a && !b && !special[i] {
			t.Errorf("%d. işlemin (%s) rapor adı yok: ad koduna ya da opLabel'a bağlayın", i, i.Code())
		}
		if opName(i) == "" {
			t.Errorf("%s: rapor adı boş", i.Code())
		}
	}
}
