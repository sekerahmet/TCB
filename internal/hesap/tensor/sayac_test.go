package tensor

import (
	"testing"
	"time"
)

// sayacFarki, f'nin çalışması boyunca (cpu, yön, işlem) kaydında biriken çağrı, ölçüm ve
// baytı verir. Sayaçlar süreç genelidir: bu testler t.Parallel kullanmaz (paralel testler
// sıralı testler bitince koşar, sayaçlara aynı anda yazılmaz).
func sayacFarki(geri bool, is Islem, f func()) (cagri, olcum, bayt int64, sure time.Duration) {
	bul := func() SayacKaydi {
		for _, k := range SayacGoruntusu() {
			if k.Cihaz == KindCPU && k.Geri == geri && k.Islem == is {
				return k
			}
		}
		return SayacKaydi{}
	}
	once := bul()
	f()
	sonra := bul()
	return sonra.Cagri - once.Cagri, sonra.Olcum - once.Olcum, sonra.Bayt - once.Bayt, sonra.Sure - once.Sure
}

func TestSayacBilinenSayidaIslem(t *testing.T) {
	a := New(make([]float32, 16), 4, 4)
	b := New(make([]float32, 16), 4, 4)
	cagri, _, bayt, _ := sayacFarki(false, IslemToplama, func() {
		for i := 0; i < 7; i++ {
			Add(a, b)
		}
	})
	if cagri != 7 {
		t.Errorf("toplama çağrısı %d, beklenen 7", cagri)
	}
	if bayt != 7*16*4 {
		t.Errorf("toplama baytı %d, beklenen %d (7 çıktı × 16 float32)", bayt, 7*16*4)
	}
	cagri, _, _, _ = sayacFarki(false, IslemSkalerCarpma, func() {
		for i := 0; i < 3; i++ {
			a.MulScalar(0.5)
		}
	})
	if cagri != 3 {
		t.Errorf("skaler çarpma çağrısı %d, beklenen 3", cagri)
	}
}

func TestSayacGeriYayilimDugumAdi(t *testing.T) {
	w := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	x := New([]float32{1, 0, 2, 1, 0, 1}, 3, 2)
	var y *Tensor
	ileri, _, _, _ := sayacFarki(false, IslemMatrisCarpimi, func() { y = MatMul(w, x) })
	if ileri != 1 {
		t.Fatalf("ileri matris çarpımı %d, beklenen 1", ileri)
	}
	if y.islem != IslemMatrisCarpimi {
		t.Fatalf("düğümün işlemi %v, beklenen matris çarpımı", y.islem)
	}
	kayip := y.Reshape(-1).SumAll()
	geri, _, bayt, _ := sayacFarki(true, IslemMatrisCarpimi, func() { kayip.Backward() })
	if geri != 1 {
		t.Errorf("geri matris çarpımı %d, beklenen 1", geri)
	}
	if bayt != 6*4 {
		t.Errorf("geri matris çarpımının ayırdığı gradyan %d bayt, beklenen 24 (w'nin gradyanı)", bayt)
	}
	if g, _, _, _ := sayacFarki(true, IslemGorunum, func() {
		k := MatMul(w, x).Reshape(-1).SumAll()
		k.Backward()
	}); g != 1 {
		t.Errorf("geri görünüm %d, beklenen 1 (yeniden şekillendirme)", g)
	}
}

// Uzun işlemin (ilk çağrı her zaman ölçülür) sonraki bütün çağrıları da ölçülür.
func TestSayacBuyukIslemHerCagridaOlculur(t *testing.T) {
	k := &sayaclar[0][0][IslemMatrisCarpimi]
	k.geri, k.agr = 0, 0 // işlem türü ilk kez çalışıyormuş gibi (önceki testlerin küçük çarpımları)
	a := New(make([]float32, 128*128), 128, 128)
	cagri, olcum, _, sure := sayacFarki(false, IslemMatrisCarpimi, func() {
		for i := 0; i < 3; i++ {
			MatMul(a, a)
		}
	})
	if cagri != 3 || olcum != 3 {
		t.Errorf("büyük matris çarpımı: %d çağrı, %d ölçüm; beklenen 3 ve 3", cagri, olcum)
	}
	if sure <= 0 {
		t.Errorf("büyük matris çarpımının süresi %v, sıfırdan büyük olmalı", sure)
	}
}

// Kısa işlemlerde süre örneklenir: her çağrı p = 1/ornekAraligi olasılıkla ölçülür ve ölçülen
// süre 1/p ile ağırlıklanır. Ağırlıkların toplamı çağrı sayısının yansız tahminidir; ölçülen
// çağrı sayısı n·p civarındadır. Uzun işlemde (uzunIslemTik) her çağrı ölçülür.
func TestSayacOrneklemeAgirliklari(t *testing.T) {
	k := &sayaclar[0][0][IslemDiger]
	kimlik := *k
	*k = sayacKaydi{no: kimlik.no}
	defer func() { *k = kimlik }()
	const n = 64000
	var toplam, olculen int64
	for i := 0; i < n; i++ {
		s := sayacBasla(cpuBackend{}, IslemDiger).baslat()
		if s.k != nil {
			toplam += s.agirlik
			olculen++
		}
		s.bitir()
	}
	// Ölçülen çağrı sayısı Binom(n, 1/32): ortalama 2000, standart sapma ~44.
	if olculen < n/ornekAraligi*85/100 || olculen > n/ornekAraligi*115/100 {
		t.Errorf("%d çağrının %d'i ölçüldü; p = 1/%d ile ~%d beklenir", n, olculen, ornekAraligi, n/ornekAraligi)
	}
	if toplam < n*85/100 || toplam > n*115/100 {
		t.Errorf("ağırlıkların toplamı %d; çağrı sayısının (%d) yansız tahmini olmalı", toplam, n)
	}
	// Uzun işlem: son ölçüm sınırı aşınca her çağrı ölçülür.
	sonrakiOlcum(k, uzunIslemTik)
	for i := 0; i < 5; i++ {
		s := sayacBasla(cpuBackend{}, IslemDiger).baslat()
		if s.k == nil || s.agirlik != 1 {
			t.Fatalf("uzun işlemin %d. çağrısı ölçülmedi (ağırlık %d)", i+1, s.agirlik)
		}
		s.bas -= uzunIslemTik // uzun sürmüş gibi
		s.bitir()
	}
}

// Panikle biten işlemin etkin kaydı bir sonraki işleme kadar kalır (ertelemesiz sıcak yol);
// sonraki işlem bitince temizlenir ve sayaçlar doğru sayar.
func TestSayacEtkinIslemSonrakiIslemdeTemizlenir(t *testing.T) {
	x := New(make([]float32, 6), 2, 3)
	func() {
		defer func() { _ = recover() }()
		CrossEntropy(x, []int{0}) // 2 satır için 1 hedef: panik
	}()
	cagri, _, _, _ := sayacFarki(false, IslemCaprazEntropi, func() {
		CrossEntropy(x, []int{0, 1})
	})
	if cagri != 1 {
		t.Errorf("çapraz entropi çağrısı %d, beklenen 1", cagri)
	}
	if is, _, calisiyor := EtkinIslem(); calisiyor {
		t.Fatalf("işlem bittikten sonra etkin işlem %v kaldı", is)
	}
}

func TestSayacIslemAdlari(t *testing.T) {
	gorulen := map[string]bool{}
	for i := Islem(0); i < IslemSayisi; i++ {
		if i.Kod() == "" || i.String() == "" {
			t.Errorf("%d. işlemin adı eksik", i)
		}
		if gorulen[i.Kod()] {
			t.Errorf("%q kodu iki kez", i.Kod())
		}
		gorulen[i.Kod()] = true
	}
	if IslemToplama+Islem(OpDiv) != IslemBolme || IslemToplama+Islem(OpGe) != IslemBuyukEsit ||
		IslemUstel+Islem(OpCos) != IslemKosinus || IslemUstel+Islem(OpAbs) != IslemMutlak {
		t.Error("ikili ve tekli işlemlerin sırası BinOp/UnOp sırasıyla aynı olmalı")
	}
}
