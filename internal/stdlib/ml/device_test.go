package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (15.37): cihaz adı tek yazımlıdır. Büyük harf, boşluk, işaret, baştaki sıfır, başka yazının
// rakamı ve sayıdan sonraki çöp bilinmeyen cihazdır (H6800); üç çağrı aynı çözümleyiciyi kullanır.
func TestDeviceNameSpelling(t *testing.T) {
	for name, want := range map[string]tensor.Device{
		"cpu": tensor.CPU, "gpu": tensor.GPU(0), "gpu:0": tensor.GPU(0), "gpu:1": tensor.GPU(1), "gpu:10": tensor.GPU(10),
	} {
		if dev, err := parseDevice(name); err != nil || dev != want {
			t.Errorf("%q: %v %v, beklenen %v", name, dev, err, want)
		}
	}
	for _, name := range []string{
		"", " ", " CPU ", "Cpu", "CPU", "cpu ", " cpu", "cpu\n", "cpu:0", "GPU", "Gpu", "GPU:0", "ĞPU", " gpu", "gpu ", "gpu :0", "gpu: 0",
		"gpu:", "gpu:0 ", "gpu:00", "gpu:01", "gpu:-0", "gpu:-1", "gpu:+1", "gpu:1a", "gpu:0x1", "gpu:1_0", "gpu:\uff19",
		"gpu:99999999999999999999", "tpu", "cuda", "cuda:0",
	} {
		if dev, err := parseDevice(name); !messages.Has(err, messages.H6800) {
			t.Errorf("%q: %v %v, beklenen H6800", name, dev, err)
		}
	}
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `TR_for ad TR_in ["gpu", "gpu:0"] {
    TR_default_device(ad)
    TR_print(TR_Tensor([1.0]).TR_to_device(ad).TR_device, TR_default_device(), TR_Type(TR_device_info(ad)))
}`)
	if want := "gpu:0 gpu:0 DeviceInfo\ngpu:0 gpu:0 DeviceInfo\n"; out != want {
		t.Errorf("geçerli adlar: %q, beklenen %q", out, want)
	}
	for _, call := range []string{`TR_Tensor([1.0]).TR_to_device("GPU")`, `TR_default_device("GPU")`, `TR_device_info("GPU")`} {
		if _, err := run(t, call); !messages.Has(err, messages.H6800) {
			t.Errorf("%s: %v, beklenen H6800", call, err)
		}
	}
}

// Kural (15.37): bilinmeyen adın iletisi geçerli yazımı ve TR_devices()'ı söyler, istenen adın yerine
// başka bir ad önermez; andığı "cpu" ve TR_devices() koşar.
func TestUnknownDeviceMessage(t *testing.T) {
	for lang, want := range map[string]string{
		"tr": "bilinmeyen cihaz \"CUDA\": cihaz adı \"cpu\", \"gpu\" ya da \"gpu:0\", \"gpu:1\" gibi yazılır (küçük harf, boşluksuz, sıranın başında sıfır yok); bu makinedekileri `TR_devices()` verir",
		"en": "unknown device \"CUDA\": a device name is written \"cpu\", \"gpu\", or like \"gpu:0\", \"gpu:1\" (lowercase, no spaces, no leading zero in the index); `TR_devices()` lists the ones on this machine",
	} {
		if got := messages.TextIn(lang, messages.H6800, "CUDA"); got != want {
			t.Errorf("%s:\n%s\nbeklenen:\n%s", lang, got, want)
		}
	}
	out := mustRun(t, `TR_print(TR_Tensor([1.0]).TR_to_device("cpu").TR_device, TR_devices())`)
	if !strings.HasPrefix(out, `cpu ["cpu"`) {
		t.Errorf("iletinin yazımları: %q", out)
	}
}
