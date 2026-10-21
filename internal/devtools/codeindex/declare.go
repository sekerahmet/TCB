package codeindex

import (
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// declare bir dosyanın üst düzey bildirimlerini düğüm yapar ve `aktar` ifadelerini toplar.
func (x *indexer) declare(f *file) {
	mod := NodeID(KindModule, f.rel, "")
	for _, s := range f.res.program.Statements {
		switch s := s.(type) {
		case *ast.ExpressionStatement:
			if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Name != "" {
				x.def(f, mod, KindFunction, fl.Name, fl.Name, f.base+"."+fl.Name, signature(fl.Token.Literal, fl), fl.Token.Line, fl.Token.Column)
			}
		case *ast.VarStatement:
			x.valueNode(f, s.Name.Value, s.IsConst, s.Token.Line, s.Token.Column)
		case *ast.MultiVarStatement:
			for _, n := range s.Names {
				x.valueNode(f, n.Value, s.IsConst, s.Token.Line, s.Token.Column)
			}
		case *ast.StructStatement:
			x.structNodes(f, s)
		case *ast.TestStatement:
			line := strconv.Itoa(s.Token.Line)
			x.def(f, mod, KindTest, line, s.Name, f.base+": "+s.Name, s.Token.Literal+" "+strconv.Quote(s.Name), s.Token.Line, s.Token.Column)
		case *ast.ImportStatement:
			f.imports = append(f.imports, importRef{file: x.importTarget(f, s.Path), alias: s.Alias,
				pos: f.importPos(s)})
		}
	}
}

// def bir bildirimi düğüm yapar, üstünden `defines` kenarı çeker ve (üst düzeyse) ada kaydeder.
// idName kimliğe girer, name düğümün kanonik adıdır.
func (x *indexer) def(f *file, parent string, kind Kind, idName, name, qualified, signature string, line, col int) string {
	id := NodeID(kind, f.rel, idName)
	end, endCol := x.blockEnd(f, line)
	if kind == KindConstant || kind == KindVariable || kind == KindField {
		end, endCol = valueEnd(f, line) // değer bildiriminin sonu, sonraki bloğun sonu değil
	}
	pos := &Pos{File: f.rel, Line: line, Col: col, EndLine: end, EndCol: endCol}
	x.nodes[id] = &Node{ID: id, Kind: kind, Name: name, Pos: pos, Signature: signature, Qualified: qualified, Doc: docAbove(f, line)}
	x.addEdge(parent, id, EdgeDefines, Extracted, Pos{File: f.rel, Line: line})
	x.occurrence(id, RoleDefinition, Extracted, namePos(f, kind, name, line, col), parent)
	if parent == NodeID(KindModule, f.rel, "") && kind != KindTest {
		f.defs[name] = id
	}
	return id
}

func (x *indexer) valueNode(f *file, name string, isConst bool, line, col int) {
	kind, word := KindVariable, "var"
	if isConst {
		kind, word = KindConstant, "const"
	}
	x.def(f, NodeID(KindModule, f.rel, ""), kind, name, name, f.base+"."+name, word+" "+name, line, col)
}

func (x *indexer) structNodes(f *file, s *ast.StructStatement) {
	sid := x.def(f, NodeID(KindModule, f.rel, ""), KindStruct, s.Name, s.Name, f.base+"."+s.Name, s.Token.Literal+" "+s.Name, s.Token.Line, s.Token.Column)
	for _, fld := range s.Fields {
		line, col := fieldLine(f, fld.Name, s.Token.Line)
		x.def(f, sid, KindField, s.Name+"."+fld.Name, s.Name+"."+fld.Name, f.base+"."+s.Name+"."+fld.Name, fld.Name, line, col)
	}
	names := make([]string, 0, len(s.Methods))
	for n := range s.Methods {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fl := s.Methods[n]
		mid := x.def(f, sid, KindMethod, s.Name+"."+n, s.Name+"."+n, f.base+"."+s.Name+"."+n, signature(fl.Token.Literal, fl), fl.Token.Line, fl.Token.Column)
		f.methods[n] = append(f.methods[n], mid)
	}
}

// signature "<anahtar sözcük> ad(parametreler)" biçiminde bir işlev özetidir; anahtar sözcük
// kaynaktaki sözcüğün kendisidir.
func signature(word string, fl *ast.FunctionLiteral) string {
	params := make([]string, len(fl.Parameters))
	for i, p := range fl.Parameters {
		params[i] = p.Name
		if p.DefaultValue != nil {
			params[i] += " = …"
		}
	}
	return word + " " + fl.Name + "(" + strings.Join(params, ", ") + ")"
}

// fieldLine yapı alanının konumudur: yapının başlığından sonra adla başlayan ilk satır; sütun adın
// satırdaki kod noktası sırasıdır.
func fieldLine(f *file, name string, from int) (line, col int) {
	for i := from; i < len(f.lines) && i < from+400; i++ {
		t := strings.TrimSpace(f.lines[i])
		if strings.HasPrefix(t, name) && (len(t) == len(name) || !isIdentByte(t[len(name)])) {
			return i + 1, 1 + utf8.RuneCountInString(f.lines[i][:strings.Index(f.lines[i], t)])
		}
	}
	return from, 0
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// blockEnd bildirimin başladığı satırdan, ilk `{`yi kapatan `}` satırına kadar olan son satırdır
// (metin ve yorumların içindeki parantezler sayılmaz) ve o satırın kod noktası uzunluğudur; `{`
// yoksa başlangıç satırı.
func (x *indexer) blockEnd(f *file, start int) (line, endCol int) {
	depth, opened := 0, false
	for i := start - 1; i < len(f.lines); i++ {
		text := f.lines[i]
		inStr := false
		for j := 0; j < len(text); j++ {
			c := text[j]
			switch {
			case inStr:
				if c == '\\' {
					j++
				} else if c == '"' {
					inStr = false
				}
			case c == '"':
				inStr = true
			case c == '/' && j+1 < len(text) && text[j+1] == '/':
				j = len(text)
			case c == '{':
				depth++
				opened = true
			case c == '}':
				depth--
			}
		}
		if opened && depth <= 0 {
			return i + 1, utf8.RuneCountInString(strings.TrimRight(text, "\r")) + 1
		}
		if !opened && i > start && i-start > 3 {
			break
		}
	}
	return start, lineLen(f, start)
}

// valueEnd bir değer bildiriminin (sabit, değişken, alan) son satırıdır: başladığı satırdan, açılan
// ([{ parantezlerin hepsi kapanana kadar (metin ve yorumların içindekiler sayılmaz); parantezi
// olmayan bildirim tek satırdır.
func valueEnd(f *file, start int) (line, endCol int) {
	depth := 0
	for i := start - 1; i < len(f.lines); i++ {
		text := f.lines[i]
		inStr := false
		for j := 0; j < len(text); j++ {
			c := text[j]
			switch {
			case inStr:
				if c == '\\' {
					j++
				} else if c == '"' {
					inStr = false
				}
			case c == '"':
				inStr = true
			case c == '/' && j+1 < len(text) && text[j+1] == '/':
				j = len(text)
			case c == '(' || c == '[' || c == '{':
				depth++
			case c == ')' || c == ']' || c == '}':
				depth--
			}
		}
		if depth <= 0 {
			return i + 1, lineLen(f, i+1)
		}
	}
	return start, lineLen(f, start)
}

// lineLen bir satırın kod noktası uzunluğu + 1'dir (satır sonunun sütunu).
func lineLen(f *file, line int) int {
	if line < 1 || line > len(f.lines) {
		return 0
	}
	return utf8.RuneCountInString(strings.TrimRight(f.lines[line-1], "\r")) + 1
}

// importPos `aktar "yol"` ifadesindeki yol dizgisinin konumudur.
func (f *file) importPos(s *ast.ImportStatement) Pos {
	pos := Pos{File: f.rel, Line: s.Token.Line, Col: s.Token.Column}
	if s.Token.Line >= 1 && s.Token.Line <= len(f.lines) {
		text := f.lines[s.Token.Line-1]
		if i := strings.Index(text, `"`+s.Path); i >= 0 {
			pos.Col = 1 + utf8.RuneCountInString(text[:i])
			pos.EndLine = s.Token.Line
			pos.EndCol = pos.Col + utf8.RuneCountInString(s.Path) + 2
		}
	}
	return pos
}

// importTarget bir `aktar` yolunu dosyaya çevirir: "tr/ad" → lib/ad.tr, öbürleri içe aktaran
// dosyanın klasörüne göre (".tr" uzantısı isteğe bağlıdır). Bulunamazsa "".
func (x *indexer) importTarget(f *file, p string) string {
	cands := []string{path.Join(path.Dir(f.rel), p)}
	if rest, ok := strings.CutPrefix(p, "tr/"); ok {
		cands = append([]string{path.Join("lib", rest)}, cands...)
		cands = append(cands, libraryPrefix+path.Join("lib", rest)) // gömülü kitaplık, en son
	}
	for _, c := range cands {
		for _, name := range []string{c, c + ".tr"} {
			if strings.HasSuffix(name, ".tr") && x.nodes[NodeID(KindModule, name, "")] != nil {
				return name
			}
		}
	}
	return ""
}

// resolveImports bir dosyada görünen adları kurar: kendi tanımları, takma adsız aktarılan
// modüllerin (dolaylı olanlar dahil) tanımları ve `olarak` ile verilen takma adlar.
func (x *indexer) resolveImports(f *file) {
	f.visible = map[string]string{}
	for name, id := range f.defs {
		f.visible[name] = id
	}
	seen := map[string]bool{f.rel: true}
	var merge func(g *file)
	merge = func(g *file) {
		for _, imp := range g.imports {
			target := x.files[imp.file]
			if target == nil {
				continue
			}
			// Takma adlı aktarma, aynı dosya takma adsız da aktarılmış olsa tanınır.
			if imp.alias != "" {
				if g == f {
					f.aliases[imp.alias] = target
				}
				continue
			}
			if seen[imp.file] {
				continue
			}
			seen[imp.file] = true
			for name, id := range target.defs {
				if _, taken := f.visible[name]; !taken {
					f.visible[name] = id
				}
			}
			merge(target)
		}
	}
	merge(f)
	mod := NodeID(KindModule, f.rel, "")
	for _, imp := range f.imports {
		if imp.file == "" {
			continue
		}
		target := NodeID(KindModule, imp.file, "")
		x.addEdge(mod, target, EdgeImports, Extracted, imp.pos)
		x.occurrence(target, RoleImport, Extracted, imp.pos, mod)
	}
}

// namePos bildirimin adının konumudur (anahtar sözcük değil): satırda anahtar sözcükten sonra ilk
// geçen ad. Alan ve metotlarda ad "Yapı.ad" içindeki sondur. Testte ad yoktur, anahtar sözcük
// konumu alınır.
func namePos(f *file, kind Kind, name string, line, col int) Pos {
	short := name
	if kind == KindMethod || kind == KindField {
		short = name[strings.LastIndex(name, ".")+1:]
	}
	p := Pos{File: f.rel, Line: line, Col: col}
	if kind == KindTest || line < 1 || line > len(f.lines) {
		return p
	}
	text := f.lines[line-1]
	skip := 0
	if col > 1 {
		skip = len(string([]rune(text)[:min(col-1, len([]rune(text)))]))
	}
	if i := strings.Index(text[skip:], short); i >= 0 {
		p.Col = 1 + utf8.RuneCountInString(text[:skip+i])
		p.EndLine, p.EndCol = line, p.Col+utf8.RuneCountInString(short)
	}
	return p
}

// docAbove bildirimin hemen üstündeki yorum satırlarıdır ("// " önekleri atılır, bitişik satırlar
// bir araya gelir); boş satır ya da yorum olmayan satır blokta durur.
func docAbove(f *file, line int) string {
	var parts []string
	for i := line - 2; i >= 0 && i < len(f.lines); i-- {
		t := strings.TrimSpace(f.lines[i])
		if !strings.HasPrefix(t, "//") {
			break
		}
		parts = append([]string{strings.TrimSpace(strings.TrimPrefix(t, "//"))}, parts...)
	}
	return strings.Join(parts, "\n")
}
