package ml

import (
	"fmt"
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kaynaştırılan fonksiyonun kapanıştan okuduğu sayılar (dil tanımı 15.44). Kayıt sayıyı
// programa sabit olarak yazar; sayı sonradan değişirse kaynaşmış sonuç eski kalırdı. Bu yüzden
// ilk çağrıda fonksiyonun gövdesi (ve kapanıştan çağırdığı kullanıcı fonksiyonlarının gövdeleri)
// gezilir: kapanıştan okunan her yol (değişken, alan, tam sayı liste indeksi, metin sözlük
// anahtarı zinciri; değişken indeks ya da anahtarın değeri yolun parçası) değeriyle kaydedilir ve
// her çağrıda, cihazdan önce yeniden değerlendirilip karşılaştırılır. Argümandan ya da fonksiyonun
// kendi adlarından türeyen değer denetlenmez (argüman bileşimi önbellek anahtarındadır). Kapanıştan
// çağrılan ve argümandan beslenmeyen bir kullanıcı fonksiyonunun sonucu yolu olmayan değerdir:
// kayıtta hatadır. Kapanıştan tensör okumak izde hatadır (H3108).

// fuseCapture kapanıştan okunan bir yolun kaydıdır: yolun metni, kaydedildiği değer ve onu
// değerlendirmek için gereken ortam.
type fuseCapture struct {
	text  string
	read  func() (interp.Value, bool)
	want  string
	shown any // kaydedilen değerin gösterimi (iletide)
}

// captureKey bir sayının ya da anahtarın karşılaştırma anahtarıdır (tipiyle: 2 ile 2.0 ayrıdır).
func captureKey(v interp.Value) (string, bool) {
	switch x := v.(type) {
	case *interp.IntVal:
		return "i" + strconv.FormatInt(x.Val, 10), true
	case *interp.FloatVal:
		return "f" + strconv.FormatFloat(x.Val, 'g', -1, 64), true
	case *interp.StringVal:
		return "s" + x.Val, true
	}
	return "", false
}

// recordCaptures f'in kapanıştan okuduğu sayıları kaydeder. Yolu olmayan kapanış değeri hatadır.
func recordCaptures(f interp.Value) ([]fuseCapture, error) {
	fv, ok := f.(*interp.FuncVal)
	if !ok {
		return nil, nil // yerleşik fonksiyonun kapanışı yoktur
	}
	w := &captureWalk{seen: map[*ast.BlockStatement]bool{}}
	w.function(fv)
	return w.caps, w.err
}

// checkCaptures kaydedilen yolları yeniden değerlendirir; değeri değişen yol hatadır.
func checkCaptures(fn any, caps []fuseCapture) error {
	for _, c := range caps {
		v, ok := c.read()
		now, _ := captureKey(v)
		if !ok || now != c.want {
			var shown any = codes.NameRef(codes.KwNone) // yol artık okunamıyor (anahtar silindi)
			if ok {
				shown = interp.Shown(v)
			}
			return messages.New(messages.H6986, fn, c.text, c.shown, shown)
		}
	}
	return nil
}

type captureWalk struct {
	seen map[*ast.BlockStatement]bool
	caps []fuseCapture
	err  error
}

// scope bir fonksiyonun gövdesindeki adlardır: parametreler ve gövdede tanımlanan her ad
// (iç fonksiyonların parametreleri dahil). Bunlardan biriyle başlayan yol kapanıştan değildir.
type scope struct {
	fv    *interp.FuncVal
	local map[string]bool
}

func (w *captureWalk) function(fv *interp.FuncVal) {
	if fv.Body == nil || w.seen[fv.Body] {
		return
	}
	w.seen[fv.Body] = true
	s := scope{fv: fv, local: map[string]bool{}}
	for _, p := range fv.Parameters {
		s.local[p.Name] = true
	}
	ast.Walk(fv.Body, func(n any) bool {
		switch x := n.(type) {
		case *ast.VarStatement:
			s.local[x.Name.Value] = true
		case *ast.MultiVarStatement:
			for _, id := range x.Names {
				s.local[id.Value] = true
			}
		case *ast.ForInStatement:
			s.local[x.KeyVar], s.local[x.ValueVar] = true, true
		case *ast.FunctionLiteral:
			for _, p := range x.Parameters {
				s.local[p.Name] = true
			}
			if x.Name != "" {
				s.local[x.Name] = true
			}
		}
		return true
	})
	ast.Walk(fv.Body, func(n any) bool { return w.visit(s, n) })
}

// visit bir düğümü işler; false onun çocuklarına inilmeyeceğini söyler.
func (w *captureWalk) visit(s scope, n any) bool {
	if w.err != nil {
		return false
	}
	switch x := n.(type) {
	case *ast.CallExpression:
		w.call(s, x)
		return false
	case *ast.Identifier, *ast.MemberExpression, *ast.IndexExpression:
		if w.path(s, x.(ast.Expression)) {
			return false
		}
	}
	return true
}

// call çağrının argümanlarını gezer. Kapanıştan çağrılan kullanıcı fonksiyonu argümandan
// besleniyorsa gövdesine inilir; beslenmiyorsa sonucu yolu olmayan kapanış değeridir.
func (w *captureWalk) call(s scope, c *ast.CallExpression) {
	for _, a := range c.Arguments {
		ast.Walk(a, func(n any) bool { return w.visit(s, n) })
	}
	for _, na := range c.NamedArgs {
		ast.Walk(na.Value, func(n any) bool { return w.visit(s, n) })
	}
	switch f := c.Function.(type) {
	case *ast.Identifier:
		if s.local[f.Value] {
			return
		}
		callee, ok := s.fv.Env.Get(f.Value)
		fv, user := callee.(*interp.FuncVal)
		if !ok || !user {
			return // yerleşik ya da bilinmeyen: sonucu argümanlarından gelir
		}
		if !s.usesLocal(c) {
			w.err = messages.New(messages.H6987, codes.NameRef(codes.FnFuse), f.Value)
			return
		}
		w.function(fv)
	case *ast.MemberExpression:
		ast.Walk(f.Object, func(n any) bool { return w.visit(s, n) }) // metodun nesnesi
	default:
		ast.Walk(f, func(n any) bool { return w.visit(s, n) })
	}
}

// usesLocal çağrının bir argümanının fonksiyonun kendi adlarından birini okuyup okumadığını söyler.
func (s scope) usesLocal(c *ast.CallExpression) bool {
	found := false
	visit := func(n any) bool {
		if id, ok := n.(*ast.Identifier); ok && s.local[id.Value] {
			found = true
		}
		return !found
	}
	for _, a := range c.Arguments {
		ast.Walk(a, visit)
	}
	for _, na := range c.NamedArgs {
		ast.Walk(na.Value, visit)
	}
	return found
}

// path e kapanıştan okunan bir yolsa değeri sayıysa kaydeder ve true verir (çocuklarına
// inilmez). Fonksiyonun kendi adıyla başlayan ya da yol olmayan ifade false verir.
func (w *captureWalk) path(s scope, e ast.Expression) bool {
	read, text, indexVars, ok := s.pathReader(e)
	if !ok {
		return false
	}
	for _, iv := range indexVars {
		w.add(iv.text, iv.read)
	}
	w.add(text, read)
	return true
}

func (w *captureWalk) add(text string, read func() (interp.Value, bool)) {
	v, ok := read()
	if !ok {
		return
	}
	if k, number := captureKey(v); number {
		w.caps = append(w.caps, fuseCapture{text: text, read: read, want: k, shown: interp.Shown(v)})
	}
}

type indexVar struct {
	text string
	read func() (interp.Value, bool)
}

// pathReader e'yi kapanış ortamında değerlendiren bir okuyucu kurar: kök kapanıştan bir ad,
// sonra alan, tam sayı ya da metin değişmez indeks, ya da kapanıştan bir değişken indeks.
func (s scope) pathReader(e ast.Expression) (func() (interp.Value, bool), string, []indexVar, bool) {
	switch x := e.(type) {
	case *ast.Identifier:
		if s.local[x.Value] {
			return nil, "", nil, false
		}
		name, fv := x.Value, s.fv
		read := func() (interp.Value, bool) {
			if name == codes.Name(codes.KwSelf) && fv.BoundSelf != nil {
				return fv.BoundSelf, true
			}
			return fv.Env.Get(name)
		}
		return read, name, nil, true
	case *ast.MemberExpression:
		obj, text, ivs, ok := s.pathReader(x.Object)
		if !ok || x.Safe {
			return nil, "", nil, false
		}
		field := x.Property
		read := func() (interp.Value, bool) {
			o, ok := obj()
			si, inst := o.(*interp.StructInstanceVal)
			if !ok || !inst {
				return nil, false
			}
			v, ok := si.Fields[field]
			return v, ok
		}
		return read, text + "." + field, ivs, true
	case *ast.IndexExpression:
		obj, text, ivs, ok := s.pathReader(x.Left)
		if !ok {
			return nil, "", nil, false
		}
		var key func() (interp.Value, bool)
		var keyText string
		switch k := x.Index.(type) {
		case *ast.IntegerLiteral:
			v := &interp.IntVal{Val: k.Value}
			key, keyText = func() (interp.Value, bool) { return v, true }, strconv.FormatInt(k.Value, 10)
		case *ast.StringLiteral:
			v := &interp.StringVal{Val: k.Value}
			key, keyText = func() (interp.Value, bool) { return v, true }, strconv.Quote(k.Value)
		default:
			kr, kt, kivs, ok := s.pathReader(x.Index)
			if !ok {
				return nil, "", nil, false
			}
			ivs = append(append(ivs, kivs...), indexVar{kt, kr})
			key, keyText = kr, kt
		}
		read := func() (interp.Value, bool) {
			o, ok1 := obj()
			k, ok2 := key()
			if !ok1 || !ok2 {
				return nil, false
			}
			return indexValue(o, k)
		}
		return read, fmt.Sprintf("%s[%s]", text, keyText), ivs, true
	}
	return nil, "", nil, false
}

// indexValue listenin tam sayı indeksini ya da sözlüğün metin anahtarını okur; başka bileşim
// yol değildir.
func indexValue(o, k interp.Value) (interp.Value, bool) {
	switch c := o.(type) {
	case *interp.ListVal:
		i, ok := k.(*interp.IntVal)
		if !ok {
			return nil, false
		}
		n := int64(len(c.Elements))
		if i.Val < -n || i.Val >= n {
			return nil, false
		}
		if i.Val < 0 {
			return c.Elements[i.Val+n], true
		}
		return c.Elements[i.Val], true
	case *interp.DictVal:
		key, ok := k.(*interp.StringVal)
		if !ok {
			return nil, false
		}
		v, ok := c.Pairs[key.Val]
		return v, ok
	}
	return nil, false
}
