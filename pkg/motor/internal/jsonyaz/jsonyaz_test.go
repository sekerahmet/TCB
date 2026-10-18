package jsonyaz

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"tr_dili/pkg/vm"
)

func sozluk(ciftler ...interface{}) *vm.DictVal {
	d := vm.NewDictVal()
	for i := 0; i < len(ciftler); i += 2 {
		d.Set(ciftler[i].(string), ciftler[i+1].(vm.Value))
	}
	return d
}

func liste(el ...vm.Value) *vm.ListVal { return &vm.ListVal{Elements: el} }

// Anahtarlar sözlükteki sırayla yazılır (eskiden Go map'i yüzünden alfabetik oluyordu).
func TestAnahtarSirasiKorunur(t *testing.T) {
	d := sozluk("b", vm.NewInt(1), "a", vm.NewInt(2))
	b, err := Yaz(d)
	if err != nil || string(b) != `{"b":1,"a":2}` {
		t.Fatalf("%s %v", b, err)
	}
}

// Biçim, alfabetik sıralı sözlükte encoding/json ile bayt bayt aynıdır (girintili ve girintisiz).
func TestEncodingJSONIleAyniBicim(t *testing.T) {
	d := sozluk("a", liste(vm.NewInt(1), &vm.FloatVal{Val: 0.5}, &vm.FloatVal{Val: 1e-7}, &vm.FloatVal{Val: 1e21}, &vm.FloatVal{Val: 2}),
		"b", sozluk(), "c", liste(), "d", vm.NULL, "e", vm.TRUE, "f", &vm.StringVal{Val: "<ş\"\n>"},
		"g", sozluk("x", liste(sozluk("y", vm.NewInt(-3)))))
	goDeger := vm.TRToGoValue(d)
	for _, girinti := range []string{"", "  "} {
		var bekle []byte
		if girinti == "" {
			bekle, _ = json.Marshal(goDeger)
		} else {
			bekle, _ = json.MarshalIndent(goDeger, "", girinti)
		}
		b, err := YazGirintili(d, girinti)
		if err != nil || string(b) != string(bekle) {
			t.Fatalf("girinti %q:\n%s\nbeklenen:\n%s (%v)", girinti, b, bekle, err)
		}
	}
}

func TestDongusVeDerinlikHata(t *testing.T) {
	l := liste(vm.NewInt(1))
	l.Elements = append(l.Elements, l)
	if _, err := Yaz(l); !errors.Is(err, vm.ErrDongusel) {
		t.Fatalf("döngü hatası bekleniyordu: %v", err)
	}
	d := vm.NewDictVal()
	d.Set("ben", liste(d))
	if _, err := Yaz(d); !errors.Is(err, vm.ErrDongusel) {
		t.Fatalf("dolaylı döngü hatası bekleniyordu: %v", err)
	}
	// Paylaşılan (döngüsüz) alt değer serbesttir
	ortak := liste(vm.NewInt(7))
	if b, err := Yaz(liste(ortak, ortak)); err != nil || string(b) != "[[7],[7]]" {
		t.Fatalf("%s %v", b, err)
	}
	var derin vm.Value = vm.NewInt(0)
	for i := 0; i < vm.DisDegerDerinligi+5; i++ {
		derin = liste(derin)
	}
	if _, err := Yaz(derin); err == nil || !strings.Contains(err.Error(), "çok derin") {
		t.Fatalf("derinlik hatası bekleniyordu: %v", err)
	}
}

func TestTanimsizVeSonsuzSayiHata(t *testing.T) {
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := Yaz(sozluk("d", &vm.FloatVal{Val: f}))
		if err == nil || !strings.Contains(err.Error(), "JSON'a yazılamaz") {
			t.Fatalf("%v için hata bekleniyordu: %v", f, err)
		}
	}
}

// Tam anahtar 1 ile metin anahtar "1" JSON'da aynı anahtardır: biri sessizce atılmaz.
func TestGorunusuAyniAnahtarHata(t *testing.T) {
	d := vm.NewDictVal()
	d.SetValue(vm.NewInt(1), vm.NewInt(10))
	d.Set("1", vm.NewInt(20))
	if _, err := Yaz(d); err == nil || !strings.Contains(err.Error(), "görünüşü aynı") {
		t.Fatalf("çakışma hatası bekleniyordu: %v", err)
	}
	tek := vm.NewDictVal()
	tek.SetValue(vm.NewInt(1), vm.NewInt(10))
	if b, err := Yaz(tek); err != nil || string(b) != `{"1":10}` {
		t.Fatalf("%s %v", b, err)
	}
}
