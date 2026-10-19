package dataset_test

import (
	"bytes"
	"context"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/devtools/debugger"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/dataset"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// run TR kodunu tam dille (motorlar dahil) çalıştırır; çıktıyı ve VM'yi döndürür.
func run(t *testing.T, kd string) (string, *interp.VM, error) {
	t.Helper()
	p := parser.New(lexer.New(kd))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	var out bytes.Buffer
	m.Out = &out
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), m, err
}

func mustRun(t *testing.T, kd string) (string, *interp.VM) {
	t.Helper()
	out, m, err := run(t, kd)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v\nkod:\n%s", err, kd)
	}
	return out, m
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
değişken k = veri_kümesi("`+trPath(d)+`")
değişken ö = k.örnekleyici(toplu: 4, bağlam: 16, tohum: 7)
değişken ilk = ö.sonraki()
değişken ikinci = ö.sonraki()
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
değişken k = veri_kümesi("`+trPath(d)+`")
değişken s = k.sıralı(toplu: 8, bağlam: 32)
değişken n = 0
değişken satır = 0
değişken son = yok
döngü {
    değişken x = s.sonraki()
    eğer x == yok { çık }
    n += 1
    satır += x[0].şekil[0]
    son = x
}
yazdır(n, satır, s.sonraki())
`)
	// p0: (300-33)/32+1 = 9 pencere, p1: (500-33)/32+1 = 15 pencere → 24 pencere, 3 toplu iş.
	if strings.TrimSpace(out) != "3 24 yok" {
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

// Kullanılmayan okuyucular çöp toplayıcıyla, kalanlar CloseAll ile kapanır.
func TestResourcesClose(t *testing.T) {
	d := buildDataset(t)
	dataset.CloseAll(time.Second)
	base := dataset.OpenResources()
	_, m := mustRun(t, `
değişken k = veri_kümesi("`+trPath(d)+`")
fonksiyon geçici() {
    değişken ö = k.örnekleyici(toplu: 2, bağlam: 8)
    ö.sonraki()
}
geçici()
geçici()
değişken kalıcı = k.örnekleyici(toplu: 2, bağlam: 8)
kalıcı.sonraki()
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
	// Kapatılmış okuyucu açık bir hata verir, çökmez.
	_, err := m.ApplyFunction(mustMember(t, variable(t, m, "kalıcı"), "sonraki"), nil, nil, 1)
	if err == nil || !strings.Contains(err.Error(), "kapatılmış") {
		t.Fatalf("kapatılmış okuyucu hatası bekleniyordu: %v", err)
	}
}

func mustMember(t *testing.T, v interp.Value, name string) interp.Value {
	t.Helper()
	pg, ok := v.(interp.PropertyGetter)
	if !ok {
		t.Fatalf("%T üyesi yok", v)
	}
	u, err := pg.GetProperty(codes.MemberCode(name), name)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// Aynı okuyucuyu kullanan eşzamanlı görevler toplu işleri paylaşır; hiçbiri kaybolmaz.
func TestConcurrentTasks(t *testing.T) {
	d := buildDataset(t)
	out, _ := mustRun(t, `
değişken k = veri_kümesi("`+trPath(d)+`")
değişken s = k.sıralı(toplu: 1, bağlam: 16)
değişken sayaç = [0, 0, 0]
döngü g içinde 0..3 {
    eşzamanlı {
        döngü s.sonraki() != yok { sayaç[g] += 1 }
    }
}
bekle_hepsi()
yazdır(sayaç[0] + sayaç[1] + sayaç[2])
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
değişken k = veri_kümesi("`+trPath(d)+`")
değişken s = k.sıralı(toplu: 1, bağlam: 16)
değişken adım = 0
eşzamanlı {
    döngü i içinde 0..200 { adım += 1 }
}
döngü s.sonraki() != yok { }
bekle_hepsi()
yazdır(adım)
`)
	if strings.TrimSpace(out) != "200" {
		t.Fatalf("çıktı %q", out)
	}
}

// Tensörler gömme ve çapraz_entropi'ye doğrudan verilebilir.
func TestEmbeddingAndCrossEntropy(t *testing.T) {
	d := buildDataset(t)
	out, _ := mustRun(t, `
değişken k = veri_kümesi("`+trPath(d)+`")
değişken ö = k.örnekleyici(toplu: 2, bağlam: 8, tohum: 1)
değişken girdi, hedef = ö.sonraki()
değişken E = rastgele_normal([20000, 4], tohum: 1)
değişken W = rastgele_normal([4, 20000], tohum: 2)
değişken çıktı = gömme(E, girdi) @ W
yazdır(çıktı.şekil, tip(çapraz_entropi(çıktı, hedef)), girdi.şekil, hedef.şekil)
`)
	if strings.TrimSpace(out) != "[2, 8, 20000] Tensör [2, 8] [2, 8]" {
		t.Fatalf("çıktı %q", out)
	}
}

func TestCache(t *testing.T) {
	d := buildDataset(t)
	cache := filepath.Join(t.TempDir(), "onb")
	out, _ := mustRun(t, `
değişken k = veri_kümesi("`+trPath(d)+`", önbellek: "`+trPath(cache)+`", kota_gb: 0.001)
değişken x = k.örnekleyici(toplu: 2, bağlam: 8).sonraki()
yazdır(x[0].şekil)
`)
	if strings.TrimSpace(out) != "[2, 8]" {
		t.Fatalf("çıktı %q", out)
	}
	_, _, err := run(t, `veri_kümesi("`+trPath(d)+`", önbellek: "`+trPath(cache)+`", kota_gb: 0.000000001).örnekleyici(toplu: 2, bağlam: 8)`)
	if err == nil || !strings.Contains(err.Error(), "önbellek kotası yetersiz") {
		t.Fatalf("kota hatası bekleniyordu: %v", err)
	}
	dataset.CloseAll(time.Second)
}

func TestErrorMessages(t *testing.T) {
	d := buildDataset(t)
	empty := t.TempDir()
	broken := filepath.Join(empty, "bozuk.npy")
	if err := os.WriteFile(broken, []byte("merhaba"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ code, want string }{
		{`veri_kümesi("bu_klasor_yok_123")`, "açılamadı: böyle bir dosya ya da klasör yok"},
		{`veri_kümesi("` + trPath(t.TempDir()) + `")`, ".npy veri dosyası yok"},
		{`veri_kümesi("` + trPath(broken) + `")`, "geçerli bir .npy veri dosyası değil"},
		{`veri_kümesi("` + trPath(d) + `", kota_gb: 1)`, "yalnızca önbellek: ile birlikte"},
		{`veri_kümesi("` + trPath(d) + `", önbellek: "x")`, "kota_gb: de verilmelidir"},
		{`veri_kümesi("` + trPath(d) + `", onbelek: "x")`, "Bunu mu demek istediniz: 'önbellek'"},
		{`veri_kümesi("` + trPath(d) + `").örnekleyici(toplu: 2)`, "bağlam: verilmedi"},
		{`veri_kümesi("` + trPath(d) + `").örnekleyici(toplu: 0, bağlam: 4)`, "toplu pozitif bir tam sayı"},
		{`veri_kümesi("` + trPath(d) + `").örnekleyici(toplu: 2, bağlam: 4, tohum: -1)`, "tohum negatif olmayan"},
		{`veri_kümesi("` + trPath(d) + `").sıralı(toplu: 2, bağlam: 4, tohum: 1)`, "'tohum' adında bir parametre almaz"},
		{`veri_kümesi("` + trPath(d) + `").örnekleyici(toplu: 2, bağlam: 600)`, "en uzun veri dosyası 500 parça"},
		{`veri_kümesi("` + trPath(d) + `").karıştır()`, "üyeler: örnekleyici, sıralı"},
	}
	for _, dd := range cases {
		_, _, err := run(t, dd.code)
		if err == nil || !strings.Contains(err.Error(), dd.want) {
			t.Errorf("%s\n hata: %v\n beklenen: %q", dd.code, err, dd.want)
		}
	}
	// Hatalar dene/yakala ile yakalanabilir.
	out, _ := mustRun(t, `dene { veri_kümesi("bu_klasor_yok_123") } yakala h { yazdır("yakalandı") }`)
	if strings.TrimSpace(out) != "yakalandı" {
		t.Fatalf("çıktı %q", out)
	}
}

// Göreli yollar çalışma klasöründe yoksa programın klasörüne göre çözülür.
func TestRelativePathProgramDir(t *testing.T) {
	d := buildDataset(t)
	p := parser.New(lexer.New(`değişken k = veri_kümesi("` + filepath.Base(d) + `")`))
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
değişken k = veri_kümesi("`+trPath(d)+`")
değişken ö = k.örnekleyici(toplu: 2, bağlam: 8)
değişken s = k.sıralı(toplu: 2, bağlam: 8)
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
	if s := bind.ShortNumber(56_900_000_000); s != "56,9 milyar" {
		t.Fatalf("KisaSayi: %q", s)
	}
	dataset.CloseAll(time.Second)
}
