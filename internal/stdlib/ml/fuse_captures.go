package ml

import (
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kaynaştırılan fonksiyonun kapanıştan okuduğu değerler (dil tanımı 15.44). Kayıt değeri programa
// sabit olarak yazar; değer sonradan değişirse kaynaşmış sonuç eski kalırdı. Bu yüzden ilk çağrıda
// fonksiyonun gövdesi gezilir; kapanıştan çağırdığı kullanıcı fonksiyonlarının, metotların
// (TR_self nesnenin yoluna bağlanır), fonksiyon değerli alanların ve modül fonksiyonlarının
// gövdeleri de. Kapanıştan okunan her yol (değişken, nesnenin alanı, liste indeksi, sözlük
// anahtarı zinciri, modülün adı; değişken indeksin değeri de yolun parçası) değeriyle kaydedilir
// ve her çağrıda, cihazdan önce yeniden okunup karşılaştırılır. Argümandan ya da fonksiyonun
// kendi adlarından türeyen değer denetlenmez (argüman bileşimi önbellek anahtarındadır).
//
// Genel kural: kapanıştan okunan bir değer izlenemiyorsa sessiz geçmez, kayıtta hatadır (H6987).
// İzlenen değer sayı, mantık değeri, metin ya da yokluktur; tensör izde hatadır (H3108),
// fonksiyon değeri kaydedilmez (çağrısı gövdesine inilerek izlenir). Öbür değerler (liste,
// sözlük, nesne) bütün olarak okunuyorsa, okunamayan bir yol, kapanıştaki çekirdek değerin
// (liste, sözlük, metin) yerleşik özelliği ya da metodu ve argümandan beslenmeyen bir kapanış
// çağrısı izlenemez.

// reader bir yolu kapanış ortamında değerlendirir (VM, programın üye okumasıyla aynı yol için).
type reader func(*interp.VM) (interp.Value, bool)

// fuseCapture kapanıştan okunan bir yolun kaydıdır: yolun metni, kaydedildiği değer ve onu
// değerlendirmek için gereken ortam.
type fuseCapture struct {
	text  string
	read  reader
	want  string
	shown any // kaydedilen değerin gösterimi (iletide)
}

// captureClass bir kapanış değerinin kayıttaki yeridir.
type captureClass int

const (
	captureRecorded captureClass = iota // sayı, mantık değeri, metin, yokluk: kaydedilir
	captureSkipped                      // tensör (izde H3108) ve fonksiyon: kaydedilmez
	captureUntraced                     // liste, sözlük, nesne...: bütün olarak izlenemez (H6987)
)

// captureKey bir değerin karşılaştırma anahtarı ve sınıfıdır (tipiyle: 2 ile 2.0 ayrıdır).
func captureKey(v interp.Value) (string, captureClass) {
	switch x := v.(type) {
	case *interp.IntVal:
		return "i" + strconv.FormatInt(x.Val, 10), captureRecorded
	case *interp.FloatVal:
		return "f" + strconv.FormatFloat(x.Val, 'g', -1, 64), captureRecorded
	case *interp.StringVal:
		return "s" + x.Val, captureRecorded
	case *interp.BoolVal:
		return "b" + strconv.FormatBool(x.Val), captureRecorded
	case *interp.NullVal:
		return "n", captureRecorded
	case *TensorValue, *interp.FuncVal, *interp.BuiltinVal:
		return "", captureSkipped
	}
	return "", captureUntraced
}

// recordCaptures f'in kapanıştan okuduğu değerleri kaydeder. İzlenemeyen kapanış değeri hatadır.
func recordCaptures(m *interp.VM, f interp.Value) ([]fuseCapture, error) {
	fv, ok := f.(*interp.FuncVal)
	if !ok {
		return nil, nil // yerleşik fonksiyonun kapanışı yoktur
	}
	w := &captureWalk{vm: m, seen: map[walked]bool{}}
	w.function(fv, nil, "")
	return w.caps, w.err
}

// checkCaptures kaydedilen yolları yeniden değerlendirir; değeri değişen yol hatadır.
func checkCaptures(fn any, m *interp.VM, caps []fuseCapture) error {
	for _, c := range caps {
		v, ok := c.read(m)
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
	vm   *interp.VM
	seen map[walked]bool
	caps []fuseCapture
	err  error
}

// scope bir fonksiyonun adlarıdır: parametreler ve gövdede tanımlanan adlar; iç fonksiyonun
// adları yalnız onun kapsamındadır (iç fonksiyonun parametresi dıştaki okumayı yerel yapmaz).
// Bunlardan biriyle başlayan yol kapanıştan değildir. self, kapanıştaki bir nesnenin metodunda
// TR_self'in okuyucusudur (yolu selfText).
type scope struct {
	fv       *interp.FuncVal
	local    map[string]bool
	self     reader
	selfText string
}

// walked gezilen bir gövdedir; aynı metot ayrı nesnelerle ayrı gezilir (yolları ayrıdır).
type walked struct {
	body *ast.BlockStatement
	self string
}

func (w *captureWalk) function(fv *interp.FuncVal, self reader, selfText string) {
	key := walked{fv.Body, selfText}
	if fv.Body == nil || w.seen[key] {
		return
	}
	w.seen[key] = true
	s := scope{fv: fv, local: map[string]bool{}, self: self, selfText: selfText}
	for _, p := range fv.Parameters {
		s.local[p.Name] = true
	}
	declare(s.local, fv.Body)
	ast.Walk(fv.Body, func(n any) bool { return w.visit(s, n) })
}

// declare gövdede tanımlanan adları local'e ekler; iç fonksiyonun yalnız adı dışarıdadır,
// parametreleri ve gövdesi kendi kapsamıdır (visit).
func declare(local map[string]bool, body *ast.BlockStatement) {
	ast.Walk(body, func(n any) bool {
		switch x := n.(type) {
		case *ast.VarStatement:
			local[x.Name.Value] = true
		case *ast.MultiVarStatement:
			for _, id := range x.Names {
				local[id.Value] = true
			}
		case *ast.ForInStatement:
			local[x.KeyVar], local[x.ValueVar] = true, true
		case *ast.FunctionLiteral:
			if x.Name != "" {
				local[x.Name] = true
			}
			return false
		}
		return true
	})
}

// inner iç fonksiyonun kapsamıdır: dışın adları, parametreleri ve gövdesinde tanımlananlar.
func (s scope) inner(f *ast.FunctionLiteral) scope {
	local := make(map[string]bool, len(s.local)+len(f.Parameters))
	for k := range s.local {
		local[k] = true
	}
	for _, p := range f.Parameters {
		local[p.Name] = true
	}
	if f.Body != nil {
		declare(local, f.Body)
	}
	return scope{fv: s.fv, local: local, self: s.self, selfText: s.selfText}
}

// visit bir düğümü işler; false onun çocuklarına inilmeyeceğini söyler.
func (w *captureWalk) visit(s scope, n any) bool {
	if w.err != nil {
		return false
	}
	switch x := n.(type) {
	case *ast.FunctionLiteral:
		if x.Body != nil {
			in := s.inner(x)
			ast.Walk(x.Body, func(n any) bool { return w.visit(in, n) })
		}
		return false
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
// besleniyorsa gövdesine inilir; beslenmiyorsa sonucu izlenemez.
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
		w.enter(s, c, fv, nil, "", f.Value)
	case *ast.MemberExpression:
		w.method(s, c, f)
	case *ast.IndexExpression:
		w.indexedCall(s, f)
	default:
		ast.Walk(f, func(n any) bool { return w.visit(s, n) })
	}
}

// indexedCall 'x[i](...)' çağrısıdır: tip argümanlı kurucu (TR_Tensor[TR_Float](...)) yerleşiktir;
// kapanıştaki bir listeden ya da sözlükten alınan fonksiyonun çağrısı izlenemez.
func (w *captureWalk) indexedCall(s scope, f *ast.IndexExpression) {
	obj, text, _, ok := s.pathReader(f.Left)
	if !ok {
		ast.Walk(f, func(n any) bool { return w.visit(s, n) })
		return
	}
	switch o, _ := obj(w.vm); o.(type) {
	case *interp.ListVal, *interp.DictVal:
		w.err = messages.New(messages.H6987, codes.NameRef(codes.FnFuse), text+"[...]", "call")
	}
}

// enter kapanıştan çağrılan kullanıcı fonksiyonunun ya da metodun gövdesine iner; argümanlar
// fonksiyonun kendi adlarından beslenmiyorsa sonucu yolu olmayan değerdir (H6987).
func (w *captureWalk) enter(s scope, c *ast.CallExpression, fv *interp.FuncVal, self reader, selfText, text string) {
	if !s.usesLocal(c) {
		w.err = messages.New(messages.H6987, codes.NameRef(codes.FnFuse), text, "call")
		return
	}
	w.function(fv, self, selfText)
}

// method 'nesne.ad(...)' çağrısını işler. Nesne kapanıştan bir yolsa ve ad onun kullanıcı
// metodu (TR_self nesnenin yoluna bağlanır), fonksiyon değerli alanı ya da modül sözlüğünün
// fonksiyonuysa gövdesine inilir. Kapanıştaki öbür değerlerin (liste, sözlük, metin) yerleşik
// metodu izlenemez (H6987); fonksiyonun kendi değerinin metodunda yalnız nesne gezilir.
func (w *captureWalk) method(s scope, c *ast.CallExpression, f *ast.MemberExpression) {
	obj, text, ivs, ok := s.pathReader(f.Object)
	if !ok || f.Safe {
		ast.Walk(f.Object, func(n any) bool { return w.visit(s, n) })
		return
	}
	o, _ := obj(w.vm)
	var fv *interp.FuncVal
	var self reader
	switch x := o.(type) {
	case *interp.StructInstanceVal:
		if v, field := x.Fields[f.Property]; field {
			fv, _ = v.(*interp.FuncVal)
		} else if x.Def != nil {
			fv, self = x.Def.Methods[f.Property], obj
		}
	case *interp.DictVal:
		fv, _ = x.Pairs[f.Property].(*interp.FuncVal)
	case *TensorValue:
		ast.Walk(f.Object, func(n any) bool { return w.visit(s, n) }) // izde H3108
		return
	}
	for _, iv := range ivs {
		w.add(iv.text, iv.read)
	}
	if fv == nil {
		w.err = messages.New(messages.H6987, codes.NameRef(codes.FnFuse), text+"."+f.Property, "member")
		return
	}
	selfText := ""
	if self != nil {
		selfText = text
	}
	w.enter(s, c, fv, self, selfText, text+"."+f.Property)
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

// path e kapanıştan okunan bir yolsa kaydeder ve true verir (çocuklarına inilmez). Fonksiyonun
// kendi adıyla başlayan ya da yol olmayan ifade false verir.
func (w *captureWalk) path(s scope, e ast.Expression) bool {
	read, text, indexVars, ok := s.pathReader(e)
	if !ok {
		return false
	}
	if me, member := e.(*ast.MemberExpression); member && w.coreMember(s, me) {
		w.err = messages.New(messages.H6987, codes.NameRef(codes.FnFuse), text, "member")
		return true
	}
	for _, iv := range indexVars {
		w.add(iv.text, iv.read)
	}
	w.add(text, read)
	return true
}

// coreMember me'nin kapanıştaki bir çekirdek değerin yerleşik özelliği olduğunu söyler.
func (w *captureWalk) coreMember(s scope, me *ast.MemberExpression) bool {
	obj, _, _, ok := s.pathReader(me.Object)
	if !ok {
		return false
	}
	o, ok := obj(w.vm)
	return ok && coreValue(w.vm, o, me.Property)
}

// add yolun değerini sınıfına göre kaydeder; okunamayan ya da bütün olarak okunan kap değeri
// izlenemez (H6987).
func (w *captureWalk) add(text string, read reader) {
	if w.err != nil {
		return
	}
	v, ok := read(w.vm)
	k, class := captureKey(v)
	switch {
	case !ok || class == captureUntraced:
		w.err = messages.New(messages.H6987, codes.NameRef(codes.FnFuse), text, "value")
	case class == captureRecorded:
		w.caps = append(w.caps, fuseCapture{text: text, read: read, want: k, shown: interp.Shown(v)})
	}
}
