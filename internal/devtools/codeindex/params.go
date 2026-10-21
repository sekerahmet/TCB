package codeindex

import (
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// paramNodes bir işlev ya da metodun parametrelerini düğüm yapar: kimlik
// `parameter:dosya#Sahip.ad` (metotta `Yapı.metot.ad`), konum başlıktaki adın yeridir. Parametre
// başvuruları kenar değil yalnızca başvuru olarak tutulur (işlevden kendi parametresine kenar anlamsızdır).
func (x *indexer) paramNodes(f *file, owner string, ownerID string, fl *ast.FunctionLiteral) {
	if len(fl.Parameters) == 0 {
		return
	}
	pos := paramPositions(f, fl)
	for i, p := range fl.Parameters {
		name := owner + "." + p.Name
		id := NodeID(KindParameter, f.rel, name)
		sig := p.Name
		if p.DefaultValue != nil {
			sig += " = …"
		}
		at := pos[i]
		x.nodes[id] = &Node{ID: id, Kind: KindParameter, Name: name, Pos: &at, Signature: sig, Qualified: f.base + "." + name}
		x.addEdge(ownerID, id, EdgeDefines, Extracted, Pos{File: f.rel, Line: at.Line})
		x.occurrence(id, RoleDefinition, Extracted, at, ownerID)
	}
}

// paramPositions parametre adlarının başlıktaki konumlarını bulur: işlevin anahtar sözcüğünden sonraki
// ilk `(`ten, eşleşen `)`e kadar; ilk derinlikteki her virgülden sonra gelen ilk ad bir parametredir
// (metin, yorum ve iç içe parantezler atlanır, başlık birden çok satıra yayılabilir). Bulunamayanın
// konumu başlık satırıdır (sütunsuz).
func paramPositions(f *file, fl *ast.FunctionLiteral) []Pos {
	out := make([]Pos, len(fl.Parameters))
	for i := range out {
		out[i] = Pos{File: f.rel, Line: fl.Token.Line}
	}
	depth, expect, idx := 0, false, 0
	for li := fl.Token.Line - 1; li < len(f.lines) && li < fl.Token.Line+30; li++ {
		runes := []rune(strings.TrimRight(f.lines[li], "\r"))
		start := 0
		if li == fl.Token.Line-1 && fl.Token.Column > 1 {
			start = min(fl.Token.Column-1, len(runes))
		}
		inStr := false
		for j := start; j < len(runes); j++ {
			c := runes[j]
			switch {
			case inStr:
				if c == '\\' {
					j++
				} else if c == '"' {
					inStr = false
				}
			case c == '"':
				inStr = true
			case c == '/' && j+1 < len(runes) && runes[j+1] == '/':
				j = len(runes)
			case c == '(' || c == '[' || c == '{':
				depth++
				if depth == 1 && c == '(' {
					expect = true
				}
			case c == ')' || c == ']' || c == '}':
				depth--
				if depth <= 0 {
					return out
				}
			case c == ',' && depth == 1:
				expect = true
			case expect && depth == 1 && (unicode.IsLetter(c) || c == '_'):
				k := j
				for k < len(runes) && (unicode.IsLetter(runes[k]) || unicode.IsDigit(runes[k]) || runes[k] == '_') {
					k++
				}
				if idx < len(out) && string(runes[j:k]) == fl.Parameters[idx].Name {
					out[idx] = Pos{File: f.rel, Line: li + 1, Col: j + 1, EndLine: li + 1, EndCol: k + 1}
				}
				idx++
				expect = false
				j = k - 1
			}
		}
	}
	return out
}
