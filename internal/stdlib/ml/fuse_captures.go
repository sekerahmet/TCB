package ml

import (
	"fmt"
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
// İzlenen değer sayı, mantık değeri, metin ya da yokluktur; tensör izde hatadır (H3108).
// Fonksiyon değeri kimliğiyle kaydedilir (kullanıcı fonksiyonunda gövde ve ortam, bağlı metotta
// gövde ve TR_self, yerleşikte adı) ve gövdesi nerede okunursa okunsun (çağrı, takma ad,
// argüman) gezilir; gövdesi olmayan kullanıcı fonksiyonu izlenemez. Öbür değerler (liste,
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
	captureRecorded captureClass = iota // sayı, mantık değeri, metin, yokluk, fonksiyon: kaydedilir
	captureSkipped                      // tensör: kaydedilmez (izde H3108)
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
	case *interp.FuncVal:
		return funcKey(x), captureRecorded
	case *interp.BuiltinVal:
		return "B" + x.Name, captureRecorded
	case *TensorValue:
		return "", captureSkipped
	}
	return "", captureUntraced
}

// funcKey kullanıcı fonksiyonunun kimliğidir. Bağlı metot her okumada yeni bir değerdir: kimliği
// gövdesi ve TR_self'idir. Öbür fonksiyonda gövde ve ortam (aynı gövdeli iki kapanış ayrıdır).
func funcKey(f *interp.FuncVal) string {
	if f.BoundSelf != nil {
		return fmt.Sprintf("M%p|%p", f.Body, f.BoundSelf)
	}
	return fmt.Sprintf("F%p|%p", f.Body, f.Env)
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

// scope bir noktada görünen yerel adlardır: parametreler ve o noktaya kadar tanımlanan adlar.
// Kapsam bloğa göredir: blokta tanımlanan ad (döngü değişkeni dahil) yalnız o blokta yereldir,
// iç fonksiyonun parametresi yalnız onun gövdesinde. Bunlardan biriyle başlayan yol kapanıştan
// değildir. self, kapanıştaki bir nesnenin metodunda
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
	w.block(s, fv.Body)
}

// child s'nin içindeki bir bloğun kapsamıdır: dışın adlarını görür, kendi tanımladıkları dışarı
// çıkmaz.
func (s scope) child() scope {
	local := make(map[string]bool, len(s.local))
	for k := range s.local {
		local[k] = true
	}
	return scope{fv: s.fv, local: local, self: s.self, selfText: s.selfText}
}

// block bir bloğu kendi kapsamında, sırayla gezer: adlandırılmış iç fonksiyonun adı bloğun
// başından yereldir, değişken tanımından sonra.
func (w *captureWalk) block(s scope, b *ast.BlockStatement) {
	in := s.child()
	for _, st := range b.Statements {
		ast.Walk(st, func(n any) bool {
			switch x := n.(type) {
			case *ast.FunctionLiteral:
				if x.Name != "" {
					in.local[x.Name] = true
				}
				return false
			case *ast.BlockStatement:
				return false
			}
			return true
		})
	}
	for _, st := range b.Statements {
		w.walk(in, st)
	}
}

func (w *captureWalk) walk(s scope, n any) {
	ast.Walk(n, func(n any) bool { return w.visit(s, n) })
}

// visit bir düğümü işler; false onun çocuklarına inilmeyeceğini söyler.
func (w *captureWalk) visit(s scope, n any) bool {
	if w.err != nil {
		return false
	}
	switch x := n.(type) {
	case *ast.BlockStatement:
		w.block(s, x)
		return false
	case *ast.VarStatement:
		w.walk(s, x.Value)
		s.local[x.Name.Value] = true
		return false
	case *ast.MultiVarStatement:
		for _, v := range x.Values {
			w.walk(s, v)
		}
		for _, id := range x.Names {
			s.local[id.Value] = true
		}
		return false
	case *ast.ForInStatement:
		w.walk(s, x.Iterable)
		in := s.child()
		in.local[x.KeyVar], in.local[x.ValueVar] = true, true
		w.block(in, x.Body)
		return false
	case *ast.FunctionLiteral:
		if x.Body != nil {
			in := s.child()
			for _, p := range x.Parameters {
				in.local[p.Name] = true
			}
			w.block(in, x.Body)
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
		if b, builtin := callee.(*interp.BuiltinVal); builtin && b.Name != f.Value {
			w.identity(s, f) // yerleşiğe bağlanmış ad (op = TR_relu)
		}
		fv, user := callee.(*interp.FuncVal)
		if !ok || !user {
			return // yerleşik ya da bilinmeyen: sonucu argümanlarından gelir
		}
		w.identity(s, f)
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
	w.identity(s, f)
	w.enter(s, c, fv, self, selfText, text+"."+f.Property)
}

// identity çağrılan fonksiyon değerinin yolunu kimliğiyle kaydeder; gövdesini çağrı gezer.
func (w *captureWalk) identity(s scope, e ast.Expression) {
	if read, text, _, ok := s.pathReader(e); ok {
		w.record(text, read)
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
// izlenemez (H6987). Okunan kullanıcı fonksiyonunun gövdesi de gezilir (takma ad, argüman).
func (w *captureWalk) add(text string, read reader) {
	if v := w.record(text, read); v != nil {
		if fv, ok := v.(*interp.FuncVal); ok {
			w.function(fv, nil, "")
		}
	}
}

// record yolu değeriyle kaydeder ve kaydedilen değeri verir (kaydedilmediyse nil).
func (w *captureWalk) record(text string, read reader) interp.Value {
	if w.err != nil {
		return nil
	}
	v, ok := read(w.vm)
	k, class := captureKey(v)
	if fv, fn := v.(*interp.FuncVal); fn && fv.Body == nil {
		class = captureUntraced // gövdesi gezilemeyen fonksiyon
	}
	switch {
	case !ok || class == captureUntraced:
		w.err = messages.New(messages.H6987, codes.NameRef(codes.FnFuse), text, "value")
	case class == captureRecorded:
		w.caps = append(w.caps, fuseCapture{text: text, read: read, want: k, shown: interp.Shown(v)})
		return v
	}
	return nil
}
