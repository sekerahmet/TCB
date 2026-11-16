package assistant_test

import (
	"context"
	"encoding/json"
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
TR_var açık = NicemliSeçim(değer: "açık", kaynak: "komut")
TR_print("NİCEMLİ", nicemli_hatası(TR_default_device(), açık) == TR_none)
TR_if TR_default_device() == "cpu" { TR_print("NİCEMLİ_KART atlandı") } TR_else {
    TR_var w = TR_zeros([3, 256], TR_dtype: "q4_k")
    TR_var ileti = ""
    TR_var neden = ""
    TR_try {
        TR_try { TR_quantized_activations { TR_ones([2, 256]) @ w.TR_transpose() } } TR_catch e { nicemli_hatasını_at(e, açık) }
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
// Program modelsiz koşar (yolları olmayan dosyalar): açık nicemli seçimin denetimi seçilen cihazı görür (kartta durur,
// işlemcide parçalayıcının yükleme hatasına varır), otomatik kartta da durmaz; seçimin yanlış değeri doğrusunu önerir
// (true ve on açık, false ve off kapalı, auto otomatik), değersiz bayrak dilin kullanım hatasıdır (DENETİM 774 N1);
// geçersiz ya da kullanılamayan cihaz deneme klasörü kurulmadan durur; ekranlı kipte cpu dışındaki cihaz sunucu
// açılmadan hatadır. Kart benzeticide (TR_GPU_SIM) tek kart vardır: gpu:7 hiçbir
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
		{"gpu, nicemli açık", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "açık", "--cihaz", "gpu"}, "dil modelinin cihazı gpu:0", "yüklenemedi", false},
		{"cpu, nicemli açık", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "açık", "--cihaz", "cpu"}, "yüklenemedi", "dil modelinin cihazı", false},
		{"seçeneksiz, nicemli açık", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "açık"}, "yüklenemedi", "dil modelinin cihazı", false},
		{"gpu, nicemli otomatik", []string{"--kip", "dosya", "--cihaz", "gpu"}, "yüklenemedi", "dil modelinin cihazı", false},
		{"nicemli true", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "true"}, "--nicemli_etkinleştirme otomatik, açık ya da kapalı olur, true verildi. Örnek: --nicemli_etkinleştirme açık", "yüklenemedi", false},
		{"nicemli on", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "on"}, "on verildi. Örnek: --nicemli_etkinleştirme açık", "yüklenemedi", false},
		{"nicemli false", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "false"}, "false verildi. Örnek: --nicemli_etkinleştirme kapalı", "yüklenemedi", false},
		{"nicemli off", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "off"}, "off verildi. Örnek: --nicemli_etkinleştirme kapalı", "yüklenemedi", false},
		{"nicemli auto", []string{"--kip", "dosya", "--nicemli_etkinleştirme", "auto"}, "auto verildi. Örnek: --nicemli_etkinleştirme otomatik", "yüklenemedi", false},
		{"nicemli boş", []string{"--kip", "dosya", "--nicemli_etkinleştirme", ""}, " verildi. Örnek: --nicemli_etkinleştirme açık", "yüklenemedi", false},
		{"nicemli değersiz", []string{"--kip", "dosya", "--cihaz", "cpu", "--nicemli_etkinleştirme"}, "bir değer bekler", "yüklenemedi", false},
		{"kart", []string{"--kip", "dosya", "--cihaz", "kart"}, `bilinmeyen cihaz "kart"`, "DENEME", true},
		{"boş ad", []string{"--kip", "dosya", "--cihaz", ""}, `bilinmeyen cihaz ""`, "DENEME", true},
		{"gpu:7", []string{"--kip", "dosya", "--cihaz", "gpu:7"}, "gpu:7 cihazı kullanılamıyor; bu makinede kullanılabilir cihazlar: cpu, gpu:0", "DENEME", true},
		{"ekranlı, gpu", []string{"--kip", "canlı", "--ekran", "0", "--cihaz", "gpu"}, "--cihaz gpu ekranlı kipte verilmez", "EKRAN", true}, // kullanıcının yazdığı ad, motorun uzun adı değil
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

// Ekranlı kipte hesap cihazı (tasarım dost-sohbet-gpu T3, T5, T7): modeller ayarın `device`'ında kurulur; `session`
// bu makinenin değerlerini (`compute_devices`) ve kullanılan cihazı (`compute_device_in_use`) söyler; deneme kaydının
// başlığı ayardan sonra yazılır, cihaz motorun adıyla ve kartın satırıyla. Ayar `gpu` deyip kart yoksa işlemci.
func startScreenWithDevice(t *testing.T, device string, env []string) (screenServer, string) {
	t.Helper()
	dost := filepath.Join(t.TempDir(), "Dost")
	if err := os.MkdirAll(dost, 0o700); err != nil {
		t.Fatal(err)
	}
	settings := `{"v": 1, "language_model": "qwen3-4b-q4km", "speech_language": "tr", "system_prompt": {"source": "catalog"},
"context": 8192, "device": "` + device + `", "port": 47863, "machine": {"memory_bytes": 17179869184, "memory_source": "option"}}`
	if err := os.WriteFile(filepath.Join(dost, "ayar.json"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		t.Setenv(k, v)
	}
	record := filepath.Join(t.TempDir(), "deneme")
	if err := os.MkdirAll(record, 0o700); err != nil {
		t.Fatal(err)
	}
	return startProgram(t, chainProgram, "DİNLİYORUM", "Dost", "--kip", "canlı", "--ekran", "0", "--ayar",
		filepath.Join(dost, "ayar.json"), "--katalog", dostCatalog, "--deneme_klasörü", record), record
}

func sessionData(t *testing.T, s screenServer) map[string]any {
	t.Helper()
	first := next(t, s.stream(t, ""))
	if first.Type != "session" {
		t.Fatalf("ilk olay %s", first.Type)
	}
	return first.Data
}

func TestScreenLoadsOnTheSettingsDevice(t *testing.T) {
	s, record := startScreenWithDevice(t, "gpu", []string{"TR_GPU_SIM=1"})
	d := sessionData(t, s)
	if toString(d["compute_devices"]) != "[cpu gpu]" {
		t.Errorf("compute_devices %v, beklenen [cpu gpu]", d["compute_devices"])
	}
	if d["compute_device_in_use"] != "gpu" {
		t.Errorf("compute_device_in_use %v, beklenen gpu", d["compute_device_in_use"])
	}
	_ = record
}

// Deneme kaydının ilk satırı ayardan sonra yazılır (status'un deneme başlığı satırı): cihaz kullanılanın motor adı,
// kartta kartın satırı (adı, belleği, sürücü sürümü, `@`'nin yolu; kitaplık sürümü yok).
func TestScreenTrialHeaderHasTheSettingsDevice(t *testing.T) {
	s, record := startScreenWithDevice(t, "gpu", []string{"TR_GPU_SIM=1"})
	_ = sessionData(t, s)
	trialHeaderHas(t, record, `"cihaz":"gpu:0"`, `; @ vendor"`) // kartın adı makineye göre (G4'te gerçek kart)
}

func TestMissingGpuLoadsOnCpu(t *testing.T) {
	s, record := startScreenWithDevice(t, "gpu", []string{"CUDA_VISIBLE_DEVICES=", "TR_GPU_SIM=0"})
	d := sessionData(t, s)
	if strings.Contains(toString(d["compute_devices"]), "gpu") {
		t.Skip("bu makinede kart görünür kaldı (CUDA_VISIBLE_DEVICES boşken de); kartsız yol kartsız makinede sınanır")
	}
	if d["compute_device_in_use"] != "cpu" {
		t.Errorf("kart yokken compute_device_in_use %v, beklenen cpu", d["compute_device_in_use"])
	}
	trialHeaderHas(t, record, `"cihaz":"cpu"`, "")
}

// trialHeaderHas: deneme kaydının ilk satırında cihaz ve (verildiyse) kart satırı var; kart verilmediyse kart satırı yok.
func trialHeaderHas(t *testing.T, record, device, card string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(record, "turlar.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	head := strings.SplitN(string(b), "\n", 2)[0]
	if !strings.Contains(head, device) {
		t.Errorf("deneme başlığında %s yok: %s", device, head)
	}
	if card != "" && !strings.Contains(head, card) {
		t.Errorf("deneme başlığında %s yok: %s", card, head)
	}
	if card == "" && strings.Contains(head, "@ vendor") {
		t.Errorf("işlemcide kart satırı olmamalı: %s", head)
	}
}

func toString(v any) string {
	if l, ok := v.([]any); ok {
		parts := make([]string, len(l))
		for i, x := range l {
			parts[i], _ = x.(string)
		}
		return "[" + strings.Join(parts, " ") + "]"
	}
	s, _ := v.(string)
	return s
}

// Yalnız sohbette tur seslendirilmez (tasarım dost-sohbet-gpu S2; DENETİM 821 Ş1 (a)): ekranlı turun sessizliği turun
// başındaki `e.yalnız_sohbet`'tir ve `tur_işi`'ne geçer; `tur_işi` cümleleri `söyleyen(sessiz, …)`'den geçirir, sessizde
// kaydın seslendirilen cümlelerini boşaltır. `söyleyen`'in davranışı oturum_test.tr'de ("yalnız sohbet, ses"); ekranlı
// tur gerçek model ister, bağın kendisi kaynakta sınanır (TestChainModelCallsGoThroughOneGate gibi). Gerçek modelle
// sınaması devirde (`speaking` yok).
func TestChatOnlyTurnIsNotVoiced(t *testing.T) {
	src := readFile(t, chainProgram)
	between := func(from, to string) string {
		i, j := strings.Index(src, from), strings.Index(src, to)
		if i < 0 || j < i {
			t.Fatalf("%s: %q ile %q arası bulunamadı", chainProgram, from, to)
		}
		return src[i:j]
	}
	turn := between("TR_func ekran_turu(", "TR_func tur_ölçüleri(")
	for _, want := range []string{"TR_var sessiz = e.yalnız_sohbet", "tur_işi(a, t, konuşma, kesit, söz, t0, y, sessiz)"} {
		if !strings.Contains(turn, want) {
			t.Errorf("ekran_turu'nda %q yok: yalnız sohbet turun sesine geçmiyor", want)
		}
	}
	work := between("TR_func tur_işi(", "TR_func kimlikleri_yaz(")
	for _, want := range []string{"sessiz: TR_Bool = TR_false", "cevapla(z, yazı, t0, t, söyleyen(sessiz, (c) => konuşma.söyle(a.ses.TR_synthesize(c))), aktı)",
		"TR_if sessiz { t.seslendirilen = [] }"} {
		if !strings.Contains(work, want) {
			t.Errorf("tur_işi'nde %q yok", want)
		}
	}
	if strings.Count(src, "TR_synthesize(") != 1 {
		t.Errorf("%s: sesin üretildiği tek yer tur_işi olmalı (söyleyen'den geçen)", chainProgram)
	}
	if !strings.Contains(src, "e.yalnız_sohbet = etkin.yalnız_sohbet") {
		t.Errorf("%s: ekran açılışta ayarın chat_only'sini almıyor", chainProgram)
	}
}

// kur.tr'nin kartlı yolu (DENETİM 821 N3): kart varsa öntanımlı cihaz `öntanımlı_cihaz`'dan; `gpu` yazılırsa
// `machine.gpu` kartın özetiyle, `cpu` yazılırsa `machine.gpu` yok. Benzeticinin kartı 1 GiB'tir: Qwen3 4B sığmaz,
// `cpu` yazılır. Gerçek kartlı makinede (G4) sonuç kartın belleğine bağlıdır; orada yalnız tutarlılık sınanır.
func TestSetupWritesTheDefaultDeviceWithACard(t *testing.T) {
	t.Setenv("TR_GPU_SIM", "1")
	probe := filepath.Join(t.TempDir(), "kart.tr")
	if err := os.WriteFile(probe, []byte("# TR 4\nTR_print(TR_device_info(\"gpu:0\").TR_name)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(trlangBinary(t), probe)
	cmd.Dir = reporoot.Path(".")
	name, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kartın adı okunamadı: %v\n%s", err, name)
	}
	settings := filepath.Join(t.TempDir(), "Dost", "ayar.json")
	run := exec.Command(trlangBinary(t), setupCommand, "--katalog", dostCatalog, "--ayar", settings, "--toplam_bellek", "34359738368", "--ayarı_yaz")
	run.Dir = reporoot.Path(".")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("kur.tr: %v\n%s", err, out)
	}
	b, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	var j struct {
		Device  string `json:"device"`
		Machine struct {
			GPU *struct {
				Name   string `json:"name"`
				Matmul string `json:"matmul"`
				Memory int64  `json:"memory_bytes"`
			} `json:"gpu"`
		} `json:"machine"`
	}
	if err := json.Unmarshal(b, &j); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(name)) == "TR_GPU_SIM" && j.Device != "cpu" {
		t.Errorf("benzeticinin 1 GiB'lik kartına Qwen3 4B sığmaz, device cpu olmalı: %s", b)
	}
	switch j.Device {
	case "gpu":
		if j.Machine.GPU == nil || j.Machine.GPU.Matmul != "vendor" || j.Machine.GPU.Memory <= 0 {
			t.Errorf("device gpu iken machine.gpu kartın özeti olmalı: %s", b)
		}
	case "cpu":
		if j.Machine.GPU != nil {
			t.Errorf("device cpu iken machine.gpu olmamalı: %s", b)
		}
	default:
		t.Errorf("device %q", j.Device)
	}
}
