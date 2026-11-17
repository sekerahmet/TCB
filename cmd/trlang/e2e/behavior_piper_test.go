package e2e

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/onnx"
	"github.com/sekerahmet/TCB/internal/compute/onnx/onnxtest"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Piper sesinin uçtan uca testleri (s1-piper A2–A5). Ses Go'da kurulur: tests/data/vits_piper_tiny'nin
// Hugging Face ağırlıkları (transformers'ın kurduğu, Piper biçimli küçük model) Piper'ın dışa vericisinin
// yazdığı adlar ve biçimle ONNX'e yazılır: `sid` gömmesi, `[h, h, 1]` dikkat evrişimleri, `gamma`/`beta`,
// spline akışlarının tek sıraları (2k − 1; ters yönde atılan 1. akış yazılmaz), bağlaşımların çift
// sıraları (2k), ağırlık normu birleşmiş ve adsız (`onnx::Conv_*`, onu kullanan düğümle) WaveNet
// evrişimleri, `exp(-logs)` olarak katlanmış öteleme ölçeği ve çizgenin sabitleri. Piper'ın kodu
// kullanılmaz (GPL-3). TR o dosyayı Piper sesi olarak açar; dalgası transformers'ın başvurusuyla kıyaslanır.

const piperTiny = "tests/data/vits_piper_tiny"

// stTensor safetensors dosyasındaki bir tensördür, float32'ye açılmış.
type stTensor struct {
	shape []int64
	data  []float32
}

// readSafetensorsF16 küçük modelin float16 safetensors dosyasını okur.
func readSafetensorsF16(t *testing.T, path string) map[string]stTensor {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := binary.LittleEndian.Uint64(b)
	var head map[string]json.RawMessage
	if err := json.Unmarshal(b[8:8+n], &head); err != nil {
		t.Fatal(err)
	}
	data := b[8+n:]
	out := map[string]stTensor{}
	for name, raw := range head {
		if name == "__metadata__" {
			continue
		}
		var e struct {
			Dtype   string  `json:"dtype"`
			Shape   []int64 `json:"shape"`
			Offsets [2]int  `json:"data_offsets"`
		}
		if err := json.Unmarshal(raw, &e); err != nil || e.Dtype != "F16" {
			t.Fatalf("%s: %v %s", name, err, e.Dtype)
		}
		v := make([]float32, (e.Offsets[1]-e.Offsets[0])/2)
		for i := range v {
			v[i] = halfToFloat(binary.LittleEndian.Uint16(data[e.Offsets[0]+2*i:]))
		}
		out[name] = stTensor{shape: e.Shape, data: v}
	}
	return out
}

// halfToFloat IEEE 754 yarım duyarlıklı sayıyı float32'ye kayıpsız açar.
func halfToFloat(h uint16) float32 {
	sign, exp, frac := uint32(h>>15)<<31, uint32(h>>10)&0x1f, uint32(h)&0x3ff
	switch {
	case exp == 0 && frac == 0:
		return math.Float32frombits(sign)
	case exp == 0: // alt normal
		e := uint32(127 - 15 + 1)
		for frac&0x400 == 0 {
			frac <<= 1
			e--
		}
		return math.Float32frombits(sign | e<<23 | (frac&0x3ff)<<13)
	case exp == 0x1f:
		return math.Float32frombits(sign | 0xff<<23 | frac<<13)
	}
	return math.Float32frombits(sign | (exp+127-15)<<23 | frac<<13)
}

// foldWeightNorm `g · v / ‖v‖`'dir (PyTorch weight_norm, eksen 0); Piper'ın dışa vericisi bunu katlar.
func foldWeightNorm(g, v stTensor) stTensor {
	out := stTensor{shape: v.shape, data: make([]float32, len(v.data))}
	row := len(v.data) / int(v.shape[0])
	for o := range int(v.shape[0]) {
		s := 0.0
		for _, x := range v.data[o*row : (o+1)*row] {
			s += float64(x) * float64(x)
		}
		k := float64(g.data[o]) / math.Sqrt(s)
		for i := o * row; i < (o+1)*row; i++ {
			out.data[i] = float32(float64(v.data[i]) * k)
		}
	}
	return out
}

// piperRule Hugging Face adından Piper adına bir çeviridir; to'daki {2k}, {2k-1} sıra dönüşümleridir.
type piperRule struct {
	re *regexp.Regexp
	to string
}

var piperRules = []piperRule{
	{regexp.MustCompile(`^text_encoder\.embed_tokens\.weight$`), "sid"},
	{regexp.MustCompile(`^text_encoder\.encoder\.layers\.(\d+)\.attention\.([qkv])_proj\.(\w+)$`), "enc_p.encoder.attn_layers.$1.conv_$2.$3"},
	{regexp.MustCompile(`^text_encoder\.encoder\.layers\.(\d+)\.attention\.out_proj\.(\w+)$`), "enc_p.encoder.attn_layers.$1.conv_o.$2"},
	{regexp.MustCompile(`^text_encoder\.encoder\.layers\.(\d+)\.attention\.(emb_rel_[kv])$`), "enc_p.encoder.attn_layers.$1.$2"},
	{regexp.MustCompile(`^text_encoder\.encoder\.layers\.(\d+)\.layer_norm\.weight$`), "enc_p.encoder.norm_layers_1.$1.gamma"},
	{regexp.MustCompile(`^text_encoder\.encoder\.layers\.(\d+)\.layer_norm\.bias$`), "enc_p.encoder.norm_layers_1.$1.beta"},
	{regexp.MustCompile(`^text_encoder\.encoder\.layers\.(\d+)\.final_layer_norm\.weight$`), "enc_p.encoder.norm_layers_2.$1.gamma"},
	{regexp.MustCompile(`^text_encoder\.encoder\.layers\.(\d+)\.final_layer_norm\.bias$`), "enc_p.encoder.norm_layers_2.$1.beta"},
	{regexp.MustCompile(`^text_encoder\.encoder\.layers\.(\d+)\.feed_forward\.(.*)$`), "enc_p.encoder.ffn_layers.$1.$2"},
	{regexp.MustCompile(`^text_encoder\.project\.(.*)$`), "enc_p.proj.$1"},
	{regexp.MustCompile(`^duration_predictor\.conv_pre\.(.*)$`), "dp.pre.$1"},
	{regexp.MustCompile(`^duration_predictor\.conv_proj\.(.*)$`), "dp.proj.$1"},
	{regexp.MustCompile(`^duration_predictor\.conv_dds\.(.*)$`), "dp.convs.{dds:$1}"},
	{regexp.MustCompile(`^duration_predictor\.flows\.0\.translate$`), "dp.flows.0.m"},
	{regexp.MustCompile(`^duration_predictor\.flows\.(\d+)\.conv_pre\.(.*)$`), "dp.flows.{2k-1:$1}.pre.$2"},
	{regexp.MustCompile(`^duration_predictor\.flows\.(\d+)\.conv_proj\.(.*)$`), "dp.flows.{2k-1:$1}.proj.$2"},
	{regexp.MustCompile(`^duration_predictor\.flows\.(\d+)\.conv_dds\.(.*)$`), "dp.flows.{2k-1:$1}.convs.{dds:$2}"},
	{regexp.MustCompile(`^flow\.flows\.(\d+)\.conv_pre\.(.*)$`), "flow.flows.{2k:$1}.pre.$2"},
	{regexp.MustCompile(`^flow\.flows\.(\d+)\.conv_post\.(.*)$`), "flow.flows.{2k:$1}.post.$2"},
	{regexp.MustCompile(`^flow\.flows\.(\d+)\.wavenet\.(\w+)\.(\d+)\.bias$`), "flow.flows.{2k:$1}.enc.$2.$3.bias"},
	{regexp.MustCompile(`^decoder\.upsampler\.(.*)$`), "dec.ups.$1"},
	{regexp.MustCompile(`^decoder\.(.*)$`), "dec.$1"},
}

var (
	piperIndex = regexp.MustCompile(`\{(2k|2k-1|dds):([^}]*)\}`)
	ddsNames   = strings.NewReplacer("convs_dilated", "convs_sep", "convs_pointwise", "convs_1x1", ".weight", ".gamma", ".bias", ".beta")
)

// piperName Hugging Face adının Piper'daki adıdır; "" yazılmayanlardır (arka kodlayıcı, arka akışlar,
// ters yönde atılan 1. spline akışı).
func piperName(hf string) string {
	if strings.HasPrefix(hf, "posterior_encoder.") || strings.HasPrefix(hf, "duration_predictor.post_") ||
		strings.HasPrefix(hf, "duration_predictor.flows.1.") {
		return ""
	}
	for _, r := range piperRules {
		if !r.re.MatchString(hf) {
			continue
		}
		name := r.re.ReplaceAllString(hf, r.to)
		return piperIndex.ReplaceAllStringFunc(name, func(m string) string {
			p := piperIndex.FindStringSubmatch(m)
			k, _ := strconv.Atoi(p[2])
			switch p[1] {
			case "2k":
				return strconv.Itoa(2 * k)
			case "2k-1":
				return strconv.Itoa(2*k - 1)
			}
			// Ayrık evrişim: norm katmanında weight/bias gamma/beta olur, evrişimde kalır.
			if strings.HasPrefix(p[2], "norms_") {
				return ddsNames.Replace(p[2])
			}
			return strings.NewReplacer("convs_dilated", "convs_sep", "convs_pointwise", "convs_1x1").Replace(p[2])
		})
	}
	panic("Piper adı yok: " + hf)
}

func f32Tensor(name string, x stTensor) onnxtest.Tensor {
	return onnxtest.Tensor{Name: name, Dims: x.shape, Type: onnx.Float, Raw: onnxtest.F32(x.data...)}
}

// piperEdit küçük sesin bir değişkesidir: cfg `.onnx.json`'u değiştirir, extra dosyaya eklenen başka
// ağırlıklardır, scale Piper adıyla bir ağırlığı çarpar.
type piperEdit struct {
	cfg   func(map[string]any)
	extra []onnxtest.Tensor
	scale map[string]float32
	// torch26 torch 2.6'nın dışa verilişidir: `Exp` katlanmaz, girdisi adsız `-logs`'tur; adsız sabitin
	// düğümü `/` ile başlamayabilir (`Gather_3060`) [ölçüldü: tasima/a5/kucuk_piper2.py, 2026-11-17].
	torch26 bool
}

// piperVoice küçük modeli dir/ses.onnx ve dir/ses.onnx.json olarak Piper'ın biçiminde yazar.
func piperVoice(t *testing.T, dir string, edit piperEdit) {
	t.Helper()
	w := readSafetensorsF16(t, reporoot.Path(piperTiny, "model.safetensors"))
	names := make([]string, 0, len(w))
	for n := range w {
		names = append(names, n)
	}
	slices.Sort(names)
	b := &onnxtest.Builder{}
	add := func(x onnxtest.Tensor, v stTensor) {
		if k, ok := edit.scale[x.Name]; ok {
			for i := range v.data {
				v.data[i] *= k
			}
			x.Raw = onnxtest.F32(v.data...)
		}
		b.Initializer(x)
	}
	anon := 8168
	for _, hf := range names {
		x := w[hf]
		switch {
		case strings.HasPrefix(hf, "flow.flows.") && strings.HasSuffix(hf, ".weight_v"):
			// Ağırlık normu katlanır ve ağırlık adsız kalır; adını onu kullanan Conv düğümü taşır.
			base := strings.TrimSuffix(hf, ".weight_v")
			p := strings.Split(base, ".") // flow.flows.<k>.wavenet.<katman>.<j>
			k, _ := strconv.Atoi(p[2])
			node := fmt.Sprintf("/flow/flows.%d/enc/%s.%s/Conv", 2*k, p[4], p[5])
			id := fmt.Sprintf("onnx::Conv_%d", anon)
			anon += 3
			b.Node(onnxtest.Node{Name: node, Op: "Conv", Inputs: []string{node + "_input", id, piperName(base + ".bias")}, Outputs: []string{node + "_output_0"}})
			v := foldWeightNorm(w[base+".weight_g"], x)
			add(f32Tensor(id, v), v)
			continue
		case strings.HasSuffix(hf, ".weight_g") || strings.HasSuffix(hf, ".weight_v"):
			continue // arka kodlayıcının normlu ağırlıkları: dışa verilmez
		case hf == "duration_predictor.flows.0.log_scale" && edit.torch26:
			e := stTensor{shape: x.shape, data: make([]float32, len(x.data))}
			for i, v := range x.data {
				e.data[i] = -v
			}
			b.Node(onnxtest.Node{Name: "/dp/flows.0/Exp", Op: "Exp", Inputs: []string{"onnx::Exp_6298"}, Outputs: []string{"/dp/flows.0/Exp_output_0"}})
			add(f32Tensor("onnx::Exp_6298", e), e)
			continue
		case hf == "duration_predictor.flows.0.log_scale":
			e := stTensor{shape: x.shape, data: make([]float32, len(x.data))}
			for i, v := range x.data {
				e.data[i] = float32(math.Exp(-float64(v)))
			}
			add(f32Tensor("/dp/flows.0/Exp_output_0", e), e)
			continue
		}
		name := piperName(hf)
		if name == "" {
			continue
		}
		if strings.Contains(hf, ".attention.") && strings.HasSuffix(hf, "_proj.weight") {
			x.shape = append(slices.Clone(x.shape), 1) // Piper'ın dikkati 1 çekirdekli evrişimdir
		}
		x.data = slices.Clone(x.data)
		add(f32Tensor(name, x), x)
	}
	// Çizgenin sabitleri: ağırlık değildir, kurulumda atılır.
	b.Node(onnxtest.Node{Name: "/dp/Split", Op: "Split", Inputs: []string{"y", "onnx::Split_77"}})
	b.Initializer(onnxtest.Tensor{Name: "onnx::Split_77", Dims: []int64{2}, Type: onnx.Int64, Int64s: []int64{1, 1}})
	b.Initializer(onnxtest.Tensor{Name: "/Constant_output_0", Dims: []int64{1}, Type: onnx.Int64, Int64s: []int64{2}})
	b.Initializer(onnxtest.Tensor{Name: "6549", Type: onnx.Float, Raw: onnxtest.F32(0.5)})
	if edit.torch26 {
		b.Node(onnxtest.Node{Name: "Gather_3060", Op: "Gather", Inputs: []string{"z", "onnx::Gather_3059"}})
		b.Initializer(onnxtest.Tensor{Name: "onnx::Gather_3059", Type: onnx.Int64, Int64s: []int64{1}})
	}
	for _, e := range edit.extra {
		b.Initializer(e)
	}
	writeBytes(t, filepath.Join(dir, "ses.onnx"), b.Bytes())
	writeBytes(t, filepath.Join(dir, "ses.onnx.json"), piperConfig(t, edit.cfg))
}

// piperConfig `.onnx.json`'u Piper'ın alanlarıyla kurar: fonem kimlikleri küçük modelin sözlüğüdür.
func piperConfig(t *testing.T, edit func(map[string]any)) []byte {
	t.Helper()
	raw, err := os.ReadFile(reporoot.Path(piperTiny, "vocab.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vocab map[string]int
	if err := json.Unmarshal(raw, &vocab); err != nil {
		t.Fatal(err)
	}
	ids := map[string][]int{}
	for s, i := range vocab {
		ids[s] = []int{i}
	}
	cfg := map[string]any{
		"audio": map[string]any{"sample_rate": 22050, "quality": "medium"}, "espeak": map[string]any{"voice": "tr"},
		"inference":      map[string]any{"noise_scale": 0.667, "length_scale": 1, "noise_w": 0.8},
		"phoneme_id_map": ids, "num_speakers": 1, "language": map[string]any{"code": "tr_TR", "family": "tr"},
	}
	if edit != nil {
		edit(cfg)
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// piperTests Piper sesinin TR testleridir; ses ses.onnx'tir, başvuru küçük modelinkidir.
const piperTests = `TR_import "tr/audio"
TR_import "tr/vits"

TR_const _SES = "ses.onnx"

TR_func başvuru(durum: TR_String) -> TR_Dict[TR_String, TR_Tensor] { TR_return TR_load_tensors("BASVURU/dizi_\(durum)_f32.safetensors") }

# dalga başvurunun iki gürültüsüyle üretir ve normlamadan önceki dalgayı verir (A5: kıyas normlanmamış dalgayla).
TR_func dalga(ses: TR_Vits, r: TR_Dict[TR_String, TR_Tensor], çarpan: TR_Float) -> TR_Tensor {
    TR_var kimlikler = r["ids"].TR_tolist
    TR_var ç = ses._üret(kimlikler, r["noise_duration"].TR_reshape([1, 2, kimlikler.TR_length]) * çarpan, r["noise_prior"] * çarpan, TR_none)
    TR_assert_equal(r["duration"].TR_tolist, ç.süreler)
    TR_return ç.dalga
}

# Sınır MMS kıyasınınki (tests/vits_model_test.tr): transformers'ın bu modelde float32 ile float64 farkı 2,1·10⁻⁵.
TR_test "Piper adlı küçük ses Hugging Face'in dalgasını verir, gürültülü ve gürültüsüz" {
    TR_var ses = TR_pretrained_vits(_SES)
    TR_var r = başvuru("sessiz")
    TR_assert(TR_all_close(dalga(ses, r, 0.0), r["waveform"], TR_rtol: 0.0, TR_atol: 1e-4))
    TR_var g = başvuru("gurultulu")
    TR_assert(TR_all_close(dalga(ses, g, 1.0), g["waveform"], TR_rtol: 0.0, TR_atol: 1e-4))
}

TR_test "Piper sesi açılır, ses_dili tr, örnek hızı .onnx.json'dan" {
    TR_var ses = TR_pretrained_vits(_SES)
    TR_assert_equal("tr", ses.TR_voice_language)
    TR_assert_equal(22050, ses._örnek_hızı)
}

TR_test "Piper iç girişi 22 050 Hz verir, tepe 1, aynı kimlikler aynı ses" {
    TR_var ses = TR_pretrained_vits(_SES)
    TR_var k = başvuru("sessiz")["ids"].TR_tolist
    TR_var a = ses._konuş([k])
    TR_assert_equal(22050, a.TR_sample_rate)
    TR_assert_equal(1.0, TR_max(TR_abs(a.TR_samples)).TR_tolist)
    TR_assert_equal(a.TR_samples.TR_tolist, ses._konuş([k]).TR_samples.TR_tolist)
    TR_var iki = ses._konuş([k, [], k])
    TR_assert_equal(2 * a.TR_samples.TR_shape[1], iki.TR_samples.TR_shape[1])
    TR_var d = ses._üret(k, TR_randn([1, 2, k.TR_length], TR_seed: 0), TR_none, TR_none).dalga
    TR_assert(TR_all_close(a.TR_samples.TR_reshape([d.TR_shape[0]]), d / TR_max(TR_abs(d)).TR_tolist, TR_rtol: 0.0, TR_atol: 1e-6))
}

TR_test "Piper fonem dizisi Piper'ın kimliklerini verir: ^, her fonemden sonra _, $" {
    TR_var ses = TR_pretrained_vits(_SES)
    TR_assert_equal([1, 0, 12, 0, 21, 0, 18, 0, 2], ses._kimlikler("lɾt"))
    TR_assert_equal([1, 0, 2], ses._kimlikler(""))
    TR_assert_equal("H7567", TR_error_code { ses._kimlikler("lç") })
}

TR_test "Piper sesinin sese_çevir'i fonem çevirici olmadan H7559 verir" {
    TR_assert_equal("H7559", TR_error_code { TR_pretrained_vits(_SES).TR_synthesize("merhaba") })
}
`

// Kural (s1-piper A2–A5): TR bir Piper sesini (`.onnx` ve `.onnx.json`) doğrudan açar; aynı fonem
// kimlikleriyle dalgası Hugging Face'in aynı ağırlıklı modelininkinden en çok 1·10⁻⁴ uzaktır ve süreler
// aynıdır; iç giriş cümle başına tepe 1'e normlar; dil `.onnx.json`'dan okunur.
// İki dışa veriliş: gerçek seslerinki (torch 2.0) ve torch 2.6'nınki.
func TestPiperVoiceMatchesTransformers(t *testing.T) {
	for _, torch26 := range []bool{false, true} {
		t.Run(fmt.Sprintf("torch26=%v", torch26), func(t *testing.T) {
			dir := t.TempDir()
			piperVoice(t, dir, piperEdit{torch26: torch26})
			ref := filepath.ToSlash(reporoot.Path(piperTiny, "reference"))
			writeTR(t, dir, "piper_test.tr", strings.ReplaceAll(piperTests, "BASVURU", ref))
			r := tr(t, dir, "test", "piper_test.tr")
			if r.Code != 0 || !strings.Contains(r.Stdout, "5 test, 5 geçti, 0 kaldı") {
				t.Fatalf("Piper testleri geçmedi (çıkış %d):\n%s%s", r.Code, r.Stdout, r.Stderr)
			}
			t.Log(r.Stdout)
		})
	}
}

// Kural: kıyas ağırlıktaki farkı görür. Dosyanın çevrilen her türünden bir ağırlık değişince (adsız
// WaveNet evrişimi, katlanmış ölçek, dikkat evrişimleri, HiFi-GAN bloğu) süreler değişir ya da dalga
// başvurudan 1·10⁻⁴'ten çok uzaklaşır: ad çevirisi yanlış ağırlığı seçseydi test bunu görürdü. Süre
// değişince başvurunun gürültüsü yeni çerçeve sayısına uymaz; o durumda dalga kurulmaz, sonuç "süre"dir.
func TestPiperVoiceComparisonSeesWeights(t *testing.T) {
	ref := filepath.ToSlash(reporoot.Path(piperTiny, "reference"))
	program := `TR_import "tr/vits"
TR_var r = TR_load_tensors("BASVURU/dizi_sessiz_f32.safetensors")
TR_var k = r["ids"].TR_tolist
TR_var ses = TR_pretrained_vits("ses.onnx")
TR_var süreler = TR_Tensor[TR_Int](TR_ceil(TR_exp(ses._üret(k, r["noise_duration"].TR_reshape([1, 2, k.TR_length]) * 0.0, TR_none, TR_none).log_süre))).TR_tolist
TR_if süreler != r["duration"].TR_tolist { TR_print("süre") } TR_else {
    TR_var ç = ses._üret(k, r["noise_duration"].TR_reshape([1, 2, k.TR_length]) * 0.0, r["noise_prior"] * 0.0, TR_none)
    TR_print(TR_if TR_all_close(ç.dalga, r["waveform"], TR_rtol: 0.0, TR_atol: 1e-4) { "aynı" } TR_else { "dalga" })
}
`
	for _, c := range []struct {
		name  string
		scale float32
	}{
		{"", 1}, // değişmemiş ses: denetimin kendisi
		// Dikkatin %1'i dalgada 1·10⁻⁴'ün altında kalır (ölçüldü: conv_v × 1,01 "aynı"); 2 kat görünür.
		{"onnx::Conv_8168", 1.01}, {"/dp/flows.0/Exp_output_0", 1.01}, {"enc_p.encoder.attn_layers.0.conv_v.weight", 2},
		{"enc_p.encoder.attn_layers.0.conv_q.weight", 2}, {"dec.resblocks.1.convs1.0.weight", 1.01},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			piperVoice(t, dir, piperEdit{scale: map[string]float32{c.name: c.scale}})
			writeTR(t, dir, "kiyas.tr", strings.ReplaceAll(program, "BASVURU", ref))
			r := tr(t, dir, "run", "kiyas.tr")
			got, want := strings.TrimSpace(r.Stdout), "aynı"
			if c.name != "" {
				if got == "süre" || got == "dalga" {
					t.Logf("%s × %g: %s", c.name, c.scale, got)
					return
				}
				want = "süre ya da dalga"
			}
			if got != want {
				t.Errorf("%s × %g: kıyas %q dedi, beklenen %s\n%s", c.name, c.scale, got, want, r.Stderr)
			}
		})
	}
}

// Kural (s1-piper A2): Piper sesinin dosya ve ayar hataları kodu ve dosyanın adıyla söylenir.
func TestPiperVoiceErrorsAreNamed(t *testing.T) {
	for _, c := range []struct {
		name, want string
		says       []string
		cfg        func(map[string]any)
		extra      []onnxtest.Tensor
		noJSON     bool
	}{
		{name: "onnx.json yok", want: "H7558", says: []string{"ses.onnx.json"}, noJSON: true},
		{name: "inference yok", want: "H7551", says: []string{"ses.onnx.json", "inference"}, cfg: func(m map[string]any) { delete(m, "inference") }},
		{name: "iki konuşmacı", want: "H7557", says: []string{"num_speakers"}, cfg: func(m map[string]any) { m["num_speakers"] = 2 }},
		{name: "tanınmayan ağırlık", want: "H7556", says: []string{"dec.fazla.weight"},
			extra: []onnxtest.Tensor{{Name: "dec.fazla.weight", Dims: []int64{1}, Type: onnx.Float, Raw: onnxtest.F32(1)}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			piperVoice(t, dir, piperEdit{cfg: c.cfg, extra: c.extra})
			if c.noJSON {
				if err := os.Remove(filepath.Join(dir, "ses.onnx.json")); err != nil {
					t.Fatal(err)
				}
			}
			writeTR(t, dir, "kod.tr", "TR_import \"tr/vits\"\nTR_print(TR_error_code { TR_pretrained_vits(\"ses.onnx\") })\n")
			if k := tr(t, dir, "run", "kod.tr"); strings.TrimSpace(k.Stdout) != c.want {
				t.Fatalf("kod %q, beklenen %s\n%s", k.Stdout, c.want, k.Stderr)
			}
			writeTR(t, dir, "ileti.tr", "TR_import \"tr/vits\"\nTR_pretrained_vits(\"ses.onnx\")\n")
			s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "ileti.tr"))
			for _, w := range c.says {
				if !strings.Contains(s.Stdout+s.Stderr, w) {
					t.Errorf("ileti %q demiyor:\n%s%s", w, s.Stdout, s.Stderr)
				}
			}
		})
	}
}
