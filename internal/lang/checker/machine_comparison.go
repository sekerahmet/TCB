package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Makinenin işletim sistemi ve mimarisi kapalı tablodur (docs/spec/19_surec_ve_kayit.md 19.12):
// .TR_os "windows", "linux", "macos"; .TR_arch "x86_64", "arm64"; tablonun dışında Go'nun adı.
// m.TR_os == "darwin" çalışma anında hata vermeden TR_false olurdu; denetleyici karşılaştırmada
// doğrudan yazılan sabiti (==, != ve liste sabitinin .TR_contains'i) üyenin kümesiyle denetler.
// Değişkene ya da sabite atanmış metin ve liste denetlenmez.

// machineSpelling bir üyenin kümesi ve başka yazımlarıdır.
type machineSpelling struct {
	table []string
	set   map[string]bool
	// alt harf büyüklüğüne bakmadan eşleşen başka yazımdan tablo yazımına (DENETİM 643 Ş2).
	alt map[string]string
	// suggest başka yazımın iletisi (öneri), other kümede olmayanınki (önerisiz).
	suggest, other messages.Code
}

// Go adları motorun Go sürümünün (go1.27.4) "go tool dist list" çıktısındandır; tablodaki
// değerin Go adı ("darwin", "amd64") kümenin dışında kalır.
var (
	goOperatingSystems = strings.Fields("aix android dragonfly freebsd illumos ios js linux netbsd openbsd plan9 solaris wasip1 windows")
	goArchitectures    = strings.Fields("386 arm arm64 loong64 mips mips64 mips64le mipsle ppc64 ppc64le riscv64 s390x wasm")
)

var machineSpellings = map[codes.Code]*machineSpelling{
	codes.MemOs: newMachineSpelling([]string{"windows", "linux", "macos"}, goOperatingSystems,
		map[string]string{"darwin": "macos", "osx": "macos", "mac": "macos", "win32": "windows", "win": "windows"},
		messages.H7730, messages.H7732),
	codes.MemArch: newMachineSpelling([]string{"x86_64", "arm64"}, goArchitectures,
		map[string]string{"amd64": "x86_64", "x64": "x86_64", "x86-64": "x86_64", "aarch64": "arm64"},
		messages.H7731, messages.H7733),
}

// newMachineSpelling kümeyi kurar. Tablodaki bir değerin harf büyüklüğü farklı yazımı da başka
// yazımdır (DENETİM 645 N2): "Windows", "Linux", "macOS", "ARM64".
func newMachineSpelling(table, goNames []string, alt map[string]string, suggest, other messages.Code) *machineSpelling {
	s := &machineSpelling{table: table, set: map[string]bool{}, alt: map[string]string{}, suggest: suggest, other: other}
	for _, v := range append(append([]string(nil), table...), goNames...) {
		s.set[v] = true
	}
	for _, v := range table {
		s.alt[strings.ToLower(v)] = v
	}
	for k, v := range alt {
		s.alt[k] = v
	}
	return s
}

// checkMachineComparison e, .TR_os ya da .TR_arch'ı bir metin sabitiyle (== ya da !=)
// karşılaştırıyorsa sabiti denetler.
func (c *Checker) checkMachineComparison(e *ast.InfixExpression) {
	text, other, ok := textComparison(e)
	if !ok {
		return
	}
	if sp := machineMember(other); sp != nil {
		c.checkMachineText(sp, text)
	}
}

// checkMachineContains e, metin sabitlerinden bir liste sabitinin .TR_contains(m.TR_os)'u ise
// her metin öğesini denetler.
func (c *Checker) checkMachineContains(e *ast.CallExpression) {
	fn, ok := e.Function.(*ast.MemberExpression)
	if !ok || fn.NameToken.Code != codes.MemContains || len(e.Arguments) != 1 {
		return
	}
	list, ok := fn.Object.(*ast.ListLiteral)
	if !ok {
		return
	}
	sp := machineMember(e.Arguments[0])
	if sp == nil {
		return
	}
	for _, el := range list.Elements {
		if text, ok := el.(*ast.StringLiteral); ok {
			c.checkMachineText(sp, text)
		}
	}
}

// machineMember ifade .TR_os ya da .TR_arch üyesiyse onun kümesidir.
func machineMember(e ast.Expression) *machineSpelling {
	m, ok := e.(*ast.MemberExpression)
	if !ok {
		return nil
	}
	return machineSpellings[m.NameToken.Code]
}

// checkMachineText sabit üyenin kümesinde değilse hata ekler: üyenin başka yazımı tablo yazımını
// önerir; öbür üyenin değeri ve başka yazımı, harf büyüklüğü farklı Go adı ve bilinmeyen metin
// önerisizdir (önerisi kopyalanınca yine hata olurdu).
func (c *Checker) checkMachineText(sp *machineSpelling, text *ast.StringLiteral) {
	if sp.set[text.Value] {
		return
	}
	if want, ok := sp.alt[strings.ToLower(text.Value)]; ok {
		c.addErrorAt(text, messages.New(sp.suggest, text.Value, want))
		return
	}
	c.addErrorAt(text, messages.New(sp.other, text.Value))
}
