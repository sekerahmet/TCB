package interp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// girdi() her çağrıda yeni bir tamponlu okuyucu açınca ilk okuma kaynağın tamamını
// tampona çekiyor, sonraki satırlar tamponla birlikte kayboluyordu. VM başına tek okuyucu
// kullanılır: art arda okumalar satırları sırayla verir.
func TestInputReadsConsecutiveLines(t *testing.T) {
	prog := mustParseProgram(t, `TR_var a = TR_input()
TR_var b = TR_input()
TR_var c = TR_input()
TR_print("a=[\(a)] b=[\(b)] c=[\(c)]")`)
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
func TestInputReaderShared(t *testing.T) {
	m := New()
	m.Stdin = strings.NewReader("komut\nveri\n")
	if s, _ := m.InputReader().ReadString('\n'); s != "komut\n" {
		t.Fatalf("ilk satır: %q", s)
	}
	val, err := m.Run(parser.New(source.TextLexer(`TR_return TR_input()`)).ParseProgram())
	if err != nil || val.Inspect() != "veri" {
		t.Fatalf("girdi() ikinci satırı okumalıydı: %v %v", val, err)
	}
	if r1, r2 := StdinReader(), StdinReader(); r1 != r2 {
		t.Fatal("standart giriş okuyucusu süreç başına tek olmalı")
	}
	if New().InputReader() != StdinReader() {
		t.Fatal("Stdin verilmemiş VM standart girişin ortak okuyucusunu kullanmalı")
	}
}
