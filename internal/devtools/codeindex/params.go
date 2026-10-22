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
		at := pos[i]
		if p.DefaultValue != nil {
			var next *Pos
			if i+1 < len(pos) {
				next = &pos[i+1]
			}
			sig += " = " + defaultText(f, at, next)
		}
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
		sc := newCodeScanner(runes, start, len(runes), true)
		for j, c, ok := sc.next(); ok; j, c, ok = sc.next() {
			switch {
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
				sc.i = k // adın üstünden atla
			}
		}
	}
	return out
}

// defaultText parametrenin varsayılan değerinin kaynak metnidir (`ad = değer` başlığındaki değer):
// adın bitişinden bir sonraki parametrenin başına ya da parantezin kapanışına kadar, aynı satırda.
// Birden çok satıra yayılan ya da bulunamayan değer için "…".
func defaultText(f *file, at Pos, next *Pos) string {
	if at.EndCol == 0 || at.Line < 1 || at.Line > len(f.lines) {
		return "…"
	}
	runes := []rune(strings.TrimRight(f.lines[at.Line-1], "\r"))
	from := at.EndCol - 1
	if from > len(runes) {
		return "…"
	}
	to := len(runes)
	if next != nil && next.Line == at.Line && next.Col > 0 {
		to = next.Col - 1
	}
	depth := 0
	sc := newCodeScanner(runes, from, to, false)
	for i, c, ok := sc.next(); ok; i, c, ok = sc.next() {
		switch {
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			if depth == 0 {
				to = i
				sc.end = i
			} else {
				depth--
			}
		}
	}
	text := strings.TrimSpace(string(runes[from:to]))
	text = strings.TrimSpace(strings.TrimSuffix(text, ","))
	text = strings.TrimSpace(strings.TrimPrefix(text, "="))
	if text == "" || to == len(runes) && depth != 0 {
		return "…"
	}
	return text
}
