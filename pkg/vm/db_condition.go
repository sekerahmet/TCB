package vm

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ============================================================================
// GÜVENLİ SATIR KOŞULU DEĞERLENDİRİCİSİ (NEREDE / WHERE)
//
// Koşullar artık TR kodu olarak ÇALIŞTIRILMAZ. Yalnızca şu küçük dil desteklenir:
//   sütun adları, sayılar, 'metin' / "metin" (enterpolasyon YOK), doğru/yanlış/yok,
//   ? (parametre), + - * / %, == = != <> < <= > >=, ve/AND/&&, veya/OR/||, değil/NOT/!, ( )
// Böylece koşul içinden fonksiyon çağrısı (dosya_yaz, web_gönder ...) yapılamaz.
// ============================================================================

type condTokKind int

const (
	ctEOF condTokKind = iota
	ctNum
	ctStr
	ctIdent
	ctParam
	ctOp
	ctLParen
	ctRParen
)

type condTok struct {
	kind condTokKind
	text string
	num  Value
	pos  int
}

func tokenizeCondition(s string) ([]condTok, error) {
	var toks []condTok
	rs := []rune(s)
	n := len(rs)
	i := 0
	for i < n {
		r := rs[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '(':
			toks = append(toks, condTok{kind: ctLParen, text: "(", pos: i})
			i++
		case r == ')':
			toks = append(toks, condTok{kind: ctRParen, text: ")", pos: i})
			i++
		case r == '?':
			toks = append(toks, condTok{kind: ctParam, text: "?", pos: i})
			i++
		case r == '\'' || r == '"':
			quote := r
			start := i
			i++
			var sb strings.Builder
			closed := false
			for i < n {
				c := rs[i]
				if c == '\\' && i+1 < n {
					switch rs[i+1] {
					case 'n':
						sb.WriteRune('\n')
					case 't':
						sb.WriteRune('\t')
					default:
						sb.WriteRune(rs[i+1])
					}
					i += 2
					continue
				}
				if c == quote {
					// SQL tarzı çift tırnak kaçışı: 'O''Brien'
					if i+1 < n && rs[i+1] == quote {
						sb.WriteRune(quote)
						i += 2
						continue
					}
					closed = true
					i++
					break
				}
				sb.WriteRune(c)
				i++
			}
			if !closed {
				return nil, fmt.Errorf("kapatılmamış metin (konum %d)", start+1)
			}
			toks = append(toks, condTok{kind: ctStr, text: sb.String(), pos: start})
		case unicode.IsDigit(r):
			start := i
			isFloat := false
			for i < n && (unicode.IsDigit(rs[i]) || rs[i] == '_' || (rs[i] == '.' && !isFloat && i+1 < n && unicode.IsDigit(rs[i+1]))) {
				if rs[i] == '.' {
					isFloat = true
				}
				i++
			}
			lit := strings.ReplaceAll(string(rs[start:i]), "_", "")
			var num Value
			if isFloat {
				f, err := strconv.ParseFloat(lit, 64)
				if err != nil {
					return nil, fmt.Errorf("geçersiz sayı %q", lit)
				}
				num = &FloatVal{Val: f}
			} else {
				iv, err := strconv.ParseInt(lit, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("geçersiz sayı %q", lit)
				}
				num = &IntVal{Val: iv}
			}
			toks = append(toks, condTok{kind: ctNum, text: lit, num: num, pos: start})
		case unicode.IsLetter(r) || r == '_':
			start := i
			for i < n && (unicode.IsLetter(rs[i]) || unicode.IsDigit(rs[i]) || rs[i] == '_' || unicode.Is(unicode.Mn, rs[i])) {
				i++
			}
			toks = append(toks, condTok{kind: ctIdent, text: string(rs[start:i]), pos: start})
		default:
			start := i
			two := ""
			if i+1 < n {
				two = string(rs[i : i+2])
			}
			switch two {
			case "==", "!=", "<>", "<=", ">=", "&&", "||":
				toks = append(toks, condTok{kind: ctOp, text: two, pos: start})
				i += 2
				continue
			}
			switch r {
			case '=', '<', '>', '!', '+', '-', '*', '/', '%':
				toks = append(toks, condTok{kind: ctOp, text: string(r), pos: start})
				i++
			default:
				return nil, fmt.Errorf("koşulda beklenmeyen karakter %q (konum %d)", string(r), start+1)
			}
		}
	}
	toks = append(toks, condTok{kind: ctEOF, pos: n})
	return toks, nil
}

// rowCondition ayrıştırılmış ve yeniden kullanılabilir bir satır koşuludur.
type rowCondition struct {
	root condNode
}

type condNode interface {
	eval(row map[string]interface{}) (Value, error)
}

type condLiteral struct{ val Value }

type condColumn struct{ name string }

type condUnary struct {
	op string
	x  condNode
}

type condBinary struct {
	op   string
	l, r condNode
}

type condParser struct {
	toks   []condTok
	pos    int
	params []Value
	pIdx   int
	known  map[string]bool
}

// compileRowCondition koşul metnini güvenli ifade ağacına dönüştürür.
// known: geçerli sütun adları (boşsa her ad kabul edilir, olmayan sütun 'yok' olur).
// params: '?' yer tutucularına sırayla bağlanacak değerler.
func compileRowCondition(cond string, known map[string]bool, params []Value) (*rowCondition, error) {
	toks, err := tokenizeCondition(cond)
	if err != nil {
		return nil, fmt.Errorf("Koşul hatası (%s): %v", cond, err)
	}
	p := &condParser{toks: toks, params: params, known: known}
	root, err := p.parseOr()
	if err == nil && p.peek().kind != ctEOF {
		err = fmt.Errorf("beklenmeyen %q", p.peek().text)
	}
	if err == nil && p.pIdx != len(params) {
		err = fmt.Errorf("%d parametre verildi ama koşulda %d adet '?' var", len(params), p.pIdx)
	}
	if err != nil {
		return nil, fmt.Errorf("Koşul hatası (%s): %v", cond, err)
	}
	return &rowCondition{root: root}, nil
}

func (c *rowCondition) matches(row map[string]interface{}) (bool, error) {
	res, err := c.root.eval(row)
	if err != nil {
		return false, err
	}
	return res.IsTruthy(), nil
}

func (p *condParser) peek() condTok { return p.toks[p.pos] }
func (p *condParser) next() condTok {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func condWord(t condTok) string {
	if t.kind != ctIdent {
		return ""
	}
	return foldUpperASCII(t.text)
}

func (p *condParser) isOr(t condTok) bool {
	return (t.kind == ctOp && t.text == "||") || condWord(t) == "VEYA" || condWord(t) == "OR"
}
func (p *condParser) isAnd(t condTok) bool {
	return (t.kind == ctOp && t.text == "&&") || condWord(t) == "VE" || condWord(t) == "AND"
}
func (p *condParser) isNot(t condTok) bool {
	return (t.kind == ctOp && t.text == "!") || condWord(t) == "DEGIL" || condWord(t) == "NOT"
}

func (p *condParser) parseOr() (condNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.isOr(p.peek()) {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &condBinary{op: "veya", l: left, r: right}
	}
	return left, nil
}

func (p *condParser) parseAnd() (condNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.isAnd(p.peek()) {
		p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &condBinary{op: "ve", l: left, r: right}
	}
	return left, nil
}

func (p *condParser) parseNot() (condNode, error) {
	if p.isNot(p.peek()) {
		p.next()
		x, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &condUnary{op: "değil", x: x}, nil
	}
	return p.parseCompare()
}

func (p *condParser) parseCompare() (condNode, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	t := p.peek()
	if t.kind == ctOp {
		op := t.text
		switch op {
		case "=":
			op = "=="
		case "<>":
			op = "!="
		}
		switch op {
		case "==", "!=", "<", "<=", ">", ">=":
			p.next()
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			return &condBinary{op: op, l: left, r: right}, nil
		}
	}
	return left, nil
}

func (p *condParser) parseAdd() (condNode, error) {
	left, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == ctOp && (p.peek().text == "+" || p.peek().text == "-") {
		op := p.next().text
		right, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		left = &condBinary{op: op, l: left, r: right}
	}
	return left, nil
}

func (p *condParser) parseMul() (condNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == ctOp && (p.peek().text == "*" || p.peek().text == "/" || p.peek().text == "%") {
		op := p.next().text
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &condBinary{op: op, l: left, r: right}
	}
	return left, nil
}

func (p *condParser) parseUnary() (condNode, error) {
	if p.peek().kind == ctOp && p.peek().text == "-" {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &condUnary{op: "-", x: x}, nil
	}
	return p.parsePrimary()
}

func (p *condParser) parsePrimary() (condNode, error) {
	t := p.next()
	switch t.kind {
	case ctNum:
		return &condLiteral{val: t.num}, nil
	case ctStr:
		return &condLiteral{val: &StringVal{Val: t.text}}, nil
	case ctParam:
		if p.pIdx >= len(p.params) {
			return nil, fmt.Errorf("'?' için parametre verilmedi")
		}
		v := p.params[p.pIdx]
		p.pIdx++
		return &condLiteral{val: goToTRValue(trToGoValue(v))}, nil
	case ctLParen:
		x, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != ctRParen {
			return nil, fmt.Errorf("')' bekleniyordu")
		}
		p.next()
		return x, nil
	case ctIdent:
		switch condWord(t) {
		case "DOGRU", "TRUE":
			return &condLiteral{val: TRUE}, nil
		case "YANLIS", "FALSE":
			return &condLiteral{val: FALSE}, nil
		case "YOK", "NULL":
			return &condLiteral{val: NULL}, nil
		}
		if p.peek().kind == ctLParen {
			return nil, fmt.Errorf("koşul içinde fonksiyon çağrısı yapılamaz (%s)", t.text)
		}
		if len(p.known) > 0 && !p.known[t.text] {
			return nil, fmt.Errorf("bilinmeyen sütun %q", t.text)
		}
		return &condColumn{name: t.text}, nil
	case ctEOF:
		return nil, fmt.Errorf("koşul beklenmedik şekilde bitti")
	default:
		return nil, fmt.Errorf("beklenmeyen %q", t.text)
	}
}

func (n *condLiteral) eval(row map[string]interface{}) (Value, error) { return n.val, nil }

func (n *condColumn) eval(row map[string]interface{}) (Value, error) {
	if raw, ok := row[n.name]; ok {
		return goToTRValue(raw), nil
	}
	return NULL, nil
}

func (n *condUnary) eval(row map[string]interface{}) (Value, error) {
	x, err := n.x.eval(row)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "değil":
		return boolToVal(!x.IsTruthy()), nil
	case "-":
		switch v := x.(type) {
		case *IntVal:
			return &IntVal{Val: -v.Val}, nil
		case *FloatVal:
			return &FloatVal{Val: -v.Val}, nil
		}
		return nil, fmt.Errorf("'-' işleci %s tipine uygulanamaz", x.Type())
	}
	return nil, fmt.Errorf("bilinmeyen işleç %q", n.op)
}

func (n *condBinary) eval(row map[string]interface{}) (Value, error) {
	l, err := n.l.eval(row)
	if err != nil {
		return nil, err
	}
	// Kısa devre
	switch n.op {
	case "ve":
		if !l.IsTruthy() {
			return FALSE, nil
		}
		r, err := n.r.eval(row)
		if err != nil {
			return nil, err
		}
		return boolToVal(r.IsTruthy()), nil
	case "veya":
		if l.IsTruthy() {
			return TRUE, nil
		}
		r, err := n.r.eval(row)
		if err != nil {
			return nil, err
		}
		return boolToVal(r.IsTruthy()), nil
	}

	r, err := n.r.eval(row)
	if err != nil {
		return nil, err
	}

	switch n.op {
	case "==":
		return boolToVal(valuesEqual(l, r)), nil
	case "!=":
		return boolToVal(!valuesEqual(l, r)), nil
	case "<", "<=", ">", ">=":
		// Boş (yok) değerli sütunlar sıralama karşılaştırmasında eşleşmez
		if l.Type() == VAL_NULL || r.Type() == VAL_NULL {
			return FALSE, nil
		}
		var c int
		if lf, ok1 := toFloat(l); ok1 {
			rf, ok2 := toFloat(r)
			if !ok2 {
				return nil, fmt.Errorf("%s ile %s karşılaştırılamaz", l.Type(), r.Type())
			}
			c = compareFloat(lf, rf)
		} else if ls, ok1 := l.(*StringVal); ok1 {
			rs, ok2 := r.(*StringVal)
			if !ok2 {
				return nil, fmt.Errorf("%s ile %s karşılaştırılamaz", l.Type(), r.Type())
			}
			c = strings.Compare(ls.Val, rs.Val)
		} else {
			return nil, fmt.Errorf("%s tipi sıralama ile karşılaştırılamaz", l.Type())
		}
		switch n.op {
		case "<":
			return boolToVal(c < 0), nil
		case "<=":
			return boolToVal(c <= 0), nil
		case ">":
			return boolToVal(c > 0), nil
		default:
			return boolToVal(c >= 0), nil
		}
	case "+", "-", "*", "/", "%":
		if n.op == "+" {
			if ls, ok := l.(*StringVal); ok {
				if rs, ok := r.(*StringVal); ok {
					return &StringVal{Val: ls.Val + rs.Val}, nil
				}
			}
		}
		li, lInt := l.(*IntVal)
		ri, rInt := r.(*IntVal)
		if lInt && rInt {
			switch n.op {
			case "+":
				return &IntVal{Val: li.Val + ri.Val}, nil
			case "-":
				return &IntVal{Val: li.Val - ri.Val}, nil
			case "*":
				return &IntVal{Val: li.Val * ri.Val}, nil
			case "%":
				if ri.Val == 0 {
					return nil, fmt.Errorf("sıfıra bölme hatası")
				}
				return &IntVal{Val: li.Val % ri.Val}, nil
			}
		}
		lf, ok1 := toFloat(l)
		rf, ok2 := toFloat(r)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("'%s' işleci %s ve %s tiplerine uygulanamaz", n.op, l.Type(), r.Type())
		}
		switch n.op {
		case "+":
			return &FloatVal{Val: lf + rf}, nil
		case "-":
			return &FloatVal{Val: lf - rf}, nil
		case "*":
			return &FloatVal{Val: lf * rf}, nil
		case "/":
			if rf == 0 {
				return nil, fmt.Errorf("sıfıra bölme hatası")
			}
			return &FloatVal{Val: lf / rf}, nil
		default:
			return nil, fmt.Errorf("'%%' işleci yalnızca tam sayılarla kullanılabilir")
		}
	}
	return nil, fmt.Errorf("bilinmeyen işleç %q", n.op)
}

func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// tableColumnSet tablonun bilinen sütunlarını (ve satırlarda görülen anahtarları) döndürür.
func tableColumnSet(t *dbTable) map[string]bool {
	set := map[string]bool{"id": true}
	for _, c := range t.Columns {
		set[c] = true
	}
	for _, r := range t.Rows {
		for k := range r {
			set[k] = true
		}
	}
	return set
}

// filterRows koşulu bir kez derler ve satırları eşleşen / eşleşmeyen olarak ayırır.
func filterRows(rows []map[string]interface{}, cond string, known map[string]bool, params []Value) (matched, rest []map[string]interface{}, err error) {
	if strings.TrimSpace(cond) == "" {
		if len(params) > 0 {
			return nil, nil, fmt.Errorf("Koşul hatası: parametre verildi ama koşul boş")
		}
		return rows, nil, nil
	}
	c, err := compileRowCondition(cond, known, params)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		ok, err := c.matches(row)
		if err != nil {
			return nil, nil, fmt.Errorf("Koşul hatası (%s): %v", cond, err)
		}
		if ok {
			matched = append(matched, row)
		} else {
			rest = append(rest, row)
		}
	}
	return matched, rest, nil
}

// paramsArg isteğe bağlı parametre listesini (Liste) []Value olarak döndürür.
func paramsArg(args []Value, idx int, named map[string]Value) ([]Value, error) {
	var raw Value
	if idx < len(args) {
		raw = args[idx]
	}
	for _, k := range []string{"parametreler", "params"} {
		if pv, ok := named[k]; ok {
			raw = pv
		}
	}
	if raw == nil || raw.Type() == VAL_NULL {
		return nil, nil
	}
	lv, ok := raw.(*ListVal)
	if !ok {
		return nil, fmt.Errorf("parametreler bir Liste olmalıdır (örn: [\"Yapay Zeka\", 80000])")
	}
	return lv.Elements, nil
}
