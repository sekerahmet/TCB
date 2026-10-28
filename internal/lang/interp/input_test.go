package interp

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

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
	_, err := m.Run(parser.New(source.TextLexer(`TR_var result = TR_input()`)).ParseProgram())
	val, _ := m.GlobalEnv.Get("result")
	if err != nil || val == nil || val.Inspect() != "veri" {
		t.Fatalf("girdi() ikinci satırı okumalıydı: %v %v", val, err)
	}
	if r1, r2 := StdinReader(), StdinReader(); r1 != r2 {
		t.Fatal("standart giriş okuyucusu süreç başına tek olmalı")
	}
	if New().InputReader() != StdinReader() {
		t.Fatal("Stdin verilmemiş VM standart girişin ortak okuyucusunu kullanmalı")
	}
}

// Bir durdurma girdi() beklemesini bırakır (1.5); bırakılan okuma sürer ve okuduğu satırı sonra
// girdi isteyen ertelenen iş alır, satır yitmez.
func TestAbandonedInputKeepsTheLine(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	m := New()
	m.Stdin = reader
	var out bytes.Buffer
	m.Out = &out
	stopAfter(m, 2)
	prog := mustParseProgram(t, `# TR 4
TR_defer { TR_print("ertele: \(TR_input())") }
hazır()
TR_input()
`)
	done := make(chan error, 1)
	go func() {
		_, err := m.Run(prog)
		done <- err
	}()
	time.Sleep(300 * time.Millisecond) // durdurma ilk girdi() beklerken gelir, ertelenen iş girdi bekler
	if _, err := writer.Write([]byte("bir\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrInterrupted) || !strings.Contains(out.String(), "ertele: bir") {
			t.Fatalf("ertelenen iş bırakılan okumanın satırını almalıydı: %v %q", err, out.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("program 30 sn içinde bitmedi")
	}
}
