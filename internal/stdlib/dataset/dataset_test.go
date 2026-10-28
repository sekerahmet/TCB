package dataset_test

import (
	"bytes"
	"context"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/devtools/debugger"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/dataset"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// run TR kodunu tam dille (motorlar dahil) çalıştırır; çıktıyı ve VM'yi döndürür.
func run(t *testing.T, kd string) (string, *interp.VM, error) {
	t.Helper()
	o := trrun.Exec(t, kd)
	return o.Out, o.VM, o.Err
}

func mustRun(t *testing.T, kd string) (string, *interp.VM) {
	t.Helper()
	o := trrun.MustExec(t, kd)
	return o.Out, o.VM
}

// trPath bir yolu TR metin değişmezi içine yazılabilir biçime getirir.
func trPath(path string) string { return strings.ReplaceAll(filepath.ToSlash(path), `"`, `\"`) }

// buildDataset iki parçalı, değerleri konumunu söyleyen bir küme yazar.
func buildDataset(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	// Windows'ta açık dosyalar silinemez: geçici klasör silinmeden önce kaynaklar kapanır.
	t.Cleanup(func() { dataset.CloseAll(time.Second) })
	write := func(name string, v []int32) {
		var b bytes.Buffer
		if err := dataload.WriteNpyI32(&b, v); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, name), b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for p, n := range []int{300, 500} {
		v := make([]int32, n)
		for i := range v {
			v[i] = int32(p*10000 + i)
		}
		write([]string{"p0.npy", "p1.npy"}[p], v)
	}
	return d
}

func variable(t *testing.T, m *interp.VM, name string) interp.Value {
	t.Helper()
	v, ok := m.GlobalEnv.Get(name)
	if !ok {
		t.Fatalf("%s değişkeni yok", name)
	}
	return v
}

func tensorData(t *testing.T, v interp.Value) ([]float32, []int) {
	t.Helper()
	d, ok := v.(*ml.TensorValue)
	if !ok {
		t.Fatalf("tensör bekleniyordu, %T", v)
	}
	return d.Tensor().Data(), d.Tensor().Shape()
}

// Örnekleyici, aynı tohumla Go kütüphanesinin örnekleyicisiyle aynı toplu işleri verir;
// girdi [B, T], hedef bir kaydırılmış [B, T] tensördür.
func TestSamplerSameAsGo(t *testing.T) {
	d := buildDataset(t)
	_, m := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`")
TR_var ö = k.TR_sampler(TR_batch_size: 4, TR_context: 16, TR_seed: 7)
TR_var ilk = ö.TR_next()
TR_var ikinci = ö.TR_next()
`)
	gk, err := dataload.OpenDataset(d, dataload.DatasetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	o, err := gk.Sampler(dataload.SamplerOptions{B: 4, T: 16, Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	for _, name := range []string{"ilk", "ikinci"} {
		var ti dataload.Batch
		if err := o.Fill(context.Background(), &ti); err != nil {
			t.Fatal(err)
		}
		l := variable(t, m, name).(*interp.ListVal)
		if len(l.Elements) != 2 {
			t.Fatalf("[girdi, hedef] bekleniyordu: %s", l.Inspect())
		}
		g, gs := tensorData(t, l.Elements[0])
		h, hs := tensorData(t, l.Elements[1])
		if len(gs) != 2 || gs[0] != 4 || gs[1] != 16 || len(hs) != 2 || hs[0] != 4 || hs[1] != 16 {
			t.Fatalf("şekiller %v %v", gs, hs)
		}
		for i := 0; i < 4; i++ {
			for j := 0; j < 16; j++ {
				if g[i*16+j] != float32(ti.Input(i)[j]) || h[i*16+j] != float32(ti.Target(i)[j]) {
					t.Fatalf("%s: satır %d sütun %d farklı", name, i, j)
				}
			}
		}
	}
}

// Sıralı okuyucu tüm pencereleri sırayla verir, son toplu iş eksik satırlı olabilir,
// bitince yok döner ve kaynaklarını hemen kapatır.
func TestSequentialEndsWithNoneAndCloses(t *testing.T) {
	d := buildDataset(t)
	base := dataset.OpenResources()
	out, m := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`")
TR_var s = k.TR_sequential(TR_batch_size: 8, TR_context: 32)
TR_var n = 0
TR_var satır = 0
TR_var son = TR_none
TR_for {
    TR_var x = s.TR_next()
    TR_if x == TR_none { TR_break }
    n += 1
    satır += x[0].TR_shape[0]
    son = x
}
TR_print(n, satır, s.TR_next())
`)
	// p0: (300-33)/32+1 = 9 pencere, p1: (500-33)/32+1 = 15 pencere → 24 pencere, 3 toplu iş.
	if strings.TrimSpace(out) != "3 24 none" {
		t.Fatalf("çıktı %q", out)
	}
	last := variable(t, m, "son").(*interp.ListVal)
	g, gs := tensorData(t, last.Elements[0])
	if gs[0] != 8 || gs[1] != 32 || g[0] != float32(10000+7*32) {
		t.Fatalf("son toplu iş: şekil %v ilk %v", gs, g[0])
	}
	if n := dataset.OpenResources(); n != base {
		t.Fatalf("bitmiş sıralı okuyucunun kaynağı açık kaldı: %d > %d", n, base)
	}
}

// Kural (19, kaynaklar): kullanılmayan okuyucular çöp toplayıcıyla, kalanlar CloseAll ile kapanır;
// kapatılmış okuyucu çökmez, açık bir hata verir (H5516).
func TestResourcesClose(t *testing.T) {
	d := buildDataset(t)
	dataset.CloseAll(time.Second)
	base := dataset.OpenResources()
	_, m := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`")
TR_func geçici() {
    TR_var ö = k.TR_sampler(TR_batch_size: 2, TR_context: 8)
    ö.TR_next()
}
geçici()
geçici()
TR_var kalıcı = k.TR_sampler(TR_batch_size: 2, TR_context: 8)
kalıcı.TR_next()
`)
	last := time.Now().Add(5 * time.Second)
	for dataset.OpenResources() > base+1 {
		if time.Now().After(last) {
			t.Fatalf("erişilemez okuyucular kapanmadı: açık %d, beklenen %d", dataset.OpenResources(), base+1)
		}
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	runtime.KeepAlive(m)
	dataset.CloseAll(time.Second)
	if n := dataset.OpenResources(); n != base {
		t.Fatalf("CloseAll sonrası açık kaynak %d, beklenen %d", n, base)
	}
	_, err := m.ApplyFunction(mustMember(t, variable(t, m, "kalıcı"), codes.MemNext), nil, nil, 1)
	if !messages.Has(err, messages.H5516) {
		t.Fatalf("kapatılmış okuyucu hatası bekleniyordu: %v", err)
	}
}

// mustMember değerin k üyesidir; üyeye içerideki gibi koduyla (kimliğiyle) erişilir.
func mustMember(t *testing.T, v interp.Value, k codes.Code) interp.Value {
	t.Helper()
	pg, ok := v.(interp.PropertyGetter)
	if !ok {
		t.Fatalf("%T üyesi yok", v)
	}
	u, err := pg.GetProperty(k, k.String())
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// Aynı okuyucuyu kullanan eşzamanlı görevler toplu işleri paylaşır; hiçbiri kaybolmaz.
func TestConcurrentTasks(t *testing.T) {
	d := buildDataset(t)
	out, _ := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`")
TR_var s = k.TR_sequential(TR_batch_size: 1, TR_context: 16)
TR_var sayaç = [0, 0, 0]
TR_for g TR_in 0..3 {
    TR_async {
        TR_for s.TR_next() != TR_none { sayaç[g] += 1 }
    }
}
TR_wait_all()
TR_print(sayaç[0] + sayaç[1] + sayaç[2])
`)
	// p0: (300-17)/16+1 = 18, p1: (500-17)/16+1 = 31.
	if strings.TrimSpace(out) != "49" {
		t.Fatalf("çıktı %q", out)
	}
}

// .sonraki() beklerken yorumlayıcı kilidi bırakılır: kilit tutulsaydı başka bir görev
// bu sırada ilerleyemezdi. Önden yükleme gorutini kilidi hiç almaz.
func TestNextReleasesLock(t *testing.T) {
	d := buildDataset(t)
	out, _ := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`")
TR_var s = k.TR_sequential(TR_batch_size: 1, TR_context: 16)
TR_var adım = 0
TR_async {
    TR_for i TR_in 0..200 { adım += 1 }
}
TR_for s.TR_next() != TR_none { }
TR_wait_all()
TR_print(adım)
`)
	if strings.TrimSpace(out) != "200" {
		t.Fatalf("çıktı %q", out)
	}
}

// Tensörler gömme ve çapraz_entropi'ye doğrudan verilebilir.
func TestEmbeddingAndCrossEntropy(t *testing.T) {
	d := buildDataset(t)
	out, _ := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`")
TR_var ö = k.TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_var girdi, hedef = ö.TR_next()
TR_var E = TR_randn([20000, 4], TR_seed: 1)
TR_var W = TR_randn([4, 20000], TR_seed: 2)
TR_var çıktı = TR_embedding_lookup(E, girdi) @ W
TR_print(çıktı.TR_shape, TR_Type(TR_cross_entropy(çıktı, hedef)), girdi.TR_shape, hedef.TR_shape)
`)
	if strings.TrimSpace(out) != "[2, 8, 20000] Tensor [2, 8] [2, 8]" {
		t.Fatalf("çıktı %q", out)
	}
}

func TestCache(t *testing.T) {
	d := buildDataset(t)
	cache := filepath.Join(t.TempDir(), "onb")
	out, _ := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`", TR_cache: "`+trPath(cache)+`", TR_quota_gb: 0.001)
TR_var x = k.TR_sampler(TR_batch_size: 2, TR_context: 8).TR_next()
TR_print(x[0].TR_shape)
`)
	if strings.TrimSpace(out) != "[2, 8]" {
		t.Fatalf("çıktı %q", out)
	}
	_, _, err := run(t, `TR_Dataset("`+trPath(d)+`", TR_cache: "`+trPath(cache)+`", TR_quota_gb: 0.000000001).TR_sampler(TR_batch_size: 2, TR_context: 8)`)
	if !messages.Has(err, messages.H5002) {
		t.Fatalf("kota hatası bekleniyordu: %v", err)
	}
	dataset.CloseAll(time.Second)
}

func TestErrorMessages(t *testing.T) {
	d := buildDataset(t)
	empty := t.TempDir()
	broken := filepath.Join(empty, "corrupt.npy")
	if err := os.WriteFile(broken, []byte("merhaba"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		code string
		want messages.Code
	}{
		{`TR_Dataset("bu_klasor_yok_123")`, messages.H2132},
		{`TR_Dataset("` + trPath(t.TempDir()) + `")`, messages.H5518},
		{`TR_Dataset("` + trPath(broken) + `")`, messages.H5514},
		{`TR_Dataset("` + trPath(d) + `", TR_quota_gb: 1)`, messages.H5510},
		{`TR_Dataset("` + trPath(d) + `", TR_cache: "x")`, messages.H5513},
		{`TR_Dataset("` + trPath(d) + `", onbelek: "x")`, messages.H7900},
		{`TR_Dataset("` + trPath(d) + `").TR_sampler(TR_batch_size: 2)`, messages.H5529},
		{`TR_Dataset("` + trPath(d) + `").TR_sampler(TR_batch_size: 0, TR_context: 4)`, messages.H5530},
		{`TR_Dataset("` + trPath(d) + `").TR_sampler(TR_batch_size: 2, TR_context: 4, TR_seed: -1)`, messages.H5532},
		{`TR_Dataset("` + trPath(d) + `").TR_sequential(TR_batch_size: 2, TR_context: 4, TR_seed: 1)`, messages.H7901},
		{`TR_Dataset("` + trPath(d) + `").TR_sampler(TR_batch_size: 2, TR_context: 600)`, messages.H5533},
		{`TR_Dataset("` + trPath(d) + `").karıştır()`, messages.H7904},
	}
	for _, dd := range cases {
		_, _, err := run(t, dd.code)
		if !messages.Has(err, dd.want) {
			t.Errorf("%s\n hata: %v\n beklenen: %s", dd.code, err, dd.want)
		}
	}
	// Hatalar dene/yakala ile yakalanabilir.
	out, _ := mustRun(t, `TR_try { TR_Dataset("bu_klasor_yok_123") } TR_catch h { TR_print("yakalandı") }`)
	if strings.TrimSpace(out) != "yakalandı" {
		t.Fatalf("çıktı %q", out)
	}
}

// Göreli yollar çalışma klasöründe yoksa programın klasörüne göre çözülür.
func TestRelativePathProgramDir(t *testing.T) {
	d := buildDataset(t)
	p := parser.New(source.TextLexer(`TR_var k = TR_Dataset("` + filepath.Base(d) + `")`))
	prog := p.ParseProgram()
	m := interp.New()
	m.Out = &bytes.Buffer{}
	m.SourceFile = filepath.Join(filepath.Dir(d), "program.tr")
	if _, err := m.Run(prog); err != nil {
		t.Fatal(err)
	}
}

func TestWatchPanel(t *testing.T) {
	d := buildDataset(t)
	_, m := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`")
TR_var ö = k.TR_sampler(TR_batch_size: 2, TR_context: 8)
TR_var s = k.TR_sequential(TR_batch_size: 2, TR_context: 8)
`)
	want := map[string]string{
		"k": "Veri kümesi (2 veri dosyası, 800 parça)",
		"ö": "Örnekleyici (2×8, dönem 0, 0 toplu iş)",
		"s": "Sıralı okuyucu (2×8, 0 toplu iş okundu)",
	}
	for name, label := range want {
		iz, ok := variable(t, m, name).(debugger.WatchValue)
		if !ok {
			t.Fatalf("%s WatchValue değil", name)
		}
		if iz.WatchLabel() != label {
			t.Errorf("%s: etiket %q, beklenen %q", name, iz.WatchLabel(), label)
		}
	}
	cols, rows, err := variable(t, m, "k").(debugger.WatchValue).TableData()
	if err != nil || len(cols) != 3 || len(rows) != 2 || rows[1][0] != "p1.npy" || rows[1][1] != "500" {
		t.Fatalf("tablo: %v %v %v", cols, rows, err)
	}
	if s := bind.ShortNumber(56_900_000_000).MessageText("tr"); s != "56,9 milyar" {
		t.Fatalf("KisaSayi: %q", s)
	}
	dataset.CloseAll(time.Second)
}

// Kural (B5): veri okuyucusu sırayla okunur; indekslenince ileti .sonraki()'yi söyler.
func TestReaderIndexSaysHowToRead(t *testing.T) {
	d := buildDataset(t)
	_, _, err := run(t, `TR_Dataset("`+trPath(d)+`").TR_sampler(TR_batch_size: 2, TR_context: 8)[0]`)
	if !messages.Has(err, messages.H2525) || !strings.Contains(err.Error(), ".sonraki()") {
		t.Fatalf("okuyucu indekslenince: %v", err)
	}
	dataset.CloseAll(time.Second)
}
