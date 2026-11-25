package dataset_test

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Sürdürme kesintisizle aynıdır: okuyucu durumu (ö.durum) düz değerli bir sözlüktür ve
// aynı ayarla kurulmuş yeni bir okuyucuya durumu_yükle ile verilince kesintisiz okuyucunun
// geri kalan toplu işlerini aynı sırayla verir. Durum sözlüğünün anahtarları ve okuyucu türleri
// kontrol noktası dosya biçiminin parçasıdır: dilden bağımsız İngilizce tanımlayıcılardır, Türkçe
// programda da aynıdır (dil tanımı 17.3).
func TestReaderStateResumesSameBatches(t *testing.T) {
	d := buildDataset(t)
	out, _ := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`")
TR_func çek(ö, n) {
    TR_var liste = []
    TR_for i TR_in 0..n {
        TR_var b = ö.TR_next()
        TR_if b == TR_none { liste.TR_append(TR_none) } TR_else { liste.TR_append([b[0].TR_tolist, b[1].TR_tolist]) }
    }
    TR_return liste
}
TR_func dene_okuyucu(yap) {
    TR_var kesintisiz = yap()
    çek(kesintisiz, 3)
    TR_var durum = kesintisiz.TR_state
    TR_var kalan = çek(kesintisiz, 4)
    TR_var sürdürülen = yap()
    sürdürülen.TR_load_state(durum)
    TR_print(kalan == çek(sürdürülen, 4))
    TR_print(durum.TR_keys())
}
dene_okuyucu { k.TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 3) }
dene_okuyucu { k.TR_sequential(TR_batch_size: 2, TR_context: 8) }
`)
	want := "true\n" +
		`["kind", "batch", "context", "seed", "dataset", "epoch", "position", "window", "counter"]` + "\n" +
		"true\n" +
		`["kind", "batch", "context", "dataset", "position", "window", "counter", "done"]` + "\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", strings.TrimSpace(out), strings.TrimSpace(want))
	}
}

// Kural (20.15): ö.atla(n) sıradaki n toplu işi tensör kurmadan atlar ve atladığı sayıyı
// verir; sonraki .sonraki() n tane sonraki() çağrısından sonra gelecek toplu işin aynısıdır ve
// durumun sayacı n artar. Sıralı okuyucu biterse kalan kadar atlar, sonra yok verir. n = 0
// bir şey yapmaz; negatif, Tam olmayan ya da eksik/fazla argüman açık hatadır.
// Neden: kayıp sıçramasından sonra son noktaya dönen eğitim, sıçramayı tetikleyen toplu
// işleri atlar (PaLM §5.1: 200–500 toplu iş).
func TestReaderSkipSameAsNext(t *testing.T) {
	d := buildDataset(t)
	out, _ := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(d)+`")
TR_func dene_okuyucu(yap) {
    TR_var a = yap()
    TR_for i TR_in 0..5 { a.TR_next() }
    TR_var beklenen = a.TR_next()
    TR_var b = yap()
    b.TR_next()
    TR_print(b.TR_skip(0), b.TR_skip(4))
    TR_var x = b.TR_next()
    TR_print(x[0].TR_tolist == beklenen[0].TR_tolist, x[1].TR_tolist == beklenen[1].TR_tolist, b.TR_state["counter"] == a.TR_state["counter"])
}
dene_okuyucu { k.TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 3) }
dene_okuyucu { k.TR_sequential(TR_batch_size: 2, TR_context: 8) }
TR_var s = k.TR_sequential(TR_batch_size: 8, TR_context: 32)
TR_print(s.TR_skip(10), s.TR_next())
`)
	want := "0 4\ntrue true true\n0 4\ntrue true true\n3 none\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
	for _, code := range []string{"ö.TR_skip(-1)", "ö.TR_skip(1.5)", "ö.TR_skip()", "ö.TR_skip(1, 2)", "ö.TR_skip(\"a\")"} {
		_, _, err := run(t, `TR_var ö = TR_Dataset("`+trPath(d)+`").TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 3)
`+code)
		if err == nil {
			t.Errorf("%s hata vermeliydi", code)
		}
	}
}

// Kural (16.7, 20.13): tohum her TR_Int'tir; eksi tohumlu örnekleyicinin durumu yüklenir, okuma
// kaldığı yerden sürer (dosyadan: tests/tohum_test.tr). Tam sayı olmayan tohum H5542, başka tohum
// H5544, tohum dışındaki alanda eksi sayı H5543'tür.
func TestNegativeSeedStateLoads(t *testing.T) {
	d := buildDataset(t)
	k := `TR_var k = TR_Dataset("` + trPath(d) + `")
TR_func yap(s: TR_Int) -> TR_Sampler { TR_return k.TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: s) }
`
	out, _ := mustRun(t, k+`
TR_for s TR_in [-3, -9223372036854775807 - 1, TR_fold_in(7, 0), 3] {
    TR_var a = yap(s)
    a.TR_next()
    TR_var durum = a.TR_state
    TR_var b = yap(s)
    b.TR_load_state(durum)
    TR_print(durum["seed"] == s, a.TR_next()[0].TR_tolist == b.TR_next()[0].TR_tolist)
}`)
	if want := strings.Repeat("true true\n", 4); out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
	for _, c := range []struct {
		field, value string
		want         messages.Code
	}{
		{"seed", "1.5", messages.H5542},
		{"seed", "-4", messages.H5544},
		{"counter", "-1", messages.H5543},
		{"position", "-1", messages.H5543},
		{"batch", "-1", messages.H5543},
		{"batch", "0", messages.H5547},
	} {
		_, _, err := run(t, k+`
TR_var durum = yap(-3).TR_state
durum["`+c.field+`"] = `+c.value+`
yap(-3).TR_load_state(durum)`)
		if !messages.Has(err, c.want) {
			t.Errorf("%s = %s: %v, beklenen %s", c.field, c.value, err, c.want)
		}
	}
}
