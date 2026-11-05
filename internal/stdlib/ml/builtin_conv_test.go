package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (25.9, 15.41): TR_conv1d ve TR_logsumexp kartta hata verir, CPU'ya sessizce düşmez; ileti
// taşımanın yazımını söyler.
func TestConvAndLogSumExpDoNotFallBackFromTheCard(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, src := range []string{
		`TR_logsumexp(TR_ones([2]).TR_to_device("gpu"))`,
		`TR_conv1d(TR_ones([1, 1, 4]).TR_to_device("gpu"), TR_ones([1, 1, 2]), TR_none)`,
		`TR_conv1d(TR_ones([1, 1, 4]), TR_ones([1, 1, 2]).TR_to_device("gpu"), TR_none)`,
		`TR_conv1d(TR_ones([1, 1, 4]), TR_ones([1, 1, 2]), TR_ones([1]).TR_to_device("gpu"))`,
	} {
		_, err := run(t, src)
		if err == nil || !strings.Contains(err.Error(), ".TR_to_device(") {
			t.Errorf("%s: kartta taşıma önerilmeliydi: %v", src, err)
		}
	}
}
