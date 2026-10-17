package lexer

import "testing"

// Her token türü sözlükte yalnızca bir kez tanımlanmalı ve en az bir yazımı olmalı.
func TestAnahtarKelimeSozluguDuzgun(t *testing.T) {
	for _, takim := range yazimTakimlari {
		gorulen := map[TokenType]bool{}
		for _, ak := range takim {
			if gorulen[ak.Tur] {
				t.Errorf("%s aynı takımda iki kez tanımlı", ak.Tur)
			}
			gorulen[ak.Tur] = true
			if len(ak.Yazimlar) == 0 {
				t.Errorf("%s için yazım yok", ak.Tur)
			}
		}
	}
}

// İki takımın aynı yazımı farklı türlere bağlaması başlangıçta yakalanmalı;
// aynı türe bağlaması ise sorun değildir.
func TestAnahtarKelimeHaritasiCakismayiYakalar(t *testing.T) {
	a := []AnahtarKelime{{TOKEN_EGER, []string{"x"}}}
	ayni := []AnahtarKelime{{TOKEN_EGER, []string{"x"}}}
	if h := anahtarKelimeHaritasi([][]AnahtarKelime{a, ayni}); h["x"] != TOKEN_EGER {
		t.Fatalf("aynı türe bağlanan yazım kabul edilmeliydi: %v", h)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("çakışan yazım için panik bekleniyordu")
		}
	}()
	anahtarKelimeHaritasi([][]AnahtarKelime{a, {{TOKEN_DONGU, []string{"x"}}}})
}
