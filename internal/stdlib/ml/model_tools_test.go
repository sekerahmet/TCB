package ml

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestSinCos(t *testing.T) {
	mustRun(t, `
eşit_mi(0.0, sinüs(0))
eşit_mi(1.0, kosinüs(0))
eşit_mi(Ondalık, tip(sinüs(1)))
değişken x = tensör([0.0, 1.0, -2.0], türev_izle: doğru)
değişken y = sinüs(x)
doğrula(mutlak(y.değer[1] - 0.84147096) < 0.0000001)
geri_yayılım(toplam(y * 1))
doğrula(mutlak(x.gradyan.değer[2] - kosinüs(-2.0)) < 0.000001)
değişken z = tensör([0.5], türev_izle: doğru)
geri_yayılım(toplam(kosinüs(z)))
doğrula(mutlak(z.gradyan.değer[0] + sinüs(0.5)) < 0.000001)
`)
}

func TestConcatAndSplit(t *testing.T) {
	mustRun(t, `
değişken a = tensör([[1, 2], [3, 4]], türev_izle: doğru)
değişken b = tensör([[5], [6]], türev_izle: doğru)
değişken c = bitiştir([a, b])
eşit_mi([[1.0, 2.0, 5.0], [3.0, 4.0, 6.0]], c.değer)
eşit_mi([4, 2], bitiştir([a, a], eksen: 0).şekil)
geri_yayılım(toplam(c * tensör([1, 2, 3])))
eşit_mi([[1.0, 2.0], [1.0, 2.0]], a.gradyan.değer)
eşit_mi([[3.0], [3.0]], b.gradyan.değer)
değişken p = tensör([[1, 2, 3, 4, 5, 6]], türev_izle: doğru).böl(3)
eşit_mi(3, p.uzunluk)
eşit_mi([[3.0, 4.0]], p[1].değer)
değişken s = tensör([[1, 2], [3, 4], [5, 6], [7, 8]]).böl(2, eksen: 0)
eşit_mi([[5.0, 6.0], [7.0, 8.0]], s[1].değer)
// Metin'in .böl'ü değişmedi.
eşit_mi(["a", "b"], "a,b".böl(","))
`)
	errs := []struct {
		code string
		want messages.Code
	}{
		{"bitiştir([])", messages.H6307},
		{"bitiştir(tensör([1]))", messages.H6306},
		{"bitiştir([tensör([[1, 2]]), tensör([[1, 2, 3]])], eksen: 0)", messages.H6311},
		{"bitiştir([tensör([[1, 2]]), tensör([1, 2])])", messages.H6309},
		{"bitiştir([tensör([1]), tensör([2])], eksen: 3)", messages.H6016},
		{"bitiştir([tensör([1])], axis: 0)", messages.H7900},
		{"tensör([1, 2, 3]).böl(2)", messages.H6318},
		{"tensör([1, 2]).böl(0)", messages.H6317},
	}
	for _, h := range errs {
		if _, err := run(t, h.code); err == nil || !messages.Has(err, h.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", h.code, err, h.want)
		}
	}
}

func TestKeepDim(t *testing.T) {
	mustRun(t, `
değişken x = tensör([[[1, 2], [3, 4]], [[5, 6], [7, 8]]])
eşit_mi([2, 2, 1], toplam(x, eksen: -1, boyutu_koru: doğru).şekil)
eşit_mi([2, 1, 2], ortalama(x, eksen: 1, boyutu_koru: doğru).şekil)
eşit_mi([[[4.0, 6.0]], [[12.0, 14.0]]], toplam(x, eksen: 1, boyutu_koru: doğru).değer)
eşit_mi([[[2.0], [4.0]], [[6.0], [8.0]]], maks(x, eksen: -1, boyutu_koru: doğru).değer)
eşit_mi([[[1.0, 2.0], [3.0, 4.0]]], min(x, eksen: 0, boyutu_koru: doğru).değer)
eşit_mi([2, 2], toplam(x, eksen: -1, boyutu_koru: yanlış).şekil)
eşit_mi([1, 1, 1], toplam(x, boyutu_koru: doğru).şekil)
eşit_mi([1, 1, 1], maks(x, boyutu_koru: doğru).şekil)
// Normalleştirme: yayınlamayla satır toplamına bölme, türev izler.
değişken w = tensör([[1.0, 3.0]], türev_izle: doğru)
değişken n = w / toplam(w, eksen: -1, boyutu_koru: doğru)
eşit_mi([[0.25, 0.75]], n.değer)
geri_yayılım(toplam(n * tensör([1, 0])))
doğrula(mutlak(w.gradyan.değer[0][0] - 0.1875) < 0.000001)
`)
	for _, h := range []struct {
		code string
		want messages.Code
	}{
		{"toplam([1, 2], boyutu_koru: doğru)", messages.H6218},
		{"toplam(tensör([1, 2]), boyutu_koru: 1)", messages.H7905},
	} {
		if _, err := run(t, h.code); err == nil || !messages.Has(err, h.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", h.code, err, h.want)
		}
	}
}

// dikkat(...) bileşik sürümle (softmax(q·kᵀ/√dh + maske)·v) aynı sonucu ve gradyanı verir.
func TestAttentionBuiltin(t *testing.T) {
	mustRun(t, `
değişken q = tensör(rastgele_normal([2, 4, 6, 8], tohum: 1), türev_izle: doğru)
değişken k = tensör(rastgele_normal([2, 4, 6, 8], tohum: 2), türev_izle: doğru)
değişken v = tensör(rastgele_normal([2, 4, 6, 8], tohum: 3), türev_izle: doğru)
değişken satırlar = []
döngü i içinde 0..6 {
    değişken s = []
    döngü j içinde 0..6 { eğer j <= i { s.ekle(0.0) } değilse { s.ekle(-1000000000.0) } }
    satırlar.ekle(s)
}
değişken ek = tensör(satırlar)
fonksiyon fark(a, b) {
    değişken d = a - b
    sonuç maks(d * d).değer
}
değişken beklenen = softmax((q @ k.devrik()) * (1.0 / karekök(8)) + ek, eksen: -1) @ v
geri_yayılım(toplam(beklenen * beklenen))
değişken gq = tensör(q.gradyan)
değişken gk = tensör(k.gradyan)
değişken gv = tensör(v.gradyan)
değişken o = dikkat(tensör(q, türev_izle: doğru), tensör(k, türev_izle: doğru), tensör(v, türev_izle: doğru))
doğrula(fark(o, beklenen) < 0.0000000001, "ileri")
değişken q2 = tensör(q, türev_izle: doğru)
değişken k2 = tensör(k, türev_izle: doğru)
değişken v2 = tensör(v, türev_izle: doğru)
değişken o2 = dikkat(q2, k2, v2)
geri_yayılım(toplam(o2 * o2))
doğrula(fark(q2.gradyan, gq) < 0.0000000001, "dq")
doğrula(fark(k2.gradyan, gk) < 0.0000000001, "dk")
doğrula(fark(v2.gradyan, gv) < 0.0000000001, "dv")
`)
}

func TestAttentionOptions(t *testing.T) {
	out := mustRun(t, `
fonksiyon fark(a, b) {
    değişken d = a - b
    sonuç maks(d * d).değer
}
değişken q = rastgele_normal([1, 2, 5, 4], tohum: 1)
değişken k = rastgele_normal([1, 1, 7, 4], tohum: 2)
değişken v = rastgele_normal([1, 1, 7, 3], tohum: 3)
yazdır(dikkat(q, k, v).şekil)
yazdır(dikkat(q, k, v, nedensel: yanlış, pencere: 2).şekil)
// Son sorgu (Tq < Tk: sondan hizalı) bütün anahtarları görür: nedensel ile nedensel olmayan aynı.
değişken a = dikkat(q, k, v)
değişken b = dikkat(q, k, v, nedensel: yanlış)
yazdır(a[0][0][4].değer == b[0][0][4].değer, a[0][0][0].değer == b[0][0][0].değer)
// Tek anahtara bakan pencere: çıktı o anahtarın değeridir.
değişken p = dikkat(q, k, v, pencere: 1)
yazdır(fark(p[0][1][4], v[0][0][6]) < 0.0000000001)
// Maske 0 olan anahtarlar görülmez.
değişken m = birler([5, 7])
m[0] = 0
m[0][2] = 1
yazdır(fark(dikkat(q, k, v, nedensel: yanlış, maske: m)[0][0][0], v[0][0][2]) < 0.0000000001)
yazdır(dikkat(q, k, v, maske: yok).şekil)
`)
	want := "[1, 2, 5, 3]\n[1, 2, 5, 3]\ndoğru yanlış\ndoğru\ndoğru\n[1, 2, 5, 3]\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
	for _, h := range []struct {
		code string
		want messages.Code
	}{
		{"dikkat(tensör([1.0]), tensör([1.0]), tensör([1.0]))", messages.H6327},
		{"dikkat(sıfırlar([3, 4, 2]), sıfırlar([2, 4, 2]), sıfırlar([2, 4, 2]))", messages.H6333},
		{"dikkat(sıfırlar([2, 4, 2]), sıfırlar([2, 4, 3]), sıfırlar([2, 4, 3]))", messages.H6330},
		{"dikkat(sıfırlar([2, 4, 2]), sıfırlar([2, 4, 2]), sıfırlar([2, 5, 2]))", messages.H6331},
		{"dikkat(sıfırlar([1, 2, 4, 2]), sıfırlar([2, 2, 4, 2]), sıfırlar([2, 2, 4, 2]))", messages.H6329},
		{"dikkat(sıfırlar([4, 2]), sıfırlar([4, 2]), sıfırlar([4, 2]), maske: birler([3, 4]))", messages.H6335},
		{"dikkat(sıfırlar([4, 2]), sıfırlar([4, 2]), sıfırlar([4, 2]), pencere: -1)", messages.H6326},
		{"dikkat(sıfırlar([4, 2]), sıfırlar([4, 2]), sıfırlar([4, 2]), ölçek: 2)", messages.H7901},
		{"dikkat(sıfırlar([4, 2]), sıfırlar([4, 2]))", messages.H6321},
		{"dikkat([[1.0]], sıfırlar([1, 1]), sıfırlar([1, 1]))", messages.H6324},
	} {
		if _, err := run(t, h.code); err == nil || !messages.Has(err, h.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", h.code, err, h.want)
		}
	}
}

// Karışık hassasiyet bloğunda dikkat girdileri bf16'ya çevirir ve bf16 sonuç verir: blok
// dışında bf16 girdilerle yapılan dikkatle aynıdır.
func TestAttentionMixedPrecision(t *testing.T) {
	out := mustRun(t, `
fonksiyon fark(a, b) {
    değişken d = a - b
    sonuç maks(d * d).değer
}
değişken q = rastgele_normal([2, 6, 8], tohum: 1)
değişken a = karışık_hassasiyet("bf16") { dikkat(q, q, q) }
değişken b = dikkat(q.türe("bf16"), q.türe("bf16"), q.türe("bf16"))
yazdır(a.tür, fark(a.türe("float32"), b.türe("float32")) < 0.0000000001)
yazdır(dikkat(q.türe("bf16"), q.türe("bf16"), q.türe("bf16")).tür)
`)
	if out != "bf16 doğru\nbf16\n" {
		t.Fatalf("çıktı: %q", out)
	}
}

func TestSample(t *testing.T) {
	out := mustRun(t, `
değişken l = tensör([[0.0, 3.0, 1.0], [2.0, 0.0, 0.5]])
değişken e = örnekle(l, sıcaklık: 0)
yazdır(e, e.değer, e.şekil, tip(e.değer[0]))
yazdır(örnekle(tensör([1.0, 5.0, 2.0]), sıcaklık: 0).değer)
yazdır(örnekle(l, tohum: 7).değer == örnekle(l, tohum: 7).değer)
// en_iyi_k: 1 her zaman en olasıyı verir.
yazdır(örnekle(l, en_iyi_k: 1).değer)
// Tohumsuz çağrılar farklı sonuçlar verir (64 bağımsız örnek).
değişken düz = birler([64, 50])
yazdır(örnekle(düz).değer != örnekle(düz).değer)
// en_iyi_k: 2 dışında kalan sınıf hiç seçilmez.
değişken sayılar = [0, 0, 0]
döngü x içinde örnekle(tensör([[1.0, 1.1, 1.2]]) + birler([2000, 3]), en_iyi_k: 2, tohum: 1).değer { sayılar[x] += 1 }
yazdır(sayılar[0], sayılar[1] > 800, sayılar[2] > 800)
`)
	lines := strings.Split(out, "\n")
	want := []string{"Tensör[2] [1, 0] [1, 0] [2] Tam", "1", "doğru", "[1, 0]", "doğru", "0 doğru doğru"}
	for i, w := range want {
		if i >= len(lines) || lines[i] != w {
			t.Fatalf("satır %d: %q, beklenen %q\n%s", i+1, lines[min(i, len(lines)-1)], w, out)
		}
	}
	// Dağılım: sıcaklık 1'de olasılıklar softmax'a, yüksek sıcaklıkta düzgüne yakın.
	v := value(t, `
değişken l = tensör([0.0, 1.0, 2.0])
değişken satır = birler([30000, 3]) * l
değişken ö = örnekle(satır, tohum: 5).değer
değişken s = [0, 0, 0]
döngü x içinde ö { s[x] += 1 }
değişken cikti = s`)
	s := v.(*interp.ListVal).Elements
	top := math.Exp(0) + math.Exp(1) + math.Exp(2)
	for i, e := range s {
		got := float64(e.(*interp.IntVal).Val) / 30000
		want := math.Exp(float64(i)) / top
		if math.Abs(got-want) > 0.015 {
			t.Fatalf("sınıf %d: oran %.4f, beklenen %.4f", i, got, want)
		}
	}
	for _, h := range []struct {
		code string
		want messages.Code
	}{
		{"örnekle(tensör([1.0]), sıcaklık: -1)", messages.H6340},
		{"örnekle(tensör([1.0]), en_iyi_k: -2)", messages.H6341},
		{"örnekle(tensör(1.0))", messages.H6338},
		{"örnekle([1.0, 2.0])", messages.H6337},
		{"örnekle(tensör([1.0]), tohum: 1.5)", messages.H6342},
		{"örnekle(tensör([1.0]), temperature: 1)", messages.H7901},
	} {
		if _, err := run(t, h.code); err == nil || !messages.Has(err, h.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", h.code, err, h.want)
		}
	}
}

func TestParameters(t *testing.T) {
	fpath := filepath.ToSlash(filepath.Join(t.TempDir(), "model.trt"))
	out := mustRun(t, `
fonksiyon p(x) { sonuç tensör([x], türev_izle: doğru) }
yapı Dikkat { Wq, Wk, ölçek }
yapı Blok { dikkat, mlp }
yapı Model { gömme, bloklar, son, ad, ek }
değişken ortak = p(9)
değişken m = Model(p(1), [Blok(Dikkat(p(2), p(3), 0.5), [p(4), ortak]), Blok(Dikkat(p(5), ortak, 1), {"W1": p(6), "sabit": tensör([0.0])})], ortak, "küçük")
m.ek = p(7)
değişken liste = parametreler(m)
yazdır(liste.uzunluk, [x.değer[0] döngü x içinde liste])
değişken adlar = parametreler(m, adlarıyla: doğru)
yazdır(adlar.anahtarlar())
// Aynı tensör nesnesidir: iyileştirici ile güncellenir.
doğrula(liste[0] == m.gömme)
tensör_kaydet("`+fpath+`", adlar)
değişken y = tensör_yükle("`+fpath+`")
yazdır(y.anahtarlar() == adlar.anahtarlar(), y["bloklar.1.dikkat.Wq"].değer)
// Ara sonuçlar ve türev izlemeyenler parametre değildir.
yazdır(parametreler([p(1) * 2, tensör([1.0])]).uzunluk, parametreler(p(1)).uzunluk, parametreler(p(1), adlarıyla: doğru).anahtarlar())
// Döngülü yapılar sonsuz döngüye girmez.
değişken l = [p(1)]
l.ekle(l)
yazdır(parametreler(l).uzunluk)
`)
	lines := strings.Split(out, "\n")
	expected := []string{
		"8 [1.0, 2.0, 3.0, 4.0, 9.0, 5.0, 6.0, 7.0]",
		`["gömme", "bloklar.0.dikkat.Wq", "bloklar.0.dikkat.Wk", "bloklar.0.mlp.0", "bloklar.0.mlp.1", "bloklar.1.dikkat.Wq", "bloklar.1.mlp.W1", "ek"]`,
		"doğru [5.0]",
		`0 1 ["0"]`,
		"1",
	}
	for i, w := range expected {
		if i >= len(lines) || lines[i] != w {
			t.Fatalf("satır %d:\n  %q\nbeklenen:\n  %q\ntüm çıktı:\n%s", i+1, lines[min(i, len(lines)-1)], w, out)
		}
	}
	if _, err := run(t, "parametreler([], adlariyla: doğru, x: 1)"); err == nil {
		t.Fatal("bilinmeyen adlı argüman hata vermeliydi")
	}
	if _, err := run(t, `parametreler({"a.b": tensör([1.0], türev_izle: doğru), "a": {"b": tensör([1.0], türev_izle: doğru)}}, adlarıyla: doğru)`); err == nil || !messages.Has(err, messages.H6505) {
		t.Fatalf("çakışan ad hata vermeliydi: %v", err)
	}
}

// AlanlariGez tanım sırasını, ardından sonradan eklenen alanları ad sırasıyla verir.
func TestFieldWalkOrder(t *testing.T) {
	v := value(t, `
yapı A { z, a, b, y }
değişken x = A(1, 2)
x.y = 3
x.b = 4
değişken cikti = x`)
	var names []string
	v.(*interp.StructInstanceVal).EachField(func(ad string, _ interp.Value) { names = append(names, ad) })
	if strings.Join(names, ",") != "z,a,b,y" {
		t.Fatalf("sıra: %v", names)
	}
}

// örnekle tanımsız dağılımda (NaN logit, hepsi -∞, +∞) sessizce son adayı seçmez, hata verir.
func TestSampleUndefinedDistribution(t *testing.T) {
	for _, logits := range []string{"tensör([nan, 1.0, 2.0])", "tensör([-1e40, -1e40])", "tensör([1e40, 1.0])"} {
		_, err := run(t, "değişken nan = log(tensör([-1.0])).değer[0]\nörnekle("+logits+", tohum: 1)")
		if err == nil || !messages.Has(err, messages.H6343) {
			t.Fatalf("%s: hata beklenirdi: %v", logits, err)
		}
	}
	mustRun(t, "örnekle(tensör([-1e40, 0.0]), tohum: 1)") // bir -∞ aday sorun değil
}
