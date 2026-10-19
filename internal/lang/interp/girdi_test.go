package interp

import (
	"bytes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"strings"
	"testing"
)

// girdi() her çağrıda yeni bir tamponlu okuyucu açınca ilk okuma kaynağın tamamını
// tampona çekiyor, sonraki satırlar tamponla birlikte kayboluyordu. VM başına tek okuyucu
// kullanılır: art arda okumalar satırları sırayla verir.
func TestGirdiArdArdaSatirlariOkur(t *testing.T) {
	p := parser.New(lexer.New(`değişken a = girdi()
değişken b = girdi()
değişken c = girdi()
yazdır("a=[{a}] b=[{b}] c=[{c}]")`))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	var out bytes.Buffer
	m := New()
	m.Out = &out
	m.Stdin = strings.NewReader("bir\r\niki\n")
	if _, err := m.Run(prog); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "a=[bir] b=[iki] c=[]" {
		t.Fatalf("girdi satırları: %q", got)
	}
}

// Kabuk ve hata ayıklayıcı da VM'in okuyucusunu kullanır: girdi() ile birlikte okununca
// satır kaybolmaz.
func TestGirdiOkuyucusuPaylasilir(t *testing.T) {
	m := New()
	m.Stdin = strings.NewReader("komut\nveri\n")
	if s, _ := m.GirdiOkuyucu().ReadString('\n'); s != "komut\n" {
		t.Fatalf("ilk satır: %q", s)
	}
	val, err := m.Run(parser.New(lexer.New(`sonuç girdi()`)).ParseProgram())
	if err != nil || val.Inspect() != "veri" {
		t.Fatalf("girdi() ikinci satırı okumalıydı: %v %v", val, err)
	}
	if StdinOkuyucu() != StdinOkuyucu() {
		t.Fatal("standart giriş okuyucusu süreç başına tek olmalı")
	}
	if New().GirdiOkuyucu() != StdinOkuyucu() {
		t.Fatal("Stdin verilmemiş VM standart girişin ortak okuyucusunu kullanmalı")
	}
}
