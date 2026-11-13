package assistant_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Zincirin cihaz geçişleri (T1 tasarımının G7'si, dilim 4a): kesitleyiciye giden örnekler ve mikrofonun çerçevesi
// işlemcide, kesit yazıya çevrilmeden önce varsayılan cihazda. Kart benzeticide (TR_GPU_SIM) sınanır; varsayılan cihaz
// işlemciyken üçü de boş taşımadır. Program modelsizdir: zincir.tr'nin işlevlerini çağırır.
const deviceProgram = `# TR 4
TR_import "{zincir}"
TR_import "{nicemli}"
TR_var ayar = TR_command_line({"cihaz": "cpu"})
TR_default_device(TR_String(ayar["cihaz"]))
TR_var örnekler = TR_zeros([1, 1024])
TR_print("KURULAN", örnekler.TR_device)
TR_print("VARSAYILAN", TR_default_device(), örnekler.TR_device == TR_default_device())
TR_print("G1", işlemcide(örnekler[0]).TR_device)
TR_var çerçeve = çerçeve_örnekleri(TR_Bytes([0, 128, 255, 127, 1, 0, 0, 0]))
TR_print("C1", çerçeve.TR_device, çerçeve.TR_dtype, çerçeve[0].TR_item(), çerçeve[2].TR_item(), çerçeve[3].TR_item())
TR_print("ÜST", çerçeve[1].TR_item() < 1.0, çerçeve[1].TR_item() > 0.9999)
TR_print("G2", kesit_cihazda(işlemcide(TR_zeros([1024]))).TR_device)
TR_print("NİCEMLİ", nicemli_hatası(TR_default_device()) == TR_none)
TR_if TR_default_device() == "cpu" { TR_print("NİCEMLİ_KART atlandı") } TR_else {
    TR_var w = TR_zeros([3, 256], TR_dtype: "q4_k")
    TR_var ileti = ""
    TR_var neden = ""
    TR_try {
        TR_try { TR_quantized_activations { TR_ones([2, 256]) @ w.TR_transpose() } } TR_catch e { nicemli_hatasını_at(e) }
    } TR_catch h {
        ileti = TR_String(h.TR_value)
        neden = TR_String(h.TR_cause.TR_code)
    }
    TR_print("NİCEMLİ_KART", ileti.TR_contains("nicemlenmiş ağırlıkları kartta"), neden)
}
`

func TestChainDeviceTransitions(t *testing.T) {
	zincir := filepath.ToSlash(strings.TrimSuffix(reporoot.Path(chainModule), ".tr"))
	nicemli := filepath.ToSlash(strings.TrimSuffix(reporoot.Path(quantModule), ".tr"))
	dir := t.TempDir()
	program := filepath.Join(dir, "cihaz.tr")
	text := strings.NewReplacer("{zincir}", zincir, "{nicemli}", nicemli).Replace(deviceProgram)
	if err := os.WriteFile(program, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	// Nicemli seçenek (H1 nicemli zincir, K3): kartta cihaz denetimi hata verir; kartta kurulan q4_k ağırlığın
	// blokta verdiği gerçek H6724 nicemli_hatasını_at'tan kullanıcının metniyle, nedeniyle çıkar.
	for _, d := range []struct{ device, built, whisper, quant, card string }{
		{"cpu", "cpu", "cpu", "true", "atlandı"},
		{"gpu", "gpu:0", "gpu:0", "false", "true H6724"},
	} {
		t.Run(d.device, func(t *testing.T) {
			cmd := exec.Command(trlangBinary(t), program, "--cihaz", d.device)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "TR_GPU_SIM=1")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("trlang: %v\n%s", err, out)
			}
			got := string(out)
			for _, want := range []string{
				"KURULAN " + d.built + "\n",
				// Kararların adı .TR_device ile aynı uzun addır (15.38): karşılaştırma kartta da doğru.
				"VARSAYILAN " + d.built + " true\n",
				"G1 cpu\n",
				"C1 cpu float32 -1.0 3.0517578e-05 0.0\n",
				"ÜST true true\n",
				"G2 " + d.whisper + "\n",
				"NİCEMLİ " + d.quant + "\n",
				"NİCEMLİ_KART " + d.card + "\n",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("--cihaz %s: çıktıda %q yok:\n%s", d.device, want, got)
				}
			}
		})
	}
}

// asistan.tr'nin --cihaz'ı (T1 tasarımının G7'si, dilim 4b): varsayılan cihaz modellerden ve yan etkilerden önce kurulur.
// Program modelsiz koşar (yolları olmayan dosyalar): nicemli denetimi seçilen cihazı görür (kartta durur, işlemcide
// parçalayıcının yükleme hatasına varır); geçersiz ya da kullanılamayan cihaz deneme klasörü kurulmadan durur; ekranlı
// kipte cpu dışındaki cihaz sunucu açılmadan hatadır. Kart benzeticide (TR_GPU_SIM) tek kart vardır: gpu:7 hiçbir
// makinede yoktur.
func TestChainDeviceOptionIsSetBeforeTheModels(t *testing.T) {
	program := reporoot.Path(chainProgram)
	models := []string{"--dil_modeli", "yok.gguf", "--whisper", "yok", "--vad", "yok", "--ses_modeli", "yok"}
	// record: koşu --deneme_klasörü alır ve klasör kurulmamış olmalı (cihazda duran durumlar).
	for _, c := range []struct {
		name      string
		args      []string
		want, not string
		record    bool
	}{
		{"gpu, nicemli", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "--cihaz", "gpu"}, "dil modelinin cihazı gpu:0", "yüklenemedi", false},
		{"cpu, nicemli", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "--cihaz", "cpu"}, "yüklenemedi", "dil modelinin cihazı", false},
		{"seçeneksiz, nicemli", []string{"--kip", "dosya", "--nicemli_etkinleştirme"}, "yüklenemedi", "dil modelinin cihazı", false},
		{"kart", []string{"--kip", "dosya", "--cihaz", "kart"}, `bilinmeyen cihaz "kart"`, "DENEME", true},
		{"boş ad", []string{"--kip", "dosya", "--cihaz", ""}, `bilinmeyen cihaz ""`, "DENEME", true},
		{"gpu:7", []string{"--kip", "dosya", "--cihaz", "gpu:7"}, "gpu:7 cihazı kullanılamıyor; bu makinede kullanılabilir cihazlar: cpu, gpu:0", "DENEME", true},
		{"ekranlı, gpu", []string{"--kip", "canlı", "--ekran", "0", "--cihaz", "gpu"}, "--cihaz gpu:0 ekranlı kipte verilmez", "EKRAN", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			record := filepath.Join(dir, "kayit")
			args := append([]string{program}, c.args...)
			args = append(args, models...)
			args = append(args, "--belirteç_dosyası", filepath.Join(dir, "belirtec.txt"))
			if c.record {
				args = append(args, "--deneme_klasörü", record)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, trlangBinary(t), args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "TR_GPU_SIM=1")
			out, err := cmd.CombinedOutput()
			got := string(out)
			if err == nil || ctx.Err() != nil {
				t.Fatalf("%s: hatayla durmalıydı (err %v, ctx %v):\n%s", c.name, err, ctx.Err(), got)
			}
			if !strings.Contains(got, c.want) {
				t.Errorf("%s: çıktıda %q yok:\n%s", c.name, c.want, got)
			}
			if strings.Contains(got, c.not) {
				t.Errorf("%s: çıktıda %q olmamalı:\n%s", c.name, c.not, got)
			}
			if _, err := os.Stat(record); c.record && !os.IsNotExist(err) {
				t.Errorf("%s: deneme klasörü kurulmamalıydı (%v)", c.name, err)
			}
		})
	}
}
