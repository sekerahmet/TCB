package checker

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// Tip değeri metinle karşılaştırılmaz (docs/spec/03_tipler.md 3.11): tip(x) bir tip değeri
// verir, tip adları tırnaksız yazılır (tip(x) == Tam). tip(x) == "Tam" çalışma anında da
// hatadır; denetleyici, yerleşik tip() çağrısının bir metin sabitiyle karşılaştırıldığını
// çalıştırmadan önce bulur.

// tipMetinDenetle e, yerleşik tip()'in sonucunu bir metin sabitiyle (== ya da !=)
// karşılaştırıyorsa hata ekler.
func (c *Checker) tipMetinDenetle(e *ast.InfixExpression) {
	if e.Operator != "==" && e.Operator != "!=" {
		return
	}
	metin, ok := e.Right.(*ast.StringLiteral)
	cagri := e.Left
	if !ok {
		metin, ok = e.Left.(*ast.StringLiteral)
		cagri = e.Right
	}
	if !ok || !c.tipCagrisi(cagri) {
		return
	}
	c.errors = append(c.errors, fmt.Sprintf("Satır %d: %s", e.Line(),
		hint.TypeTextComparison(e.Operator, metin.Value, codes.Name(codes.A200))))
}

// tipCagrisi ifade, kullanıcının gölgelemediği yerleşik tip()'in bir çağrısı mı?
func (c *Checker) tipCagrisi(ifade ast.Expression) bool {
	call, ok := ifade.(*ast.CallExpression)
	if !ok {
		return false
	}
	ad, ok := call.Function.(*ast.Identifier)
	if !ok || ad.Value != codes.Name(codes.G201) {
		return false
	}
	_, golge := c.scope.Resolve(ad.Value)
	return !golge
}
