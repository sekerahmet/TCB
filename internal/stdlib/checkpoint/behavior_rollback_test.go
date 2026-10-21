package checkpoint_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
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
