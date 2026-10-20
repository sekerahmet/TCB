package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- FONKSİYONLAR ---

// funcCode bir fonksiyonun derlenmiş hâlidir; o fonksiyondan üretilen tüm değerler
// (her kapanış) paylaşır.
type funcCode struct {
	info      *scopeInfo
	bodyBlock *blockCode
	// param[i] i. parametrenin yuvası; varsayilan[i] varsayılan değer kodu (yoksa nil).
	// Varsayılanlar çağrının kendi ortamında, önceki parametreler bağlandıktan sonra
	// değerlendirilir.
	param      []int
	defaultVal []exprCode
	self       int // metotlarda 'kendi'nin yuvası, değilse -1
	// simple: parametre yuvaları birbirinden ve 'kendi'den farklı; argümanlar doğrudan
	// yuvalara yazılabilir (bağlama sırası sonucu değiştirmez).
	simple   bool
	hasDefer bool
	// pooled: gövde ortamı yakalayamaz (iç fonksiyon, ertele, eşzamanlı, yapı yok); çağrı
	// bitince ortam temizlenip sonraki çağrılarda yeniden kullanılabilir. serbest bu
	// ortamların yığınıdır (yalnızca yorumlayıcı kilidi tutulurken erişilir).
	pooled bool
	free   []*Environment
	// source fonksiyonun tanımlandığı dosyadır (nil: ana program); çağıranınkinden farklıysa
	// çağrı o dosyanın bağlamında yapılır (kaynaktaCagir).
	source *Source
}

// poolLimit bir fonksiyon için saklanan boş ortam sayısının üst sınırıdır.
const poolLimit = 64

// getEnv fonksiyonun bir çağrısı için ortam verir (varsa havuzdan).
func (fk *funcCode) getEnv(outer *Environment) *Environment {
	if n := len(fk.free); n > 0 {
		e := fk.free[n-1]
		fk.free = fk.free[:n-1]
		if len(e.vals) == len(fk.info.nameList) {
			e.outer = outer
			return e
		}
	}
	return newEnv(outer, fk.info)
}

// release biten bir çağrının ortamını havuza geri koyar.
func (fk *funcCode) release(e *Environment) {
	if len(fk.free) >= poolLimit {
		return
	}
	clear(e.vals)
	e.outer = nil
	e.extra = nil
	fk.free = append(fk.free, e)
}

// function bir fonksiyon gövdesini, tanımlandığı k kapsamının içinde derler.
func (c *compiler) function(k *compilerScope, params []ast.Param, body *ast.BlockStatement, meth bool) *funcCode {
	sub := subScope(k)
	fk := &funcCode{
		param:      make([]int, len(params)),
		defaultVal: make([]exprCode, len(params)),
		self:       -1,
		simple:     true,
		source:     c.source,
	}
	if meth {
		fk.self = sub.info.slot(selfName)
	}
	seenSet := map[int]bool{}
	if fk.self >= 0 {
		seenSet[fk.self] = true
	}
	paramLine := 0
	if body != nil {
		paramLine = body.Line()
	}
	for i, p := range params {
		fk.param[i] = sub.info.slot(p.Name)
		sub.recordShadow(p.Name, paramLine, messages.H2451)
		if seenSet[fk.param[i]] {
			fk.simple = false
		}
		seenSet[fk.param[i]] = true
	}
	// Varsayılanlar fonksiyonun kendi kapsamında derlenir ve çağrının ortamında, soldan
	// sağa hesaplanır: önceki parametreleri görür; parametrenin kendisi ve sonrakiler henüz
	// yoktur (dıştaki aynı adlı değişkene çözülmez).
	for i, p := range params {
		if p.DefaultValue == nil {
			continue
		}
		notYet := make(map[string]bool, len(params)-i)
		for _, s := range params[i:] {
			notYet[s.Name] = true
		}
		vk := &compilerScope{info: sub.info, parent: sub.parent, notYet: notYet}
		fk.defaultVal[i] = c.expression(vk, p.DefaultValue)
	}
	var stmts []ast.Statement
	if body != nil {
		stmts = body.Statements
	}
	sub.predefine(stmts)
	fk.bodyBlock = c.statements(sub, stmts)
	fk.info = sub.info
	fk.hasDefer = bodyHasDefer(stmts)
	fk.pooled = true
	for _, st := range stmts {
		if c.blocks.statementCaptures(st) {
			fk.pooled = false
			break
		}
	}
	return fk
}

// prepareFuncCode derleyici dışında oluşturulmuş bir fonksiyon değerini (kod alanı boş)
// adla çözümlemeyle derler.
func (v *VM) prepareFuncCode(fn *FuncVal) *funcCode {
	fk := v.newCompiler().function(boundaryScope(), fn.Parameters, fn.Body, fn.BoundSelf != nil)
	fn.codeVal = fk
	return fk
}
