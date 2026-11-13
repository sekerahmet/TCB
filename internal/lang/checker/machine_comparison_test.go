package checker

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// machineErrors kodlu gövdeyi denetler ve hataları verir.
func machineErrors(t *testing.T, source string) []error {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi: %v\n%s", p.Errors(), source)
	}
	c := New()
	c.CheckProgram(prog)
	return c.Errs()
}

// oneMachineError gövdenin tek hatasının kodunu ve parametrelerini verir.
func oneMachineError(t *testing.T, source string) (messages.Code, []any) {
	t.Helper()
	errs := machineErrors(t, source)
	if len(errs) != 1 {
		t.Fatalf("tek hata bekleniyordu, %d: %v\n%s", len(errs), errs, source)
	}
	e, ok := messages.As(errs[0])
	if !ok {
		t.Fatalf("kodlu hata değil: %v", errs[0])
	}
	return e.Code, e.Args
}

// distList "go tool dist list"in (go1.27.4) işletim sistemi ve mimari sütunlarıdır
// (dost-indirme-deneme/dist-list.txt, 47 hedef).
var (
	distOS   = strings.Fields("aix android darwin dragonfly freebsd illumos ios js linux netbsd openbsd plan9 solaris wasip1 windows")
	distArch = strings.Fields("386 amd64 arm arm64 loong64 mips mips64 mips64le mipsle ppc64 ppc64le riscv64 s390x wasm")
)

// TestMachineSpellingRule .TR_os ve .TR_arch'ın sabitle karşılaştırması (19.12; DENETİM 640 Ş1,
// 643 Ş2, 645 N2): iki küme dist list'in iki sütunuyla aynı (tablonun Go adı dışarıda); kendi
// başka yazımı ve tablo değerinin harf büyüklüğü farklı yazımı doğru öneriyi verir; öbür üyenin
// değeri, başka yazımı ve bilinmeyen metin önerisiz hatadır.
func TestMachineSpellingRule(t *testing.T) {
	type member struct {
		code           codes.Code
		name           string
		table, goNames []string
		alt            map[string]string
		suggest, other messages.Code
	}
	osm := member{codes.MemOs, "TR_os", []string{"windows", "linux", "macos"}, distOS,
		map[string]string{"darwin": "macos", "osx": "macos", "mac": "macos", "win32": "windows", "win": "windows",
			"Windows": "windows", "Linux": "linux"}, messages.H7730, messages.H7732}
	arch := member{codes.MemArch, "TR_arch", []string{"x86_64", "arm64"}, distArch,
		map[string]string{"amd64": "x86_64", "x64": "x86_64", "x86-64": "x86_64", "aarch64": "arm64"},
		messages.H7731, messages.H7733}
	for _, pair := range [][2]member{{osm, arch}, {arch, osm}} {
		own, other := pair[0], pair[1]
		// Küme: tablo ve Go adları, tablonun Go adı ("darwin", "amd64") dışarıda.
		var want []string
		want = append(want, own.table...)
		for _, g := range own.goNames {
			if _, isAlt := own.alt[g]; !isAlt {
				want = append(want, g)
			}
		}
		for _, v := range want {
			for _, form := range []string{"m.%s == %q", "m.%s != %q", "%[2]q == m.%[1]s", "[%[2]q].TR_contains(m.%[1]s)"} {
				src := "TR_var m = TR_machine()\nTR_print(" + fmt.Sprintf(form, own.name, v) + ")\n"
				if errs := machineErrors(t, src); len(errs) != 0 {
					t.Errorf("%s: %q kümede, hata beklenmiyordu: %v", own.name, v, errs)
				}
			}
		}
		check := func(v string, code messages.Code, suggestion string) {
			t.Helper()
			for _, expr := range []string{
				fmt.Sprintf("m.%s == %q", own.name, v),
				fmt.Sprintf("[%q, %q].TR_contains(m.%s)", own.table[0], v, own.name),
			} {
				got, args := oneMachineError(t, "TR_var m = TR_machine()\nTR_print("+expr+")\n")
				if got != code {
					t.Errorf("%s %q: %s, beklenen %s", own.name, v, got, code)
					continue
				}
				if suggestion != "" && (len(args) != 2 || args[1] != suggestion) {
					t.Errorf("%s %q: öneri %v, beklenen %q", own.name, v, args, suggestion)
				}
				if suggestion == "" && len(args) != 1 {
					t.Errorf("%s %q: önerisiz bekleniyordu: %v", own.name, v, args)
				}
			}
		}
		for v, s := range own.alt {
			check(v, own.suggest, s)
		}
		for _, v := range own.table {
			check(strings.ToUpper(v), own.suggest, v)
			check(strings.ToUpper(v[:1])+v[1:], own.suggest, v)
		}
		// Öbür üyenin değeri, başka yazımı ve tablo değerinin harf büyüklüğü farklı yazımı önerisiz.
		for _, v := range other.table {
			if !slices.Contains(own.goNames, v) {
				check(v, own.other, "")
			}
			check(strings.ToUpper(v), own.other, "")
		}
		for v := range other.alt {
			if !slices.Contains(own.goNames, v) {
				check(v, own.other, "")
			}
		}
		check("beos", own.other, "")
		check("", own.other, "")
	}
	// Tablo dışındaki Go adının harf büyüklüğü farklı yazımı başka yazım değildir.
	if got, args := oneMachineError(t, "TR_var m = TR_machine()\nTR_print(m.TR_os == \"FreeBSD\")\n"); got != messages.H7732 || len(args) != 1 {
		t.Errorf("FreeBSD: %s %v", got, args)
	}
}

// TestMachineRuleSeesOnlyLiterals değişkene ya da sabite atanmış metin ve liste denetlenmez
// (kararlar sürüm 6, DENETİM 639 N5); üyenin adı başka bir yerde metinle karşılaştırılmaz.
func TestMachineRuleSeesOnlyLiterals(t *testing.T) {
	for _, src := range []string{
		"TR_var m = TR_machine()\nTR_var a = \"darwin\"\nTR_print(m.TR_os == a)\n",
		"TR_var m = TR_machine()\nTR_const OSLAR = [\"windows\", \"darwin\"]\nTR_print(OSLAR.TR_contains(m.TR_os))\n",
		"TR_var m = TR_machine()\nTR_print([\"windows\", 3].TR_contains(m.TR_os))\n",
		"TR_var m = TR_machine()\nTR_print(m.TR_os + \"x\" == \"darwinx\")\n",
		"TR_print([\"darwin\"].TR_contains(\"darwin\"))\n",
	} {
		if errs := machineErrors(t, src); len(errs) != 0 {
			t.Errorf("hata beklenmiyordu: %v\n%s", errs, src)
		}
	}
}
