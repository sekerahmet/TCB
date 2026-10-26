package main

import (
	"go/ast"
	"go/types"
	"slices"
)

// Bu dosya metinle seçilen kollar: komut adı bir dize olarak (`run([]string{"who-uses", ...})`) dağıtıcının `switch cmd`'ine
// gider ve `case "who-uses":` kolu printFieldUses'ı çağırır. Çağrı zinciri testten dağıtıcıya kadar görünür, ama hangi kolun
// koştuğu metindedir: zincir her kolu aynı uzaklıkta, dağıtıcının ötesinde gösterir (eksikler listesi, ince ayar: query_fields.go'nun
// en yakın testi 5 adımdı). Kural: bir test (ya da test dosyasındaki yardımcısı) bir dize kolunun değerini metin olarak taşıyor
// ve kolun işlevine ulaşıyorsa, kolun çağırdığı modül işlevlerini doğrudan sınar (`tests` [INFERRED]).

// caseCall bir dize kolunun değeri ve kolda çağrılan modül işlevleridir.
type caseCall struct {
	owner, file string
	line        int
	values      []string
	callees     []string
}

// testText bir test dosyasındaki işlevin taşıdığı dize değişmezidir.
type testText struct{ owner, value string }

// noteCaseCalls bir switch kolunun dize değerlerini ve kolda çağrılan modül işlevlerini kaydeder.
func (x *goExtractor) noteCaseCalls(fc *fileCtx, owner string, cc *ast.CaseClause) {
	if fc.isTest {
		return
	}
	var values []string
	for _, e := range cc.List {
		if v, ok := stringLiteral(fc, e); ok {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return
	}
	var callees []string
	for _, st := range cc.Body {
		ast.Inspect(st, func(n ast.Node) bool {
			if _, nested := n.(*ast.FuncLit); nested {
				return false
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if id := nameIdent(call.Fun); id != nil {
					if f, ok := fc.u.Info.Uses[id].(*types.Func); ok {
						if ref := x.objectID(f); ref != "" {
							callees = append(callees, ref)
						}
					}
				}
			}
			return true
		})
	}
	if len(callees) > 0 {
		x.idx.caseCalls = append(x.idx.caseCalls, caseCall{owner: owner, file: fc.rel, line: x.line(cc.Pos()), values: values, callees: callees})
	}
}

// noteTestText test dosyasındaki bir dize değişmezini sahibiyle kaydeder.
func (x *goExtractor) noteTestText(fc *fileCtx, owner string, lit *ast.BasicLit) {
	if !fc.isTest {
		return
	}
	if v, ok := stringLiteral(fc, lit); ok {
		x.idx.testTexts = append(x.idx.testTexts, testText{owner, v})
	}
}

// linkCaseTests kolun değerini taşıyan ve kolun işlevine ulaşan testlerden koldaki çağrılara `tests` kenarı çeker.
func linkCaseTests(b *builder, idx *goIndex, reach *testReach) {
	values := map[string]bool{}
	for _, c := range idx.caseCalls {
		for _, v := range c.values {
			values[v] = true
		}
	}
	carriers := map[string]map[string]bool{} // değer -> onu taşıyan testler
	for _, t := range idx.testTexts {
		if !values[t.value] {
			continue
		}
		if carriers[t.value] == nil {
			carriers[t.value] = map[string]bool{}
		}
		for _, test := range reach.testsReaching(t.owner) {
			carriers[t.value][test] = true
		}
	}
	for _, c := range idx.caseCalls {
		for _, test := range reach.testsReaching(c.owner) {
			if !slices.ContainsFunc(c.values, func(v string) bool { return carriers[v][test] }) {
				continue
			}
			for _, callee := range c.callees {
				b.addEdge(test, callee, edgeTests, inferred, c.file, c.line)
			}
		}
	}
}
