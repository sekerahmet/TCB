package veritabani

import (
	"fmt"
	"strconv"
	"strings"
	"tr_dili/pkg/motor/internal/baglama"
	"tr_dili/pkg/vm"
	"unicode"
)

// ============================================================================
// GÜVENLİ SATIR KOŞULU DEĞERLENDİRİCİSİ (NEREDE)
//
// Koşullar artık TR kodu olarak ÇALIŞTIRILMAZ. Yalnızca şu küçük dil desteklenir:
//   sütun adları, sayılar, 'metin' / "metin" (enterpolasyon YOK), doğru/yanlış/yok,
//   ? (parametre), + - * / %, == = != <> < <= > >=, ve/&&, veya/||, değil/!, ( )
// Sözcükler yalnızca Türkçedir (bkz. sozcukler.go); AND, TRUE, DOGRU ... hata verir.
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
	num  vm.Value
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
			var num vm.Value
			if isFloat {
				f, err := strconv.ParseFloat(lit, 64)
				if err != nil {
					return nil, fmt.Errorf("geçersiz sayı %q", lit)
				}
				num = &vm.FloatVal{Val: f}
			} else {
				iv, err := strconv.ParseInt(lit, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("geçersiz sayı %q", lit)
				}
				num = &vm.IntVal{Val: iv}
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
	eval(row map[string]interface{}) (vm.Value, error)
}

type condLiteral struct{ val vm.Value }

type condColumn struct{ name string }

type condUnary struct {
	op string
	x  condNode
	d  int // ağaç derinliği
}

type condBinary struct {
	op   string
	l, r condNode
	d    int // ağaç derinliği
}

type condParser struct {
	toks   []condTok
	pos    int
	params []vm.Value
	pIdx   int
	known  map[string]bool
	// derinlik iç içe parantez ve önek işleç (değil, -) seviyesidir; ayrıştırıcının kendi
	// özyinelemesini sınırlar.
	derinlik int
}

// Koşul sınırları: dışarıdan gelen (ör. 10 MB'lık web gövdesindeki) bir koşul yığın
// taşmasıyla süreci çökertemesin diye iç içe seviye ve ifade ağacının derinliği sınırlıdır;
// aşılınca yakalanabilir bir hata verilir.
const (
	enFazlaKosulIcIce    = 200
	enFazlaKosulDerinlik = 10000
)

func kosulDerinlikHatasi() error {
	return fmt.Errorf("koşul çok derin iç içe (en fazla %d seviye parantez ya da önek işleç, %d terimlik zincir)", enFazlaKosulIcIce, enFazlaKosulDerinlik)
}

func (p *condParser) gir() error {
	p.derinlik++
	if p.derinlik > enFazlaKosulIcIce {
		return kosulDerinlikHatasi()
	}
	return nil
}

func (p *condParser) cik() { p.derinlik-- }

func dugumDerinligi(n condNode) int {
	switch x := n.(type) {
	case *condBinary:
		return x.d
	case *condUnary:
		return x.d
	}
	return 1
}

// ikili iki terimli bir düğüm kurar; ağaç derinliği sınırı aşılırsa hata verir (değerlendirme
// özyinelemesi de böylece sınırlı kalır).
func ikili(op string, l, r condNode) (condNode, error) {
	d := dugumDerinligi(l)
	if dr := dugumDerinligi(r); dr > d {
		d = dr
	}
	if d+1 > enFazlaKosulDerinlik {
		return nil, kosulDerinlikHatasi()
	}
	return &condBinary{op: op, l: l, r: r, d: d + 1}, nil
}

func tekli(op string, x condNode) condNode {
	return &condUnary{op: op, x: x, d: dugumDerinligi(x) + 1}
}

// kosulOzeti hata mesajlarında gösterilecek koşul metnidir; çok uzun koşullar kısaltılır.
func kosulOzeti(s string) string {
	r := []rune(s)
	if len(r) <= 120 {
		return s
	}
	return string(r[:120]) + "…"
}

// compileRowCondition koşul metnini güvenli ifade ağacına dönüştürür.
// known: geçerli sütun adları (boşsa her ad kabul edilir, olmayan sütun 'yok' olur).
// params: '?' yer tutucularına sırayla bağlanacak değerler.
func compileRowCondition(cond string, known map[string]bool, params []vm.Value) (*rowCondition, error) {
	toks, err := tokenizeCondition(cond)
	if err != nil {
		return nil, fmt.Errorf("Koşul hatası (%s): %v", kosulOzeti(cond), err)
	}
	p := &condParser{toks: toks, params: params, known: known}
	root, err := p.parseOr()
	if err == nil && p.peek().kind != ctEOF {
		if t, ok := sqlOneri(p.peek().text); ok && p.peek().kind == ctIdent {
			err = fmt.Errorf("%s", sqlIpucu(p.peek().text, t))
		} else {
			err = fmt.Errorf("beklenmeyen %q", p.peek().text)
		}
	}
	if err == nil && p.pIdx != len(params) {
		err = fmt.Errorf("%d parametre verildi ama koşulda %d adet '?' var", len(params), p.pIdx)
	}
	if err != nil {
		return nil, fmt.Errorf("Koşul hatası (%s): %v", kosulOzeti(cond), err)
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

// condWord ad sözcüğünün karşılaştırma biçimidir (Türkçe büyük harf; bkz. sozcukler.go).
func condWord(t condTok) string {
	if t.kind != ctIdent {
		return ""
	}
	return sqlBuyuk(t.text)
}

func (p *condParser) isOr(t condTok) bool {
	return (t.kind == ctOp && t.text == "||") || condWord(t) == kosulVeya
}
func (p *condParser) isAnd(t condTok) bool {
	return (t.kind == ctOp && t.text == "&&") || condWord(t) == kosulVe
}
func (p *condParser) isNot(t condTok) bool {
	return (t.kind == ctOp && t.text == "!") || condWord(t) == kosulDegil
}

func (p *condParser) parseOr() (condNode, error) {
	if err := p.gir(); err != nil {
		return nil, err
	}
	defer p.cik()
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
		if left, err = ikili("veya", left, right); err != nil {
			return nil, err
		}
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
		if left, err = ikili("ve", left, right); err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (p *condParser) parseNot() (condNode, error) {
	if p.isNot(p.peek()) {
		p.next()
		if err := p.gir(); err != nil {
			return nil, err
		}
		defer p.cik()
		x, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return tekli("değil", x), nil
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
			return ikili(op, left, right)
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
		if left, err = ikili(op, left, right); err != nil {
			return nil, err
		}
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
		if left, err = ikili(op, left, right); err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (p *condParser) parseUnary() (condNode, error) {
	if p.peek().kind == ctOp && p.peek().text == "-" {
		p.next()
		if err := p.gir(); err != nil {
			return nil, err
		}
		defer p.cik()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return tekli("-", x), nil
	}
	return p.parsePrimary()
}

func (p *condParser) parsePrimary() (condNode, error) {
	t := p.next()
	switch t.kind {
	case ctNum:
		return &condLiteral{val: t.num}, nil
	case ctStr:
		return &condLiteral{val: &vm.StringVal{Val: t.text}}, nil
	case ctParam:
		if p.pIdx >= len(p.params) {
			return nil, fmt.Errorf("'?' için parametre verilmedi")
		}
		v := p.params[p.pIdx]
		p.pIdx++
		// Parametre satır değerleriyle aynı biçime getirilir (ör. tensör → liste); Tam/Ondalık korunur
		g, err := bellekSatirDegeri("parametre", v)
		if err != nil {
			return nil, err
		}
		return &condLiteral{val: trDegeri(g)}, nil
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
		case kosulDogru:
			return &condLiteral{val: vm.TRUE}, nil
		case kosulYanlis:
			return &condLiteral{val: vm.FALSE}, nil
		case kosulYok:
			return &condLiteral{val: vm.NULL}, nil
		}
		if p.peek().kind == ctLParen {
			return nil, fmt.Errorf("koşul içinde fonksiyon çağrısı yapılamaz (%s)", t.text)
		}
		// İngilizce ya da ASCII yazılmış bir koşul sözcüğü (TRUE, NOT, DOGRU) sessizce sütun
		// adı sayılmaz; o adda bir sütun varsa sütundur.
		if k, ok := sqlOneri(t.text); ok && !p.known[t.text] {
			return nil, fmt.Errorf("%s", sqlIpucu(t.text, k))
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

func (n *condLiteral) eval(row map[string]interface{}) (vm.Value, error) { return n.val, nil }

func (n *condColumn) eval(row map[string]interface{}) (vm.Value, error) {
	if raw, ok := row[n.name]; ok {
		return trDegeri(raw), nil
	}
	return vm.NULL, nil
}

func (n *condUnary) eval(row map[string]interface{}) (vm.Value, error) {
	x, err := n.x.eval(row)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "değil":
		return vm.BoolToVal(!x.IsTruthy()), nil
	case "-":
		switch v := x.(type) {
		case *vm.IntVal:
			return &vm.IntVal{Val: -v.Val}, nil
		case *vm.FloatVal:
			return &vm.FloatVal{Val: -v.Val}, nil
		}
		return nil, fmt.Errorf("'-' işleci %s tipine uygulanamaz", x.Type())
	}
	return nil, fmt.Errorf("bilinmeyen işleç %q", n.op)
}

func (n *condBinary) eval(row map[string]interface{}) (vm.Value, error) {
	l, err := n.l.eval(row)
	if err != nil {
		return nil, err
	}
	// Kısa devre
	switch n.op {
	case "ve":
		if !l.IsTruthy() {
			return vm.FALSE, nil
		}
		r, err := n.r.eval(row)
		if err != nil {
			return nil, err
		}
		return vm.BoolToVal(r.IsTruthy()), nil
	case "veya":
		if l.IsTruthy() {
			return vm.TRUE, nil
		}
		r, err := n.r.eval(row)
		if err != nil {
			return nil, err
		}
		return vm.BoolToVal(r.IsTruthy()), nil
	}

	r, err := n.r.eval(row)
	if err != nil {
		return nil, err
	}

	switch n.op {
	case "==":
		return vm.BoolToVal(vm.ValuesEqual(l, r)), nil
	case "!=":
		return vm.BoolToVal(!vm.ValuesEqual(l, r)), nil
	case "<", "<=", ">", ">=":
		// Boş (yok) değerli sütunlar sıralama karşılaştırmasında eşleşmez
		if l.Type() == vm.VAL_NULL || r.Type() == vm.VAL_NULL {
			return vm.FALSE, nil
		}
		var c int
		if lf, ok1 := vm.ToFloat(l); ok1 {
			rf, ok2 := vm.ToFloat(r)
			if !ok2 {
				return nil, fmt.Errorf("%s ile %s karşılaştırılamaz", l.Type(), r.Type())
			}
			c = compareFloat(lf, rf)
		} else if ls, ok1 := l.(*vm.StringVal); ok1 {
			rs, ok2 := r.(*vm.StringVal)
			if !ok2 {
				return nil, fmt.Errorf("%s ile %s karşılaştırılamaz", l.Type(), r.Type())
			}
			c = strings.Compare(ls.Val, rs.Val)
		} else {
			return nil, fmt.Errorf("%s tipi sıralama ile karşılaştırılamaz", l.Type())
		}
		switch n.op {
		case "<":
			return vm.BoolToVal(c < 0), nil
		case "<=":
			return vm.BoolToVal(c <= 0), nil
		case ">":
			return vm.BoolToVal(c > 0), nil
		default:
			return vm.BoolToVal(c >= 0), nil
		}
	case "+", "-", "*", "/", "%":
		if n.op == "+" {
			if ls, ok := l.(*vm.StringVal); ok {
				if rs, ok := r.(*vm.StringVal); ok {
					return &vm.StringVal{Val: ls.Val + rs.Val}, nil
				}
			}
		}
		li, lInt := l.(*vm.IntVal)
		ri, rInt := r.(*vm.IntVal)
		if lInt && rInt {
			switch n.op {
			case "+":
				return &vm.IntVal{Val: li.Val + ri.Val}, nil
			case "-":
				return &vm.IntVal{Val: li.Val - ri.Val}, nil
			case "*":
				return &vm.IntVal{Val: li.Val * ri.Val}, nil
			case "%":
				if ri.Val == 0 {
					return nil, fmt.Errorf("sıfıra bölme hatası")
				}
				return &vm.IntVal{Val: li.Val % ri.Val}, nil
			}
		}
		lf, ok1 := vm.ToFloat(l)
		rf, ok2 := vm.ToFloat(r)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("'%s' işleci %s ve %s tiplerine uygulanamaz", n.op, l.Type(), r.Type())
		}
		switch n.op {
		case "+":
			return &vm.FloatVal{Val: lf + rf}, nil
		case "-":
			return &vm.FloatVal{Val: lf - rf}, nil
		case "*":
			return &vm.FloatVal{Val: lf * rf}, nil
		case "/":
			if rf == 0 {
				return nil, fmt.Errorf("sıfıra bölme hatası")
			}
			return &vm.FloatVal{Val: lf / rf}, nil
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
func filterRows(rows []map[string]interface{}, cond string, known map[string]bool, params []vm.Value) (matched, rest []map[string]interface{}, err error) {
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
			return nil, nil, fmt.Errorf("Koşul hatası (%s): %v", kosulOzeti(cond), err)
		}
		if ok {
			matched = append(matched, row)
		} else {
			rest = append(rest, row)
		}
	}
	return matched, rest, nil
}

// eslesmeler her satırın koşula uyup uymadığını verir (koşul boşsa hepsi uyar).
func eslesmeler(rows []map[string]interface{}, cond string, known map[string]bool, params []vm.Value) ([]bool, error) {
	uyar := make([]bool, len(rows))
	if strings.TrimSpace(cond) == "" {
		if len(params) > 0 {
			return nil, fmt.Errorf("Koşul hatası: parametre verildi ama koşul boş")
		}
		for i := range uyar {
			uyar[i] = true
		}
		return uyar, nil
	}
	c, err := compileRowCondition(cond, known, params)
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		ok, err := c.matches(row)
		if err != nil {
			return nil, fmt.Errorf("Koşul hatası (%s): %v", kosulOzeti(cond), err)
		}
		uyar[i] = ok
	}
	return uyar, nil
}

// paramsArg isteğe bağlı parametre listesini (Liste) []Value olarak döndürür. Önce adlı
// argümanların yalnızca parametreler (ve fonksiyonun ek adları) olduğunu denetler; bilinmeyen
// ya da kaldırılmış bir ad (params, nerede) sessizce yok sayılmaz, asıl ad önerilir.
func paramsArg(fn string, args []vm.Value, idx int, named map[string]vm.Value, ekAdlar ...baglama.Ad) ([]vm.Value, error) {
	if err := baglama.AdliDenetle(fn, named, append([]baglama.Ad{adParametreler}, ekAdlar...)...); err != nil {
		return nil, err
	}
	var raw vm.Value
	if idx < len(args) {
		raw = args[idx]
	}
	for _, k := range parametreYazimlari {
		if pv, ok := named[k]; ok {
			raw = pv
		}
	}
	if raw == nil || raw.Type() == vm.VAL_NULL {
		return nil, nil
	}
	lv, ok := raw.(*vm.ListVal)
	if !ok {
		return nil, fmt.Errorf("parametreler bir Liste olmalıdır (örn: [\"Yapay Zeka\", 80000])")
	}
	return lv.Elements, nil
}
