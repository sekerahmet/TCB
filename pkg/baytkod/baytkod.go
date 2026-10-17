// Package baytkod, 'tr bytecode' (baytkod) komutunun gösterdiği komut listesini üretir.
//
// Bu liste yalnızca gösterim içindir: yorumlayıcı (pkg/vm) programları doğrudan sözdizimi
// ağacı üzerinden çalıştırır ve burada üretilen komutları hiçbir zaman yürütmez. Paket,
// kullanıcıların bir programın hangi temel adımlara ayrıldığını görebilmesi için 'tr bytecode'
// komutuyla birlikte korunur; sözdizimi ağacından (pkg/ast) başka hiçbir pakete bağlı değildir.
package baytkod

import (
	"fmt"
	"strings"
	"tr_dili/pkg/ast"
)

type OpCode string

const (
	OP_YUKLE_SABIT      OpCode = "YÜKLE_SABİT"
	OP_TANIMLA_DEGISKEN OpCode = "TANIMLA_DEĞİŞKEN"
	OP_TANIMLA_SABIT    OpCode = "TANIMLA_SABİT"
	OP_ATA_DEGISKEN     OpCode = "ATA_DEĞİŞKEN"
	OP_OKU_DEGISKEN     OpCode = "OKU_DEĞİŞKEN"
	OP_TEKLI_ISLEM      OpCode = "TEKLİ_İŞLEM"
	OP_TOPLA            OpCode = "TOPLA"
	OP_CIKAR            OpCode = "ÇIKAR"
	OP_CARP             OpCode = "ÇARP"
	OP_BOL              OpCode = "BÖL"
	OP_MOD              OpCode = "MOD"
	OP_US               OpCode = "ÜS"
	OP_MATRIS_CARP      OpCode = "MATRİS_ÇARP"
	OP_KARSILASTIR      OpCode = "KARŞILAŞTIR"
	OP_BORU_AKTAR       OpCode = "BORU_AKTAR"
	OP_BOS_KORUMA       OpCode = "BOŞ_KORUMA"
	OP_LISTE_OLUSTUR    OpCode = "LİSTE_OLUŞTUR"
	OP_LISTE_URETECI    OpCode = "LİSTE_ÜRETECİ"
	OP_SOZLUK_OLUSTUR   OpCode = "SÖZLÜK_OLUŞTUR"
	OP_ARALIK_OLUSTUR   OpCode = "ARALIK_OLUŞTUR"
	OP_INDEKS_OKU       OpCode = "İNDEKS_OKU"
	OP_UYE_OKU          OpCode = "ÜYE_OKU"
	OP_FONKSIYON_TANIM  OpCode = "FONKSİYON_TANIMLA"
	OP_YAPI_TANIM       OpCode = "YAPI_TANIMLA"
	OP_CAGIR            OpCode = "ÇAĞIR"
	OP_KOSULLU_ATLA     OpCode = "KOŞULLU_ATLA"
	OP_ATLA             OpCode = "ATLA"
	OP_DONGU_BASLAT     OpCode = "DÖNGÜ_BAŞLAT"
	OP_DONGU_CIK        OpCode = "DÖNGÜ_ÇIK"
	OP_DONGU_DEVAM      OpCode = "DÖNGÜ_DEVAM"
	OP_DONDUR           OpCode = "DÖNDÜR"
	OP_DENE_BASLAT      OpCode = "DENE_BAŞLAT"
	OP_YAKALA_BLOK      OpCode = "YAKALA_BLOK"
	OP_FIRLAT           OpCode = "FIRLAT"
	OP_ERTELE           OpCode = "ERTELE"
	OP_MODUL_AKTAR      OpCode = "MODÜL_AKTAR"
	OP_KESME_NOKTASI    OpCode = "KESME_NOKTASI"
	OP_ESZAMANLI_BASLAT OpCode = "EŞZAMANLI_BAŞLAT"
)

type Instruction struct {
	Op      OpCode
	Operand string
	Line    int
}

type BytecodeChunk struct {
	Instructions []Instruction
	Constants    []string
}

func CompileToBytecode(prog *ast.Program) *BytecodeChunk {
	chunk := &BytecodeChunk{}
	for _, stmt := range prog.Statements {
		compileStmt(chunk, stmt)
	}
	return chunk
}

func (b *BytecodeChunk) Emit(op OpCode, operand string, line int) int {
	b.Instructions = append(b.Instructions, Instruction{
		Op:      op,
		Operand: operand,
		Line:    line,
	})
	return len(b.Instructions) - 1
}

func (b *BytecodeChunk) Disassemble() string {
	var sb strings.Builder
	sb.WriteString("=== TR SANAL MAKİNESİ (VM) BYTECODE ÇIKTISI ===\n")
	sb.WriteString(fmt.Sprintf("%-6s %-8s %-22s %s\n", "ADIM", "SATIR", "KOMUT (OPCODE)", "OPERAND"))
	sb.WriteString(strings.Repeat("-", 62) + "\n")
	for i, ins := range b.Instructions {
		sb.WriteString(fmt.Sprintf("%04d   %-8d %-22s %s\n", i, ins.Line, ins.Op, ins.Operand))
	}
	return sb.String()
}

func compileStmt(c *BytecodeChunk, stmt ast.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.VarStatement:
		compileExpr(c, s.Value)
		if s.IsConst {
			c.Emit(OP_TANIMLA_SABIT, s.Name.Value, s.Line())
		} else {
			c.Emit(OP_TANIMLA_DEGISKEN, s.Name.Value, s.Line())
		}
	case *ast.MultiVarStatement:
		for _, v := range s.Values {
			compileExpr(c, v)
		}
		var names []string
		for _, n := range s.Names {
			names = append(names, n.Value)
		}
		op := OP_TANIMLA_DEGISKEN
		if s.IsConst {
			op = OP_TANIMLA_SABIT
		}
		c.Emit(op, strings.Join(names, ", "), s.Line())
	case *ast.AssignStatement:
		compileExpr(c, s.Value)
		c.Emit(OP_ATA_DEGISKEN, fmt.Sprintf("%s (%s)", s.Target.TokenLiteral(), s.Operator), s.Line())
	case *ast.MultiAssignStatement:
		for _, v := range s.Values {
			compileExpr(c, v)
		}
		var targets []string
		for _, t := range s.Targets {
			targets = append(targets, t.TokenLiteral())
		}
		c.Emit(OP_ATA_DEGISKEN, strings.Join(targets, ", "), s.Line())
	case *ast.ReturnStatement:
		compileExpr(c, s.ReturnValue)
		c.Emit(OP_DONDUR, "", s.Line())
	case *ast.BreakStatement:
		c.Emit(OP_DONGU_CIK, "", s.Line())
	case *ast.ContinueStatement:
		c.Emit(OP_DONGU_DEVAM, "", s.Line())
	case *ast.BlockStatement:
		for _, bs := range s.Statements {
			compileStmt(c, bs)
		}
	case *ast.ForInStatement:
		compileExpr(c, s.Iterable)
		c.Emit(OP_DONGU_BASLAT, s.ValueVar, s.Line())
		for _, bs := range s.Body.Statements {
			compileStmt(c, bs)
		}
	case *ast.WhileStatement:
		compileExpr(c, s.Condition)
		c.Emit(OP_DONGU_BASLAT, "iken", s.Line())
		for _, bs := range s.Body.Statements {
			compileStmt(c, bs)
		}
	case *ast.StructStatement:
		c.Emit(OP_YAPI_TANIM, s.Name, s.Line())
		for mName, fn := range s.Methods {
			c.Emit(OP_FONKSIYON_TANIM, s.Name+"."+mName, fn.Line())
			for _, bs := range fn.Body.Statements {
				compileStmt(c, bs)
			}
		}
	case *ast.TryCatchStatement:
		c.Emit(OP_DENE_BASLAT, "", s.Line())
		if s.TryBody != nil {
			for _, bs := range s.TryBody.Statements {
				compileStmt(c, bs)
			}
		}
		c.Emit(OP_YAKALA_BLOK, s.ErrorVar, s.Line())
		if s.CatchBody != nil {
			for _, bs := range s.CatchBody.Statements {
				compileStmt(c, bs)
			}
		}
	case *ast.ThrowStatement:
		compileExpr(c, s.Value)
		c.Emit(OP_FIRLAT, "", s.Line())
	case *ast.DeferStatement:
		c.Emit(OP_ERTELE, "ertele_bloğu", s.Line())
		if s.Body != nil {
			for _, bs := range s.Body.Statements {
				compileStmt(c, bs)
			}
		} else if s.Call != nil {
			compileExpr(c, s.Call)
		}
	case *ast.ImportStatement:
		operand := s.Path
		if s.Alias != "" {
			operand += " olarak " + s.Alias
		}
		c.Emit(OP_MODUL_AKTAR, operand, s.Line())
	case *ast.BreakpointStatement:
		if s.Condition != nil {
			compileExpr(c, s.Condition)
			c.Emit(OP_KESME_NOKTASI, "koşullu", s.Line())
		} else {
			c.Emit(OP_KESME_NOKTASI, "kesme_noktası", s.Line())
		}
	case *ast.AsyncStatement:
		c.Emit(OP_ESZAMANLI_BASLAT, "görev", s.Line())
		if s.Body != nil {
			for _, bs := range s.Body.Statements {
				compileStmt(c, bs)
			}
		} else if s.Call != nil {
			compileExpr(c, s.Call)
		}
	case *ast.ExpressionStatement:
		compileExpr(c, s.Expression)
	}
}

func compileExpr(c *BytecodeChunk, expr ast.Expression) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		c.Emit(OP_YUKLE_SABIT, fmt.Sprintf("Tam(%d)", e.Value), e.Line())
	case *ast.FloatLiteral:
		c.Emit(OP_YUKLE_SABIT, fmt.Sprintf("Ondalık(%g)", e.Value), e.Line())
	case *ast.StringLiteral:
		c.Emit(OP_YUKLE_SABIT, fmt.Sprintf("Metin(%q)", e.Value), e.Line())
	case *ast.InterpolatedString:
		for _, p := range e.Parts {
			compileExpr(c, p)
		}
		c.Emit(OP_TOPLA, fmt.Sprintf("MetinBirleştir(%d parça)", len(e.Parts)), e.Line())
	case *ast.BooleanLiteral:
		c.Emit(OP_YUKLE_SABIT, fmt.Sprintf("Mantık(%v)", e.Value), e.Line())
	case *ast.NullLiteral:
		c.Emit(OP_YUKLE_SABIT, "Yok", e.Line())
	case *ast.Identifier:
		c.Emit(OP_OKU_DEGISKEN, e.Value, e.Line())
	case *ast.ListLiteral:
		for _, el := range e.Elements {
			compileExpr(c, el)
		}
		c.Emit(OP_LISTE_OLUSTUR, fmt.Sprintf("%d eleman", len(e.Elements)), e.Line())
	case *ast.ListComprehension:
		compileExpr(c, e.Iterable)
		if e.Condition != nil {
			compileExpr(c, e.Condition)
		}
		compileExpr(c, e.Output)
		c.Emit(OP_LISTE_URETECI, e.VarName, e.Line())
	case *ast.DictLiteral:
		for _, pair := range e.Pairs {
			compileExpr(c, pair.Key)
			compileExpr(c, pair.Value)
		}
		c.Emit(OP_SOZLUK_OLUSTUR, fmt.Sprintf("%d çift", len(e.Pairs)), e.Line())
	case *ast.RangeExpression:
		compileExpr(c, e.Start)
		compileExpr(c, e.End)
		c.Emit(OP_ARALIK_OLUSTUR, e.Token.Literal, e.Line())
	case *ast.PrefixExpression:
		compileExpr(c, e.Right)
		c.Emit(OP_TEKLI_ISLEM, e.Operator, e.Line())
	case *ast.InfixExpression:
		compileExpr(c, e.Left)
		compileExpr(c, e.Right)
		switch e.Operator {
		case "+":
			c.Emit(OP_TOPLA, "+", e.Line())
		case "-":
			c.Emit(OP_CIKAR, "-", e.Line())
		case "*":
			c.Emit(OP_CARP, "*", e.Line())
		case "/":
			c.Emit(OP_BOL, "/", e.Line())
		case "%":
			c.Emit(OP_MOD, "%", e.Line())
		case "**":
			c.Emit(OP_US, "**", e.Line())
		case "@":
			c.Emit(OP_MATRIS_CARP, "@", e.Line())
		case "|>":
			c.Emit(OP_BORU_AKTAR, "|>", e.Line())
		case "??":
			c.Emit(OP_BOS_KORUMA, "??", e.Line())
		default:
			c.Emit(OP_KARSILASTIR, e.Operator, e.Line())
		}
	case *ast.IndexExpression:
		compileExpr(c, e.Left)
		compileExpr(c, e.Index)
		c.Emit(OP_INDEKS_OKU, "[]", e.Line())
	case *ast.MemberExpression:
		compileExpr(c, e.Object)
		op := "." + e.Property
		if e.Safe {
			op = "?." + e.Property
		}
		c.Emit(OP_UYE_OKU, op, e.Line())
	case *ast.FunctionLiteral:
		name := e.Name
		if name == "" {
			name = "<lambda>"
		}
		c.Emit(OP_FONKSIYON_TANIM, name, e.Line())
		for _, bs := range e.Body.Statements {
			compileStmt(c, bs)
		}
	case *ast.CallExpression:
		for _, arg := range e.Arguments {
			compileExpr(c, arg)
		}
		for _, na := range e.NamedArgs {
			compileExpr(c, na.Value)
		}
		c.Emit(OP_CAGIR, fmt.Sprintf("%s(%d arg)", e.Function.TokenLiteral(), len(e.Arguments)+len(e.NamedArgs)), e.Line())
	case *ast.IfExpression:
		compileExpr(c, e.Condition)
		c.Emit(OP_KOSULLU_ATLA, "değilse_kolu", e.Line())
		for _, bs := range e.Consequence.Statements {
			compileStmt(c, bs)
		}
		for _, elif := range e.ElseIfs {
			compileExpr(c, elif.Condition)
			c.Emit(OP_KOSULLU_ATLA, "değilse_eğer_kolu", e.Line())
			for _, bs := range elif.Consequence.Statements {
				compileStmt(c, bs)
			}
		}
		if e.Alternative != nil {
			for _, bs := range e.Alternative.Statements {
				compileStmt(c, bs)
			}
		}
	case *ast.MatchExpression:
		compileExpr(c, e.Subject)
		c.Emit(OP_KARSILASTIR, fmt.Sprintf("eşle (%d durum)", len(e.Cases)), e.Line())
		for _, mc := range e.Cases {
			if mc.Body != nil {
				for _, bs := range mc.Body.Statements {
					compileStmt(c, bs)
				}
			} else if mc.ValueExpr != nil {
				compileExpr(c, mc.ValueExpr)
			}
		}
	}
}
