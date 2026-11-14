package ast

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// SourceText is node n's spelling in the file (lines are the file's lines): its range (RangeOf),
// with the closing ')', ']' and '}' the tree does not keep taken from the rest of the line. ok is
// false when the line is not known or the node does not fit on one line.
func SourceText(lines []string, n any) (string, bool) {
	start, end, ok := sourceSpan(lines, n)
	if !ok {
		return "", false
	}
	return string([]rune(lines[RangeOf(n).Line-1])[start:end]), true
}

// sourceSpan n'nin satırındaki yeridir: rune olarak [start, end), kapanan ayraçlar dahil.
func sourceSpan(lines []string, n any) (start, end int, ok bool) {
	r := RangeOf(n)
	if r.Line < 1 || r.Line > len(lines) || r.EndLine != r.Line || r.Col < 1 {
		return 0, 0, false
	}
	line := []rune(lines[r.Line-1])
	if r.EndCol-1 > len(line) || r.Col >= r.EndCol {
		return 0, 0, false
	}
	closed, ok := closeParens(line[r.Col-1:r.EndCol-1], line[r.EndCol-1:])
	if !ok {
		return 0, 0, false
	}
	return r.Col - 1, r.EndCol - 1 + closed, true
}

// closeParens ifadenin ağaçtaki aralığına (kapanan ')', ']' ve '}' ağaçta tutulmaz) satırın devamından
// eksik kapanan parantezlerin sayısını verir; metin içindekiler sayılmaz.
func closeParens(text, rest []rune) (int, bool) {
	var open []rune // kapanmayı bekleyenler, en içteki sonda
	quote := rune(0)
	for _, r := range text {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '(':
			open = append(open, ')')
		case r == '[':
			open = append(open, ']')
		case r == '{':
			open = append(open, '}')
		case (r == ')' || r == ']' || r == '}') && len(open) > 0:
			open = open[:len(open)-1]
		}
	}
	n := 0
	for _, r := range rest {
		if len(open) == 0 {
			break
		}
		if r != open[len(open)-1] {
			return 0, false
		}
		n++
		open = open[:len(open)-1]
	}
	return n, len(open) == 0
}

// CopyWriteText bir kopyaya yazmanın iletisinin metinleridir (15.6d; H6740, H6741; kapta 5.4, H2740).
type CopyWriteText struct {
	Step, Base, Index string // kopya adımı; tensörle indekste solu ve indeksi (ayraçlı)
	Write             string // deyimin kopya adımı yerine Copy yazılmış hâli
	Copy, K, I        string // önerinin adları: kapsamdakilerle ve deyimdekilerle çakışmaz
}

// NewCopyWriteText deyimin metinlerini kurar: first ve last deyimin ilk ve son parçasıdır (atamada
// hedef ve değer, çağrıda çağrının kendisi), step kopya adımıdır. taken kapsamda tanımlı adlardır.
// ok false ise deyim tek satıra sığmıyor ya da dosyası bilinmiyor.
func NewCopyWriteText(lines []string, first, last Node, step WriteStep, taken func(string) bool) (CopyWriteText, bool) {
	s0, _, ok1 := sourceSpan(lines, first)
	_, e1, ok2 := sourceSpan(lines, last)
	a, b, ok3 := sourceSpan(lines, step.Expr)
	r := RangeOf(first)
	if !ok1 || !ok2 || !ok3 || RangeOf(last).Line != r.Line || RangeOf(step.Expr).Line != r.Line {
		return CopyWriteText{}, false
	}
	line := []rune(lines[r.Line-1])
	t := CopyWriteText{Step: string(line[a:b])}
	if a > 0 && b < len(line) && line[a-1] == '(' && line[b] == ')' {
		a, b = a-1, b+1 // (t * 2.0)[0]: ayraç adımla gider
		s0 = min(s0, a)
	}
	used := wordsIn(lines) // dosyada sonra tanımlanan ad da çakışırdı
	fresh := func(base string) string {
		name := base
		for n := 2; used[name] || taken(name); n++ {
			name = base + strconv.Itoa(n)
		}
		used[name] = true
		return name
	}
	t.Copy, t.K, t.I = fresh("c"), fresh("k"), fresh("i")
	t.Write = string(line[s0:a]) + t.Copy + string(line[b:e1])
	if step.Kind == StepIndex {
		t.Base, _ = SourceText(lines, step.Left)
		t.Index, _ = SourceText(lines, step.Index)
		if !simpleOperand(step.Index) {
			t.Index = "(" + t.Index + ")"
		}
	}
	return t, true
}

// StatementWith deyimin (first'ten last'a) metnidir, part'ın yerine with yazılır (5.4'ün önerileri:
// H2741'de kabın indeksi, H2743'te kaynağın indeksi). ok false ise deyim tek satıra sığmıyor ya da
// part deyimin içinde değil.
func StatementWith(lines []string, first, last, part Node, with string) (string, bool) {
	s0, _, ok1 := sourceSpan(lines, first)
	_, e1, ok2 := sourceSpan(lines, last)
	a, b, ok3 := sourceSpan(lines, part)
	r := RangeOf(first)
	if !ok1 || !ok2 || !ok3 || RangeOf(last).Line != r.Line || RangeOf(part).Line != r.Line || a < s0 || b > e1 {
		return "", false
	}
	line := []rune(lines[r.Line-1])
	return string(line[s0:a]) + with + string(line[b:e1]), true
}

// SliceSourceIndex dilimin (base[a..b]) üstündeki i indeksinin kaynaktaki karşılığıdır (3.6, H2743):
// a + i, ikisi de sabitse toplamları. Aralığın başı hep yazılıdır (`l[..2]` sözdizimi hatası). i
// negatif olmamalıdır (çağıran bilir).
func SliceSourceIndex(lines []string, r *RangeExpression, i Expression) (string, bool) {
	it, ok := SourceText(lines, i)
	if !ok {
		return "", false
	}
	st, ok := SourceText(lines, r.Start)
	if !ok {
		return "", false
	}
	a, aLit := r.Start.(*IntegerLiteral)
	b, bLit := i.(*IntegerLiteral)
	if aLit && bLit && a.Value >= 0 && b.Value >= 0 && a.Value <= math.MaxInt64-b.Value {
		return strconv.FormatInt(a.Value+b.Value, 10), true
	}
	if !simpleOperand(r.Start) && !aLit {
		st = "(" + st + ")"
	}
	if !simpleOperand(i) && !bLit {
		it = "(" + it + ")"
	}
	return st + " + " + it, true
}

// FreshName önerinin yeni adıdır (3.5, H1182): base, dosyada geçen sözcüklerle ya da kapsamdaki
// adlarla (taken) çakışırsa base2, base3, ...
func FreshName(lines []string, base string, taken func(string) bool) string {
	used := wordsIn(lines)
	name := base
	for n := 2; used[name] || taken(name); n++ {
		name = base + strconv.Itoa(n)
	}
	return name
}

// wordsIn dosyada geçen sözcüklerdir (harf, rakam ve '_' dizileri): önerinin adları onlarla çakışmaz.
// Yalnız hata yolunda çalışır.
func wordsIn(lines []string) map[string]bool {
	out := map[string]bool{}
	for _, l := range lines {
		for _, w := range strings.FieldsFunc(l, func(r rune) bool { return r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			out[w] = true
		}
	}
	return out
}

// simpleOperand ifadenin arkasına bir üye (.TR_reshape) ayraçsız yazılabilir mi.
func simpleOperand(e Expression) bool {
	switch e.(type) {
	case *Identifier, *IndexExpression, *MemberExpression, *CallExpression, *ElementExpression:
		return true
	}
	return false
}
