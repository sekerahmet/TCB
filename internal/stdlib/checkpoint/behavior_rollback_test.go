package checkpoint_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// rollbackProgram trainingProgram'ın eğitimidir; tek farkı corruptAt adımında kaybın NaN'a
// dönmesi, iyileştiricinin bu bozuk gradyanla adım atması (parametreler bozulur) ve döngünün
// bunu görünce kn.geri_dön() ile son noktaya dönüp oradan sürmesidir. corruptAt < 0 bozmaz.
func rollbackProgram(runDir string, total, corruptAt int) string {
	return fmt.Sprintf(`
değişken KOŞU = %q
değişken V = 4096
değişken D = 16
değişken sayaç = 0
fonksiyon parametre(şekil, ölçek) {
    sayaç += 1
    sonuç tensör(türevsiz(rastgele_normal(şekil, tohum: sayaç) * ölçek), türev_izle: doğru)
}
değişken model = {E: parametre([V, D], 0.1), W: parametre([D, V], 0.1)}
değişken ikinci = [parametre([D], 1.0)]
değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.01)
değişken op2 = SGD(ikinci, öğrenme_oranı: 0.1, momentum: 0.9)
değişken küme = veri_kümesi(%q)
değişken ö = küme.örnekleyici(toplu: 4, bağlam: 8, tohum: 5)
değişken s = küme.sıralı(toplu: 2, bağlam: 8)
değişken g = günlük(yol(KOŞU, "metrics.jsonl"))
değişken izlenen = {en_iyi: 99.0, geçmiş: [], son_sıralı: yok}
değişken kn = kontrol_noktası(KOŞU, {model: model, op: op, ikinci: ikinci, op2: op2, ö: ö, s: s, g: g, izlenen: izlenen}, her_dakika: 3)
değişken ç = ilerleme(%d, günlük: g)
değişken bozuldu = yanlış
değişken adım = kn.kaldığı_adım
döngü adım < %d {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    op2.sıfırla()
    değişken kayıp = çapraz_entropi(gömme(model.E, giriş) @ model.W, hedef) + toplam(ikinci[0] * ikinci[0]) * 0.01
    eğer adım == %d ve değil bozuldu {
        kayıp = kayıp * ondalık("nan")
        bozuldu = doğru
    }
    geri_yayılım(kayıp)
    op.adım()
    op2.adım()
    eğer değil sonlu_mu(kayıp.değer) {
        yazdır("bozuk:", sonlu_mu(toplam(model.W).değer))
        adım = kn.geri_dön()
        yazdır("geri:", adım)
        devam
    }
    değişken d = s.sonraki()
    eğer d != yok { izlenen.son_sıralı = d[0].değer[0][0] }
    eğer kayıp.değer < izlenen.en_iyi { izlenen.en_iyi = kayıp.değer }
    izlenen.geçmiş.ekle(adım)
    ç.güncelle({kayıp: kayıp}, adım: adım + 1)
    adım += 1
    kn.güncelle(adım)
}
tensör_kaydet(yol(KOŞU, "son.trt"), parametreler([model, ikinci], adlarıyla: doğru))
tensör_kaydet(yol(KOŞU, "op.trt"), op.durum)
tensör_kaydet(yol(KOŞU, "op2.trt"), op2.durum)
yazdır("izlenen:", izlenen)
yazdır("ö:", ö.durum)
`, trPath(runDir), trPath(filepath.Join(reporoot.Root(), "tests", "data", "text")), total, total, corruptAt)
}

// Kural (20.14): kn.geri_dön() en yeni geçerli noktayı aynı süreçte yerinde yükler —
// parametreler, iyileştirici durumları, okuyucular, düz değerler, günlüğe devam kaydı — ve
// noktanın adımını verir. NaN'la bozulan bir adımdan geri dönüp sürdürülen eğitim, hiç
// bozulmamış eğitimle bit düzeyinde aynı biter.
// Neden: uzun eğitimde kayıp sıçraması ya da NaN olunca son sağlam noktadan sürmek (PaLM §5.1).
func TestRollbackAfterNaNSameAsUninterrupted(t *testing.T) {
	var now time.Time
	t.Cleanup(checkpoint.FakeClock(func() time.Time { now = now.Add(time.Minute); return now }))
	a := t.TempDir()
	outputA, err := run(t, rollbackProgram(a, 20, -1))
	if err != nil {
		t.Fatalf("%v\n%s", err, outputA)
	}
	now = time.Time{}
	b := t.TempDir()
	outputB, err := run(t, rollbackProgram(b, 20, 11))
	if err != nil {
		t.Fatalf("%v\n%s", err, outputB)
	}
	// Her güncelle saati 1 dk ilerletir, her_dakika: 3 → 3, 6, 9. adımlarda kayıt; 11. adımda
	// bozulan eğitim 9'a döner.
	if !strings.Contains(outputB, "bozuk: yanlış") || !strings.Contains(outputB, "geri: 9") {
		t.Fatalf("bozulup 9. adıma dönmeliydi:\n%s", outputB)
	}
	sameResults(t, a, b, outputA, outputB)
}

// Kural (20.14): nokta yokken geri_dön açık hatadır (H7486); argüman almaz.
func TestRollbackWithoutCheckpointIsError(t *testing.T) {
	dir := trPath(t.TempDir())
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{fmt.Sprintf(`değişken w = tensör([1.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w})
kn.geri_dön()`, dir), messages.H7486},
		{fmt.Sprintf(`değişken w = tensör([1.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w})
kn.kaydet()
kn.geri_dön(1)`, dir), messages.H2012},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// Kural (20.7, 20.14): geri_dön (ve kurulumda yükleme) yazmaya başlamadan önce modelin
// tensörlerinin yazılabilir olduğunu denetler; yazılamayan bir tensör varsa hiçbir nesne
// (okuyucu, parametre, iyileştirici, düz değer) değişmez. Parametreler okuyuculardan önce
// yazılır; yazma yine de yarıda kalırsa (cihaz belleği) okuyucular değişmez.
// Neden: eskiden okuyucular önce sarılıyor, ikinci parametrede hata çıkınca ilk parametre ve
// okuyucu yeni, öbürleri eski kalıyordu (Gemini kararlılık denetimi, bulgu 1). Bırakılmış tensör
// TR kodundan oluşamaz; test onu bir Go yardımcısıyla kurar.
func TestRollbackStopsBeforeWritingWhenAParamIsUnusable(t *testing.T) {
	runDir := t.TempDir()
	dataDir := filepath.Join(reporoot.Root(), "tests", "data", "text")

	m := interp.New()
	w := &testkit.TeeWriter{}
	m.Out, m.Err = w, w
	m.GlobalEnv.Define("tensör_bırak", &interp.BuiltinVal{
		Name: "tensör_bırak",
		Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			args[0].(*ml.TensorValue).Tensor().Drop()
			return interp.Null, nil
		},
	}, true)

	code := fmt.Sprintf(`
değişken KOŞU = %q
değişken küme = veri_kümesi(%q)
değişken ö = küme.örnekleyici(toplu: 2, bağlam: 8, tohum: 1)
değişken model = {
    a: tensör([10.0, 20.0], türev_izle: doğru),
    b: tensör([30.0, 40.0], türev_izle: doğru)
}
değişken op = AdamW([model.a, model.b], öğrenme_oranı: 0.01)
değişken izlenen = {adım: 1}
değişken kn = kontrol_noktası(KOŞU, {model: model, op: op, ö: ö, izlenen: izlenen})

// 1. adımda 1 toplu iş okuyup kontrol noktasını kaydet (ö.durum.sayaç == 1, model.a == [10, 20]).
ö.sonraki()
kn.güncelle(1)
kn.kaydet()

// Eğitim 4. adıma ilerlesin: ö 3 toplu iş daha okur (sayaç == 4), model.a = [1, 2], izlenen.adım = 4.
ö.sonraki()
ö.sonraki()
ö.sonraki()
türevsiz { model.a[0..2] = tensör([1.0, 2.0]) }
izlenen.adım = 4

// İkinci parametre (model.b) writeParams sırasında hata üretsin.
tensör_bırak(model.b)

değişken hata_var = yanlış
dene {
    kn.geri_dön()
} yakala h {
    hata_var = doğru
}
yazdır("HATA:", hata_var)
yazdır("OKU_SAYAC:", ö.durum.sayaç)
yazdır("MODEL_A:", model.a.değer)
yazdır("IZLENEN_ADIM:", izlenen.adım)
`, trPath(runDir), trPath(dataDir))

	if _, err := runVM(t, m, code); err != nil {
		t.Fatalf("beklenmeyen çalışma hatası: %v\n%s", err, w.String())
	}
	output := w.String()
	if !strings.Contains(output, "HATA: doğru") {
		t.Fatalf("geri_dön hata vermeliydi:\n%s", output)
	}
	if !strings.Contains(output, "OKU_SAYAC: 4") || !strings.Contains(output, "MODEL_A: [1.0, 2.0]") || !strings.Contains(output, "IZLENEN_ADIM: 4") {
		t.Fatalf("geri_dön hata verdiğinde hiçbir nesne değişmemeliydi (20.7), ancak nesneler yarı yüklenmiş kaldı:\n%s", output)
	}
}
