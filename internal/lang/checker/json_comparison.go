package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// JSON'un karşılaştırması (3.7a "Karşılaştırmak", 4.5): == ve != iki TR_JSON değerini karşılaştırır.
// JSON değeriyle tipi bilinen, JSON olmayan bir değerin karşılaştırması sonucu hep aynı olan bir
// denetim hatasıdır (H1131); TR_JSON isteğe bağlı değildir, TR_none'la karşılaştırması da (H1132).
// Kural kabın içinde (liste, sözlük, demet), .TR_contains'te ve eşleme deseninde de geçer.
// TR_Type(j) == TR_JSON hep yanlıştır: TR_Type JSON'un türünü verir (H1134). Öneriler programın
// kendi yazımıyla kurulur (exprText); yazım bilinmiyorsa öneri yoktur.

// jsonPair JSON olmayan değerle karşılaştırılan bir JSON değeridir: json JSON yanın ifadesi (kabın
// öğe tipinden geliyorsa nil), other öbür yanın ifadesi ve tipi; none öbür yanın TR_none olduğudur.
type jsonPair struct {
	json, other ast.Expression
	otherType   string
	none        bool
	// optional JSON yanın TR_JSON? olduğudur (j.TR_get(k)): açma önerisi verilmez.
	optional bool
}

// typedItem tipi bilinen bir değer ya da kabın öğesidir; expr değişmezin öğesiyse ifadesidir.
type typedItem struct {
	typ  string
	expr ast.Expression
}

// jsonPairs lt/le ile rt/re'nin karşılaştırmasında JSON olmayanla karşılaşan JSON değerlerini
// toplar; kabın içine öğe öğe iner.
func (c *Checker) jsonPairs(l, r typedItem, out *[]jsonPair) {
	if optionalJSON := optionalType(typeJSON); l.typ == optionalJSON || r.typ == optionalJSON {
		if r.typ == optionalJSON {
			l, r = r, l
		}
		if r.typ != typeNull && requiredType(r.typ) != typeJSON && c.knownType(r.typ) {
			*out = append(*out, jsonPair{json: l.expr, other: r.expr, otherType: r.typ, optional: true})
		}
		return
	}
	if l.typ != typeJSON && r.typ == typeJSON {
		l, r = r, l
	}
	if l.typ == typeJSON {
		switch {
		case r.typ == typeNull:
			*out = append(*out, jsonPair{json: l.expr, other: r.expr, otherType: r.typ, none: true})
		case requiredType(r.typ) != typeJSON && c.knownType(r.typ):
			*out = append(*out, jsonPair{json: l.expr, other: r.expr, otherType: r.typ})
		}
		return
	}
	ls, rs := c.items(l), c.items(r)
	if ls == nil || rs == nil || containerKind(l) != containerKind(r) {
		return
	}
	if _, lit := l.expr.(*ast.ListLiteral); lit || containerKind(l) == "tuple" {
		if _, lit2 := r.expr.(*ast.ListLiteral); (lit2 || containerKind(r) == "tuple") && len(ls) == len(rs) && len(ls) > 1 {
			for i := range ls {
				c.jsonPairs(ls[i], rs[i], out)
			}
			return
		}
	}
	for _, a := range ls {
		for _, b := range rs {
			c.jsonPairs(a, b, out)
		}
	}
}

// containerKind karşılaştırmada içine inilen kabın türüdür: "list", "dict", "tuple" ya da "".
func containerKind(v typedItem) string {
	switch v.expr.(type) {
	case *ast.ListLiteral:
		return "list"
	case *ast.DictLiteral:
		return "dict"
	case *ast.TupleExpression:
		return "tuple"
	}
	s := shapeOf(v.typ)
	switch {
	case s.optional:
		return ""
	case s.elems != nil:
		return "tuple"
	case normalizeTypeName(s.base) == typeList:
		return "list"
	case normalizeTypeName(s.base) == typeDict:
		return "dict"
	}
	return ""
}

// items kabın karşılaştırılan öğeleridir: değişmezde öğeleri (sözlükte değerleri), tipte öğe tipi.
func (c *Checker) items(v typedItem) []typedItem {
	var out []typedItem
	switch x := v.expr.(type) {
	case *ast.ListLiteral:
		for _, el := range x.Elements {
			out = append(out, typedItem{c.itemTypes[el], el})
		}
		return out
	case *ast.DictLiteral:
		for _, p := range x.Pairs {
			out = append(out, typedItem{c.itemTypes[p.Value], p.Value})
		}
		return out
	case *ast.TupleExpression:
		for _, el := range x.Elements {
			out = append(out, typedItem{c.itemTypes[el], el})
		}
		return out
	}
	s := shapeOf(v.typ)
	switch {
	case s.optional:
	case s.elems != nil:
		for _, t := range s.elems {
			out = append(out, typedItem{t, nil})
		}
	case normalizeTypeName(s.base) == typeList && len(s.args) == 1:
		out = append(out, typedItem{s.args[0], nil})
	case normalizeTypeName(s.base) == typeDict && len(s.args) == 2:
		out = append(out, typedItem{s.args[1], nil})
	}
	return out
}

// checkJSONComparison == ve != için JSON'un kurallarını uygular.
func (c *Checker) checkJSONComparison(e *ast.InfixExpression, left, right string) {
	if e.Operator != "==" && e.Operator != "!=" {
		return
	}
	if c.typeOfJSON(e) {
		return
	}
	var pairs []jsonPair
	c.jsonPairs(typedItem{left, e.Left}, typedItem{right, e.Right}, &pairs)
	if len(pairs) == 0 {
		return
	}
	inner := requiredType(left) != typeJSON && requiredType(right) != typeJSON
	c.reportJSONPairs(e, e, pairs, inner)
}

// reportJSONPairs bir yerin (at) JSON karşılaştırma hatasını yazar: whole önerinin içinde yazıldığı
// ifadedir (karşılaştırma, .TR_contains çağrısı, eşlemenin konusu).
func (c *Checker) reportJSONPairs(at ast.Node, whole ast.Expression, pairs []jsonPair, inner bool) {
	kind := "top"
	if inner {
		kind = "inner"
	}
	p := pairs[0]
	if p.none {
		absent, missing := c.noneSpellings(whole, p)
		c.addErrorAt(at, messages.New(messages.H1132, kind, absent, missing))
		return
	}
	var wraps []spliced
	for _, q := range pairs {
		if q.other != nil && !q.none {
			wraps = append(wraps, spliced{q.other, codes.Spelling(codes.TypeJSON) + "(", ")", ""})
		}
	}
	wrap := c.splice(whole, wraps)
	open := ""
	if t, ok := scalarCode(p.otherType); ok && p.json != nil && !p.optional {
		open = c.splice(whole, []spliced{{p.json, codes.Spelling(t) + "(", ")", ""}})
	}
	c.addErrorAt(at, messages.New(messages.H1131, c.typeArg(p.otherType), kind, open, wrap))
}

// noneSpellings H1132'nin iki önerisidir: JSON yokluğu için TR_Type(j) == TR_NoneType, eksik alan
// için j.TR_get(k) == TR_none (JSON yanı bir indeksse). JSON yanı kabın öğe tipiyse ve öbür yan bir
// ifadeyse JSON yokluğu TR_JSON(TR_none)'dır.
func (c *Checker) noneSpellings(whole ast.Expression, p jsonPair) (absent, missing string) {
	none := []spliced{}
	if p.other != nil {
		none = append(none, spliced{p.other, "", "", codes.Spelling(codes.TypeNoneType)})
	}
	if p.json == nil {
		if p.other != nil {
			absent = c.splice(whole, []spliced{{p.other, codes.Spelling(codes.TypeJSON) + "(", ")", ""}})
		}
		return absent, ""
	}
	absent = c.splice(whole, append(none, spliced{p.json, codes.Spelling(codes.TypeType) + "(", ")", ""}))
	if ix, ok := p.json.(*ast.IndexExpression); ok {
		obj, ok1 := c.exprText(ix.Left)
		key, ok2 := c.exprText(ix.Index)
		if ok1 && ok2 {
			missing = c.splice(whole, []spliced{{p.json, "", "", obj + "." + codes.Spelling(codes.MemGet) + "(" + key + ")"}})
		}
	}
	return absent, missing
}

// scalarCode açma yazımı (TR_Int(j)) olan tipin kodudur: sayı, metin, mantık değeri.
func scalarCode(t string) (codes.Code, bool) {
	switch t {
	case typeInt:
		return codes.TypeInt, true
	case typeFloat:
		return codes.TypeFloat, true
	case typeString:
		return codes.TypeString, true
	case typeBool:
		return codes.TypeBool, true
	}
	return codes.None, false
}

// typeOfJSON TR_Type(j) == TR_JSON'u (ya da !=) bulur ve H1134'ü yazar: TR_Type JSON değerinin
// türünü verir, TR_JSON'u hiç vermez; sonuç hep aynıdır. Öneri türlerden birini sınar (TR_Dict).
func (c *Checker) typeOfJSON(e *ast.InfixExpression) bool {
	call, name := e.Left, e.Right
	if !c.jsonTypeName(name) {
		call, name = name, call
	}
	ce, ok := call.(*ast.CallExpression)
	if !ok || !c.jsonTypeName(name) || len(ce.Arguments) != 1 || c.builtinCallCode(ce, typeAny) != codes.TypeType || c.itemTypes[ce.Arguments[0]] != typeJSON {
		return false
	}
	suggestion := c.splice(e, []spliced{{name, "", "", codes.Spelling(codes.TypeDict)}})
	c.addErrorAt(e, messages.New(messages.H1134, suggestion))
	return true
}

// jsonTypeName ifade TR_JSON tip adının kendisi mi (programın bir adı değil)?
func (c *Checker) jsonTypeName(e ast.Expression) bool {
	id, ok := e.(*ast.Identifier)
	if !ok || id.Token.Code != codes.TypeJSON {
		return false
	}
	_, user := c.scope.Resolve(id.Value)
	return !user
}

// spliced önerideki bir değişikliktir: sub ifadesinin yazımı before + yazım + after olur; with boş
// değilse yazımın yerine with gelir.
type spliced struct {
	sub                 ast.Expression
	before, after, with string
}

// splice whole'un dosyadaki yazımında her değişikliği uygular; bir yazım bilinmiyor ya da whole'un
// satırında değilse "" verir.
func (c *Checker) splice(whole ast.Expression, edits []spliced) string {
	text, ok := c.exprText(whole)
	if !ok || len(edits) == 0 {
		return ""
	}
	base := ast.RangeOf(whole)
	runes := []rune(text)
	type cut struct {
		from, to int
		repl     string
	}
	var cuts []cut
	for _, ed := range edits {
		sub, ok := c.exprText(ed.sub)
		r := ast.RangeOf(ed.sub)
		if !ok || r.Line != base.Line || r.Col < base.Col {
			return ""
		}
		from := r.Col - base.Col
		to := from + len([]rune(sub))
		if to > len(runes) {
			return ""
		}
		repl := ed.before + sub + ed.after
		if ed.with != "" {
			repl = ed.with
		}
		cuts = append(cuts, cut{from, to, repl})
	}
	out := ""
	at := 0
	for len(cuts) > 0 {
		k := 0
		for i := range cuts {
			if cuts[i].from < cuts[k].from {
				k = i
			}
		}
		cu := cuts[k]
		cuts = append(cuts[:k], cuts[k+1:]...)
		if cu.from < at {
			return ""
		}
		out += string(runes[at:cu.from]) + cu.repl
		at = cu.to
	}
	return out + string(runes[at:])
}

// checkJSONContains .TR_contains(x)'i JSON'un kuralıyla denetler: listenin öğesiyle x'in
// karşılaştırmasıdır (3.7a).
func (c *Checker) checkJSONContains(e *ast.CallExpression, objType string, argTypes []string) {
	m, ok := e.Function.(*ast.MemberExpression)
	if !ok || m.Safe || m.NameToken.Code != codes.MemContains || len(e.Arguments) != 1 {
		return
	}
	recv := typedItem{objType, m.Object}
	if containerKind(recv) != "list" {
		return
	}
	var pairs []jsonPair
	for _, it := range c.items(recv) {
		c.jsonPairs(it, typedItem{argTypes[0], e.Arguments[0]}, &pairs)
	}
	if len(pairs) > 0 {
		c.reportJSONPairs(e.Arguments[0], e, pairs, true)
	}
}

// checkJSONPatterns eşlemenin konusu TR_JSON ise desendeki JSON olmayan değeri denetler (3.7a): desen
// konuyla karşılaştırılır. Öneri konuyu tipiyle açar ya da deseni JSON'a çevirir.
func (c *Checker) checkJSONPatterns(e *ast.MatchExpression, subject string, pattern ast.Expression, patType string) {
	if subject != typeJSON {
		return
	}
	var pairs []jsonPair
	c.jsonPairs(typedItem{subject, e.Subject}, typedItem{patType, pattern}, &pairs)
	if len(pairs) == 0 {
		return
	}
	p := pairs[0]
	if p.none {
		absent := c.splice(pattern, []spliced{{pattern, codes.Spelling(codes.TypeJSON) + "(", ")", ""}})
		c.addErrorAt(pattern, messages.New(messages.H1132, "inner", absent, ""))
		return
	}
	wrap := c.splice(pattern, []spliced{{pattern, codes.Spelling(codes.TypeJSON) + "(", ")", ""}})
	open := ""
	if t, ok := scalarCode(p.otherType); ok {
		if s := c.splice(e.Subject, []spliced{{e.Subject, codes.Spelling(t) + "(", ")", ""}}); s != "" {
			open = codes.Spelling(codes.KwMatch) + " " + s
		}
	}
	c.addErrorAt(pattern, messages.New(messages.H1131, c.typeArg(p.otherType), "inner", open, wrap))
}
