package codeindex

import (
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// declaredIDs bir dosyanın üst düzey ifadelerinden dizinde olması gereken kimlikleri ast'yi
// doğrudan sayarak çıkarır (dizinin kendi kodundan bağımsız bir döküm).
func declaredIDs(rel string, p *parsed) []string {
	var ids []string
	for _, s := range p.program.Statements {
		switch s := s.(type) {
		case *ast.ExpressionStatement:
			if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Name != "" {
				ids = append(ids, NodeID(KindFunction, rel, fl.Name))
			}
		case *ast.VarStatement:
			kind := KindVariable
			if s.IsConst {
				kind = KindConstant
			}
			ids = append(ids, NodeID(kind, rel, s.Name.Value))
		case *ast.StructStatement:
			ids = append(ids, NodeID(KindStruct, rel, s.Name))
			for _, f := range s.Fields {
				ids = append(ids, NodeID(KindField, rel, s.Name+"."+f.Name))
			}
			for name := range s.Methods {
				ids = append(ids, NodeID(KindMethod, rel, s.Name+"."+name))
			}
		case *ast.TestStatement:
			ids = append(ids, NodeID(KindTest, rel, strconv.Itoa(s.Token.Line)))
		}
	}
	return ids
}
