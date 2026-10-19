package interp

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Yerleşik ad gölgeleme: kullanıcı bir yerleşiğin adını (uzunluk, toplam, girdi ...)
// değişken ya da parametre adı olarak kullanınca o kapsamda yerleşik artık görünmez.
// Denetleyici bunun için bir uyarı verir (internal/lang/checker); burada, gölgelenmiş ad bir
// fonksiyon gibi çağrılırsa çıkan çalışma zamanı hatası, sıradan "çağrılamaz" mesajı
// yerine gölgelemeyi ve tanımın satırını söyler. Kayıt derleme anında, yalnızca yerleşik
// adları için tutulur; çalışma anına yalnızca hata yolunda bir maliyet ekler.

// golgeTanimi bir kapsamda yerleşik adını gölgeleyen tanımdır.
type golgeTanimi struct {
	satir int
	ne    string // "bir değişkene", "bir parametreye" ...
}

// yerlesikAdi ad sözlükteki bir yerleşik fonksiyonun (G/T/V/W) adı mı?
func yerlesikAdi(ad string) bool {
	k, ok := codes.Find(codes.NamespaceGlobal, ad)
	return ok && k.IsBuiltin()
}

// golgeKaydet ad bir yerleşik adıysa bu kapsamdaki tanımını kaydeder (kapsamdaki ilk
// tanım tutulur).
func (k *derleyiciKapsam) golgeKaydet(ad string, satir int, ne string) {
	if !yerlesikAdi(ad) {
		return
	}
	if k.golge == nil {
		k.golge = map[string]golgeTanimi{}
	}
	if _, var_ := k.golge[ad]; !var_ {
		k.golge[ad] = golgeTanimi{satir: satir, ne: ne}
	}
}

// golgeBul ad, bu kapsamdan dışa doğru bir kullanıcı tanımıyla gölgelenmişse tanımı verir.
func (k *derleyiciKapsam) golgeBul(ad string) (golgeTanimi, bool) {
	for c := k; c != nil; c = c.ust {
		if g, ok := c.golge[ad]; ok {
			return g, true
		}
	}
	return golgeTanimi{}, false
}

// deyimGolgeleri deyimin bu kapsama tanımladığı adları satırlarıyla golgeKaydet'e verir
// (deyimAdlari'nın yerleşik gölgeleme için gereken kısmı).
func (k *derleyiciKapsam) deyimGolgeleri(st ast.Statement) {
	switch s := st.(type) {
	case *ast.VarStatement:
		k.golgeKaydet(s.Name.Value, s.Line(), "bir değişkene")
	case *ast.MultiVarStatement:
		for _, n := range s.Names {
			k.golgeKaydet(n.Value, s.Line(), "bir değişkene")
		}
	}
}

// golgeliCagri, adı gölgelenmiş bir yerleşik olan çağrının kodunu sarar: çağrı hata
// verirse ve ad çağrılamaz bir değere bağlıysa hata gölgelemeyi anlatan mesajla
// değiştirilir.
func golgeliCagri(kd ifadeKodu, fonk ifadeKodu, ad string, g golgeTanimi, line int) ifadeKodu {
	return func(v *VM, env *Environment) (Value, error) {
		val, err := kd(v, env)
		if err == nil {
			return val, nil
		}
		if fv, e := fonk(v, env); e == nil && !cagrilabilir(fv) {
			return nil, fmt.Errorf("Satır %d: '%s' bir fonksiyon değil (%s); %d. satırda %s verildi, yerleşik '%s' bu kapsamda gölgelendi",
				line, ad, fv.Type(), g.satir, g.ne, ad)
		}
		return nil, err
	}
}
