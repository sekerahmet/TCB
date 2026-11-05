package ml

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

func TestTensorSaveLoad(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.ToSlash(filepath.Join(dir, "model.trt"))
	out := mustRun(t, `TR_var p = {"gömme": TR_Tensor([[0.1, 0.2], [0.3, 0.4]]), "sapma": TR_Tensor([1.5]), "ölçek": TR_sum(TR_Tensor([2.0, 3.0])), "boş": TR_zeros([0, 3])}
TR_save_tensors("`+fpath+`", p)
TR_var y = TR_load_tensors("`+fpath+`")
TR_print(y.TR_keys())
TR_print(y["gömme"].TR_tolist, y["sapma"].TR_tolist, y["ölçek"].TR_tolist, y["boş"].TR_shape)
TR_print(y["gömme"].TR_tolist[0][0] == 0.1, y["sapma"].TR_grad)
TR_var z = TR_load_tensors("`+fpath+`", TR_requires_grad: TR_true)
TR_backward(TR_sum(z["sapma"] * 2))
TR_print(z["sapma"].TR_grad.TR_tolist)`)
	want := "[\"gömme\", \"sapma\", \"ölçek\", \"boş\"]\n" +
		"[[0.1, 0.2], [0.3, 0.4]] [1.5] 5.0 [0, 3]\n" +
		"true none\n" +
		"[2.0]\n"
	if out != want {
		t.Fatalf("kaydet/yükle:\n%q\nbeklenen:\n%q", out, want)
	}

	// Biçim: başlık, sürüm ve tür alanı yerinde.
	data, err := os.ReadFile(filepath.FromSlash(fpath))
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:8]) != "TRTENSOR" || binary.LittleEndian.Uint16(data[8:10]) != 2 || binary.LittleEndian.Uint32(data[10:14]) != 4 {
		t.Fatalf("başlık hatalı: % x", data[:14])
	}

	write := func(ad string, b []byte) string {
		p := filepath.Join(dir, ad)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(p)
	}
	broken := append([]byte{}, data...)
	broken[len(broken)-10] ^= 0xff
	version := append([]byte{}, data...)
	version[8] = 9
	dtype := append([]byte{}, data...)
	// İlk girdinin tür baytı: başlık (14) + ad uzunluğu (4) + "gömme" (6 bayt).
	dtype[14+4+len("gömme")] = 200 // 1–12 tür kodudur (15.46)
	cases := []struct {
		fpath string
		want  messages.Code
	}{
		{write("eksik.trt", data[:len(data)-20]), messages.H6601},
		{write("bozuk.trt", broken), messages.H6627},
		{write("baslik.trt", []byte("merhaba dünya, bu bir tensör dosyası değil")), messages.H6610},
		{write("kisa.trt", []byte("TRT")), messages.H6610},
		{write("surum.trt", version), messages.H6611},
		{write("tur.trt", dtype), messages.H6616},
		{write("fazla.trt", append(append([]byte{}, data...), 0)), messages.H6628},
		{filepath.ToSlash(filepath.Join(dir, "TR_none.trt")), messages.H2107},
		{filepath.ToSlash(dir), messages.H6629},
	}
	for _, d := range cases {
		_, err := run(t, `TR_load_tensors("`+d.fpath+`")`)
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", filepath.Base(d.fpath), err, d.want)
		}
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`TR_save_tensors("` + fpath + `", {"a": 1})`, messages.H6608},
		{`TR_save_tensors("` + fpath + `", {1: TR_Tensor([1.0])})`, messages.H6606},
		{`TR_save_tensors("` + fpath + `", [TR_Tensor([1.0])])`, messages.H6605},
		{`TR_save_tensors("", {})`, messages.H6603},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// fileBytes girdileri tensör dosyası baytlarına çevirir (bulanıklaştırma tohumları için).
func fileBytes(tb testing.TB, inputs []fileEntry) []byte {
	tb.Helper()
	var b bytes.Buffer
	if err := writeEntries(&b, inputs); err != nil {
		tb.Fatal(err)
	}
	return b.Bytes()
}

// Güvenlik denetimi (2026-10-18), bulgu: sıfır uzunluklu bir eksen tensör dosyasındaki
// eleman sınırını (interp.MaxCollectionSize) devre dışı bırakıyordu. 126 baytlık bir dosya
// [2, 0], [1048576, 1, 0] ve [1048576, 0, 8192] şekilli tensörler bildirebiliyor, examples/tr/30_xor_agi.tr
// tarzı bir ileri geçiş (X @ W1 + b1 ... @ W2) 137 GB ayırmaya çalışıp süreci
// kurtarılamaz biçimde ("fatal error: out of memory") çökertiyordu.
func TestSaveZeroAxisLimit(t *testing.T) {
	bad := fileBytes(t, []fileEntry{
		{ad: "W1", shape: []int{2, 0}, dtype: tensor.Float32, data: []float32{}},
		{ad: "b1", shape: []int{1 << 20, 1, 0}, dtype: tensor.Float32, data: []float32{}},
		{ad: "W2", shape: []int{1 << 20, 0, 8192}, dtype: tensor.Float32, data: []float32{}},
	})
	if _, err := decodeFile(bytes.NewReader(bad), int64(len(bad))); err == nil || !messages.Has(err, messages.H6621) {
		t.Fatalf("sıfır eksenle dev şekil reddedilmeliydi: %v", err)
	}

	// Sınırın altındaki sıfır eksenli tensörler (ör. boş tampon) okunmaya devam eder.
	good := fileBytes(t, []fileEntry{
		{ad: "boş", shape: []int{0, 3}, dtype: tensor.Float32, data: []float32{}},
		{ad: "sınırda", shape: []int{0, 5000, 10000}, dtype: tensor.BFloat16, data: []float32{}},
	})
	g, err := decodeFile(bytes.NewReader(good), int64(len(good)))
	if err != nil || len(g) != 2 || g[1].shape[2] != 10000 {
		t.Fatalf("sınır içindeki sıfır eksenli tensörler okunmalı: %v %+v", err, g)
	}

	// Kullanıcının gördüğü hata: tensör_yükle açık bir hata verir, süreç çökmez.
	fpath := filepath.Join(t.TempDir(), "kotu.trt")
	if err := os.WriteFile(fpath, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = run(t, `TR_var p = TR_load_tensors("`+filepath.ToSlash(fpath)+`")`)
	if err == nil || !messages.Has(err, messages.H6621) {
		t.Fatalf("tensör_yükle açık bir hata vermeli: %v", err)
	}
}

// Güvenlik denetimi Y2: dosyanın bildirdiği şekiller tek tek sınır içinde olsa da
// işlemlerin çıktısı fiziksel belleği aşabilir ([46000000, 0] @ [0, 46000000] verisiz iki
// tensörden yaklaşık 7,5 PB ister). Çekirdek ayırmadan önce denetler; hata dene/yakala ile
// yakalanır, süreç çökmez.
func TestSavedShapesOutputLimited(t *testing.T) {
	file := fileBytes(t, []fileEntry{
		{ad: "A", shape: []int{46000000, 0}, dtype: tensor.Float32, data: []float32{}},
		{ad: "B", shape: []int{0, 46000000}, dtype: tensor.Float32, data: []float32{}},
	})
	fpath := filepath.Join(t.TempDir(), "sekiller.trt")
	if err := os.WriteFile(fpath, file, 0o644); err != nil {
		t.Fatal(err)
	}
	program := `TR_var p = TR_load_tensors("` + filepath.ToSlash(fpath) + `")
`
	output, err := run(t, program+`TR_try {
    p["A"] @ p["B"]
    TR_print("hata bekleniyordu")
} TR_catch h {
    TR_print(h.TR_code)
}`)
	if err != nil || output != "H3085\n" {
		t.Fatalf("hata kodu H3085 ile yakalanmalıydı: %q %v", output, err)
	}
	// Şekil ve bellek gösterilen hatanın metnindedir (yakalanan hatanın metni iletiyi kuran çağrıdır, 8.3).
	_, err = run(t, program+`TR_var c = p["A"] @ p["B"]`)
	shown := messages.ErrorIn(err, "en")
	for _, p := range []string{"the result tensor of shape [46000000, 46000000]", "PB of memory", "this machine"} {
		if !strings.Contains(shown, p) {
			t.Fatalf("gösterilen hatada %q yok: %s", p, shown)
		}
	}
}

// Kural (15.46): TR biçiminin tür kodları sabittir; yazılan dosyanın kodu ve verisi elle
// yazılmış baytlarla aynıdır, elle yazılmış dosya aynı türle ve değerle okunur. Gidiş-dönüş
// testi aynı tabloyla yazıp okuduğundan kodların yer değiştirmesini görmez; bu test görür.
func TestTensorFileTypeCodes(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		code  byte
		expr  string // tensör
		data  string // iki öğenin küçük uçlu baytları
		dtype string
		list  string
	}{
		{1, `TR_Tensor([1.5, -2.0])`, "0000c03f000000c0", "float32", "[1.5, -2.0]"},
		{2, `TR_Tensor([1.5, -2.0], TR_dtype: "bf16")`, "c03f00c0", "bf16", "[1.5, -2.0]"},
		{3, `TR_Tensor([1.5, -2.0], TR_dtype: "fp16")`, "003e00c0", "fp16", "[1.5, -2.0]"},
		{4, `TR_Tensor([1.5, -2.0], TR_dtype: "float64")`, "000000000000f83f00000000000000c0", "float64", "[1.5, -2.0]"},
		{5, `TR_Tensor([1, -2])`, "0100000000000000feffffffffffffff", "int64", "[1, -2]"},
		{6, `TR_Tensor([1, -2], TR_dtype: "int32")`, "01000000feffffff", "int32", "[1, -2]"},
		{7, `TR_Tensor([1, -2], TR_dtype: "int16")`, "0100feff", "int16", "[1, -2]"},
		{8, `TR_Tensor([1, -2], TR_dtype: "int8")`, "01fe", "int8", "[1, -2]"},
		{9, `TR_Tensor([1, 4000000000], TR_dtype: "uint32")`, "0100000000286bee", "uint32", "[1, 4000000000]"},
		{10, `TR_Tensor([1, 60000], TR_dtype: "uint16")`, "010060ea", "uint16", "[1, 60000]"},
		{11, `TR_Tensor([1, 200], TR_dtype: "uint8")`, "01c8", "uint8", "[1, 200]"},
		{12, `TR_Tensor([TR_true, TR_false])`, "0100", "bool", "[true, false]"},
	} {
		data, err := hex.DecodeString(c.data)
		if err != nil {
			t.Fatal(err)
		}
		// Tek girdili dosya: "t" adlı, [2] şekilli tensör.
		var b bytes.Buffer
		b.WriteString("TRTENSOR")
		b.Write([]byte{2, 0, 1, 0, 0, 0, 1, 0, 0, 0, 't', c.code, 1, 2, 0, 0, 0, 0, 0, 0, 0})
		b.Write(data)
		b.Write(binary.LittleEndian.AppendUint32(nil, crc32.ChecksumIEEE(b.Bytes())))
		want := b.Bytes()

		saved := filepath.ToSlash(filepath.Join(dir, c.dtype+"_yazilan.trt"))
		mustRun(t, `TR_save_tensors("`+saved+`", {"t": `+c.expr+`})`)
		got, err := os.ReadFile(filepath.FromSlash(saved))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: yazılan dosya\n% x\nbeklenen (kod %d)\n% x", c.dtype, got, c.code, want)
		}
		known := filepath.Join(dir, c.dtype+"_bilinen.trt")
		if err := os.WriteFile(known, want, 0o644); err != nil {
			t.Fatal(err)
		}
		out := mustRun(t, `TR_var t = TR_load_tensors("`+filepath.ToSlash(known)+`")["t"]
TR_print(t.TR_dtype, t.TR_tolist)`)
		if wantOut := c.dtype + " " + c.list + "\n"; out != wantOut {
			t.Errorf("kod %d okundu: %q, beklenen %q", c.code, out, wantOut)
		}
	}
}

// saveRace kaydetmenin kilidi bıraktığı anı yakalar: görev bekle()'de kilitsiz durur, kaydet()
// onu kilidi tutarak uyandırır; görev sırasını kilit bırakılınca alır ve son'a yazar. Önce büyük
// girdi gelir: kopya kilit dışına kayarsa görevin yazması son'un kopyasından önce olur.
const saveRace = `TR_var son = TR_Tensor([0.0])
TR_var d = {"ilk": TR_randn([4194304], TR_seed: 1), "son": son}
TR_var g = TR_async {
    bekle()
    son += 1.0
}
kaydet(yol, d)
g.TR_wait()
TR_print(son.TR_tolist)
TR_print(TR_load_tensors(yol)["son"].TR_tolist)
`

// Kural (10.3, DENETİM 166 Ş3): TR_save_tensors verinin kopyasını kilidi tutarken alır, kilidi
// dosyaya yazarken bırakır. Kilit bırakılınca çalışan görevin yazması dosyaya girmez.
func TestSaveCopiesBeforeReleasingTheLock(t *testing.T) {
	for _, name := range []string{"a.tensors", "a.safetensors"} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			m := interp.New()
			m.Out = &out
			parked, wake := make(chan struct{}), make(chan struct{})
			hook := func(fn interp.BuiltinFunc) *interp.BuiltinVal { return &interp.BuiltinVal{Name: "kanca", Fn: fn} }
			m.GlobalEnv.Define("yol", &interp.StringVal{Val: filepath.Join(t.TempDir(), name)}, true)
			m.GlobalEnv.Define("bekle", hook(func(y *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
				y.Blocking(func() {
					close(parked)
					<-wake
				})
				return interp.Null, nil
			}), true)
			m.GlobalEnv.Define("kaydet", hook(func(y *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
				y.Blocking(func() { <-parked })
				close(wake)
				return tensorSave(y, args, named)
			}), true)
			_, err := m.Run(trrun.Parse(t, saveRace))
			m.WaitAsync()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := out.String(), "[1.0]\n[0.0]\n"; got != want {
				t.Errorf("bellekteki ve dosyadaki son: %q, beklenen %q", got, want)
			}
		})
	}
}
