package e2e

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// llama32Divisors Llama-3.2-1B-Instruct `Q8_0` ve `Q4_K_M` dosyalarının `rope_freqs.weight`'idir
// (bartowski, 2026-11-05): ilk 15 frekans değişmez, son 14'ü 32'ye bölünür, üçü aradadır.
var llama32Divisors = []float32{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1.6513292789459229, 3.2922627925872803,
	9.666730880737305, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}

// ropeConfig Llama 3.2'nin RoPE'siyle küçük bir modeldir: bakış boyutu 64, taban 500 000.
var ropeConfig = tinyConfig{arch: "llama", vocab: 48, dim: 64, ff: 96, layers: 1, heads: 1, kv: 1, headDim: 64, context: 8192, tied: true, theta: 500000}

// llamaRopeRow llama.cpp'nin bir konumdaki açılarıdır: `ggml_rope_cache_init` (ops.cpp) float32'de
// `theta *= theta_scale` yürür, her frekansı `theta/ff` böler; `theta_scale` `powf(freq_base, -2.0f/n_dims)`dir.
func llamaRopeRow(position int, base float32, dims int, divisors []float32) (cos, sin []float64) {
	scale := float32(math.Pow(float64(base), float64(float32(-2)/float32(dims))))
	theta := float32(position)
	for i := range dims / 2 {
		ff := float32(1)
		if divisors != nil {
			ff = divisors[i]
		}
		cos = append(cos, math.Cos(float64(theta/ff)))
		sin = append(sin, math.Sin(float64(theta/ff)))
		theta *= scale
	}
	return cos, sin
}

// ropeFile dosyayı rope_freqs.weight'le ya da onsuz yazar.
func ropeFile(c tinyConfig, tensor func(b *gguftest.Builder)) []byte {
	b := c.ggufFile(c.randomWeights(5))
	if tensor != nil {
		tensor(b)
	}
	return b.Bytes()
}

func divisorTensor(v []float32) func(b *gguftest.Builder) {
	return func(b *gguftest.Builder) {
		b.Tensor("rope_freqs.weight", gguf.TypeF32, []uint64{uint64(len(v))}, floatBytes(v))
	}
}

// runRope programı m.gguf ile koşar, çıktısını verir.
func runRope(t *testing.T, dir, program string) string {
	t.Helper()
	writeTR(t, dir, "p.tr", "TR_import \"tr/pretrained\"\n"+program)
	s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "p.tr"))
	if s.Code != 0 {
		t.Fatalf("TR çıkış %d:\n%s\n%s", s.Code, s.Stdout, s.Stderr)
	}
	return s.Stdout
}

// Kural (15.55): dosyanın frekans bölenleri uygulanır, her konumda ve her boyutta: tablo llama.cpp'nin
// formülüdür. Bit değil tolerans: TR konumu frekansla çarpar, llama.cpp frekansı adım adım yürür; fark
// konumla büyür [hesap: float32'de frekansın göreli hatası 2⁻²³ · adım sayısı]. Bölensiz tablo bu
// toleransın çok dışında kalır, kısa konumda da.
func TestGGUFRopeFreqsMatchLlamaCpp(t *testing.T) {
	dir := t.TempDir()
	writeBytes(t, filepath.Join(dir, "m.gguf"), ropeFile(ropeConfig, divisorTensor(llama32Divisors)))
	positions := []int{1, 31, 1024, 8191}
	out := runRope(t, dir, `TR_var m = TR_pretrained_gguf("m.gguf")
TR_for p TR_in [1, 31, 1024, 8191] {
    TR_print(m.TR_cos_table[p].TR_tolist)
    TR_print(m.TR_sin_table[p].TR_tolist)
}
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2*len(positions) {
		t.Fatalf("%d satır: %s", len(lines), out)
	}
	for k, p := range positions {
		var got [2][]float64
		for j := range got {
			if err := json.Unmarshal([]byte(lines[2*k+j]), &got[j]); err != nil {
				t.Fatalf("konum %d: %v: %s", p, err, lines[2*k+j])
			}
		}
		tolerance := 1e-6 + float64(p)*2e-7
		wantCos, wantSin := llamaRopeRow(p, ropeConfig.theta, ropeConfig.headDim, llama32Divisors)
		plainCos, plainSin := llamaRopeRow(p, ropeConfig.theta, ropeConfig.headDim, nil)
		plain := 0.0
		for i := range wantCos {
			if len(got[0]) != len(wantCos) || len(got[1]) != len(wantSin) {
				t.Fatalf("konum %d: %d, %d öğe", p, len(got[0]), len(got[1]))
			}
			if d := math.Abs(got[0][i] - wantCos[i]); d > tolerance {
				t.Errorf("konum %d boyut %d cos %v, llama.cpp %v", p, i, got[0][i], wantCos[i])
			}
			if d := math.Abs(got[1][i] - wantSin[i]); d > tolerance {
				t.Errorf("konum %d boyut %d sin %v, llama.cpp %v", p, i, got[1][i], wantSin[i])
			}
			plain = max(plain, math.Abs(got[0][i]-plainCos[i]), math.Abs(got[1][i]-plainSin[i]))
		}
		if plain < 10*tolerance {
			t.Errorf("konum %d: bölensiz tablodan fark %v, tolerans %v: test bölenleri ayırmıyor", p, plain, tolerance)
		}
	}
}

// Kural (15.55): tensör isteğe bağlıdır, yoksa bölen 1'dir: bölenleri 1 olan dosya tensörsüz dosyayla
// aynı tabloyu verir, `tr/layers`'ın tablosunu. Bölenler modelin çıktısını değiştirir ve alanlar
// çıktının tek kaynağıdır: tensörsüz modele aynı tablolar verilince logitler aynıdır.
func TestGGUFRopeFreqsAreOptional(t *testing.T) {
	dir := t.TempDir()
	c := ropeConfig
	ones := make([]float32, c.headDim/2)
	for i := range ones {
		ones[i] = 1
	}
	writeBytes(t, filepath.Join(dir, "yok.gguf"), ropeFile(c, nil))
	writeBytes(t, filepath.Join(dir, "bir.gguf"), ropeFile(c, divisorTensor(ones)))
	writeBytes(t, filepath.Join(dir, "var.gguf"), ropeFile(c, divisorTensor(llama32Divisors)))
	out := runRope(t, dir, `TR_import "tr/layers"
TR_var yok = TR_pretrained_gguf("yok.gguf")
TR_var bir = TR_pretrained_gguf("bir.gguf")
TR_var bölen = TR_pretrained_gguf("var.gguf")
TR_var t = TR_rotary_table(8192, 64, TR_rope_theta: 500000.0)
TR_print(TR_max(TR_abs(yok.TR_cos_table - t[0])).TR_tolist, TR_max(TR_abs(yok.TR_sin_table - t[1])).TR_tolist)
TR_print(TR_max(TR_abs(bir.TR_cos_table - t[0])).TR_tolist, TR_max(TR_abs(bir.TR_sin_table - t[1])).TR_tolist)
TR_var kimlikler = TR_Tensor([[(i * 7) % 48 TR_for i TR_in 0..256]])
TR_print(TR_max(TR_abs(yok.TR_forward(kimlikler) - bölen.TR_forward(kimlikler))).TR_tolist > 0.001)
yok.TR_cos_table = bölen.TR_cos_table
yok.TR_sin_table = bölen.TR_sin_table
TR_print(TR_max(TR_abs(yok.TR_forward(kimlikler) - bölen.TR_forward(kimlikler))).TR_tolist)
`)
	if want := "0.0 0.0\n0.0 0.0\ntrue\n0.0\n"; out != want {
		t.Errorf("çıktı %q; beklenen %q", out, want)
	}
}

// Kural (15.55, birinci süzgeç): bölenleri kullanılamayan dosya yükleme hatasıdır; yanlış tablo model
// hatasız kurulup yanlış metin üretmek demektir. Sıfır bölen sonsuz, NaN bölen NaN açı verirdi;
// çok küçük bölen de açıyı float32'den taşırır. 16 bitlik bölenler kabul edilir.
func TestGGUFRopeFreqsGates(t *testing.T) {
	c := ropeConfig
	with := func(i int, v float32) []float32 {
		out := append([]float32(nil), llama32Divisors...)
		out[i] = v
		return out
	}
	raw := func(kind gguf.TensorType, dims []uint64, data []byte) func(b *gguftest.Builder) {
		return func(b *gguftest.Builder) { b.Tensor("rope_freqs.weight", kind, dims, data) }
	}
	cases := []struct {
		name, want, says string
		tensor           func(b *gguftest.Builder)
	}{
		{"kısa", "H6698", "rope_freqs.weight", divisorTensor(llama32Divisors[:16])},
		{"uzun", "H6698", "rope_freqs.weight", divisorTensor(append(append([]float32(nil), llama32Divisors...), 1))},
		{"boş", "H6698", "rope_freqs.weight", raw(gguf.TypeF32, []uint64{0}, nil)},
		{"iki eksen", "H6698", "rope_freqs.weight", raw(gguf.TypeF32, []uint64{16, 2}, floatBytes(llama32Divisors))},
		{"sıfır", "H7041", "rope_freqs.weight", divisorTensor(with(3, 0))},
		{"eksi", "H7041", "rope_freqs.weight", divisorTensor(with(31, -32))},
		{"NaN", "H7041", "rope_freqs.weight", divisorTensor(with(0, float32(math.NaN())))},
		{"sonsuz", "H7041", "rope_freqs.weight", divisorTensor(with(17, float32(math.Inf(1))))},
		{"eksi sonsuz", "H7041", "rope_freqs.weight", divisorTensor(with(17, float32(math.Inf(-1))))},
		{"açı taşar", "H7041", "rope_freqs.weight", divisorTensor(with(0, 1e-42))},
		{"nicemli", "H7040", "q8", raw(gguf.TypeQ8_0, []uint64{32}, make([]byte, 34))},
		{"tam sayı", "H6674", "rope_freqs.weight", raw(gguf.TensorType(26), []uint64{32}, make([]byte, 128))},
	}
	dir := t.TempDir()
	for i, k := range cases {
		writeBytes(t, filepath.Join(dir, fmt.Sprintf("m%d.gguf", i)), ropeFile(c, k.tensor))
	}
	for i, k := range cases {
		t.Run(k.name, func(t *testing.T) {
			writeTR(t, dir, "kapi.tr", fmt.Sprintf("TR_import \"tr/pretrained\"\nTR_pretrained_gguf(\"m%d.gguf\")\n", i))
			s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "kapi.tr"))
			if !strings.Contains(s.Stderr+s.Stdout, k.says) {
				t.Errorf("ileti %q demiyor:\n%s%s", k.says, s.Stdout, s.Stderr)
			}
			got := runRope(t, dir, fmt.Sprintf("TR_print(TR_error_code { TR_pretrained_gguf(\"m%d.gguf\") })\n", i))
			if strings.TrimSpace(got) != k.want {
				t.Errorf("kod %q; beklenen %s", got, k.want)
			}
		})
	}
	half := func(bits func(float32) uint16) []byte {
		var out []byte
		for _, v := range llama32Divisors {
			out = binary.LittleEndian.AppendUint16(out, bits(v))
		}
		return out
	}
	bf16 := func(v float32) uint16 { return uint16(math.Float32bits(v) >> 16) }
	writeBytes(t, filepath.Join(dir, "bf.gguf"), ropeFile(c, raw(gguf.TypeBF16, []uint64{32}, half(bf16))))
	got := runRope(t, dir, `TR_print(TR_pretrained_gguf("bf.gguf").TR_cos_table.TR_shape)
`)
	if strings.TrimSpace(got) != "["+strconv.Itoa(c.context)+", 32]" {
		t.Errorf("bf16 bölen: %q", got)
	}
}
