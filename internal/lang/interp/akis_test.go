package interp

import (
	"errors"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// akisDeger 0, 10, 20 ... değerlerini n tane üreten tembel bir kaynaktır (yurut.Akis);
// n < 0 ise ilk istekte hata verir. kac kaç öğe istendiğini sayar (tembellik sınaması).
type akisDeger struct{ i, n, kac int }

func (a *akisDeger) Type() ValueType { return "Akış" }
func (a *akisDeger) Inspect() string { return "akış" }
func (a *akisDeger) IsTruthy() bool  { return true }
func (a *akisDeger) Sonraki(v *VM) (Value, bool, error) {
	if a.n < 0 {
		return nil, false, errors.New("akış okunamadı")
	}
	if a.i >= a.n {
		return nil, false, nil
	}
	a.kac++
	a.i++
	return NewInt(int64((a.i - 1) * 10)), true, nil
}

// Akis kancası 'döngü x içinde', 'döngü i, x içinde' ve liste üretecinde kullanılır; öğeler
// istendikçe alınır (çık'tan sonra fazlası istenmez), döngü kaldığı yerden tüketir; kanca
// hatası satır bilgisi taşır.
func TestAkisKancasi(t *testing.T) {
	a := &akisDeger{n: 5}
	calistirKod := func(kod string) (*VM, error) {
		vmx := New()
		vmx.GlobalEnv.Define("a", a, false)
		vmx.GlobalEnv.Define("bozuk", &akisDeger{n: -1}, false)
		p := parser.New(lexer.New(kod))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		_, err := vmx.Run(prog)
		return vmx, err
	}
	vmx, err := calistirKod(`
değişken çiftler = []
döngü i, x içinde a {
    çiftler.ekle([i, x])
    eğer i == 1 { çık }
}
değişken kalan = [x + 1 döngü x içinde a eğer x > 20]
değişken boş = 0
döngü x içinde a { boş += 1 }
`)
	if err != nil {
		t.Fatal(err)
	}
	for ad, want := range map[string]string{
		"çiftler": "[[0, 0], [1, 10]]", "kalan": "[31, 41]", "boş": "0",
	} {
		if got, _ := vmx.GlobalEnv.Get(ad); got == nil || got.Inspect() != want {
			t.Errorf("%s: beklenen %s, alınan %v", ad, want, got)
		}
	}
	if a.kac != 5 {
		t.Errorf("akıştan %d öğe istendi, 5 bekleniyordu", a.kac)
	}
	if _, err := calistirKod("\ndöngü x içinde bozuk { }"); err == nil || err.Error() != "Satır 2: akış okunamadı" {
		t.Errorf("döngü kanca hatası satır önekli olmalı: %v", err)
	}
	if _, err := calistirKod("\n\nyazdır([x döngü x içinde bozuk])"); err == nil || err.Error() != "Satır 3: akış okunamadı" {
		t.Errorf("üreteç kanca hatası satır önekli olmalı: %v", err)
	}
}
