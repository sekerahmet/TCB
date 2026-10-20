package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yerleşik ad gölgeleme: kullanıcı bir yerleşiğin adını (uzunluk, toplam, girdi ...)
// değişken ya da parametre adı olarak kullanınca o kapsamda yerleşik artık görünmez.
// Denetleyici bunun için bir uyarı verir (internal/lang/checker); burada, gölgelenmiş ad bir
// fonksiyon gibi çağrılırsa çıkan çalışma zamanı hatası, sıradan "çağrılamaz" mesajı
// yerine gölgelemeyi ve tanımın satırını söyler. Kayıt derleme anında, yalnızca yerleşik
// adları için tutulur; çalışma anına yalnızca hata yolunda bir maliyet ekler.

// shadowDef bir kapsamda yerleşik adını gölgeleyen tanımdır.
type shadowDef struct {
	lineNo  int
	message messages.Code // tanımın türünü anlatan tam cümle: messages.H2450 (değişken), H2451 (parametre) ...
}

// builtinName ad sözlükteki bir yerleşik fonksiyonun (G/T/V/W) adı mı?
func builtinName(name string) bool {
	k, ok := codes.Find(codes.NamespaceGlobal, name)
	return ok && k.IsBuiltin()
}

// recordShadow ad bir yerleşik adıysa bu kapsamdaki tanımını kaydeder (kapsamdaki ilk
// tanım tutulur).
func (k *compilerScope) recordShadow(name string, lineNo int, message messages.Code) {
	if !builtinName(name) {
		return
	}
	if k.shadow == nil {
		k.shadow = map[string]shadowDef{}
	}
	if _, var_ := k.shadow[name]; !var_ {
		k.shadow[name] = shadowDef{lineNo: lineNo, message: message}
	}
}

// findShadow ad, bu kapsamdan dışa doğru bir kullanıcı tanımıyla gölgelenmişse tanımı verir.
func (k *compilerScope) findShadow(name string) (shadowDef, bool) {
	for c := k; c != nil; c = c.parent {
		if g, ok := c.shadow[name]; ok {
			return g, true
		}
	}
	return shadowDef{}, false
}

// statementShadows deyimin bu kapsama tanımladığı adları satırlarıyla golgeKaydet'e verir
// (deyimAdlari'nın yerleşik gölgeleme için gereken kısmı).
func (k *compilerScope) statementShadows(st ast.Statement) {
	switch s := st.(type) {
	case *ast.VarStatement:
		k.recordShadow(s.Name.Value, s.Line(), messages.H2450)
	case *ast.MultiVarStatement:
		for _, n := range s.Names {
			k.recordShadow(n.Value, s.Line(), messages.H2450)
		}
	}
}

// shadowedCall, adı gölgelenmiş bir yerleşik olan çağrının kodunu sarar: çağrı hata
// verirse ve ad çağrılamaz bir değere bağlıysa hata gölgelemeyi anlatan mesajla
// değiştirilir.
func shadowedCall(kd exprCode, fun exprCode, name string, g shadowDef, line int) exprCode {
	return func(v *VM, env *Environment) (Value, error) {
		val, err := kd(v, env)
		if err == nil {
			return val, nil
		}
		if fv, e := fun(v, env); e == nil && !callable(fv) {
			return nil, messages.At(line, 0, messages.New(g.message, name, fv.Type(), g.lineNo))
		}
		return nil, err
	}
}
