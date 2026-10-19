package interp

import (
	"fmt"
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// Taşınan kodlar: işi başka bir biçime taşınan kodların (fonksiyondan metoda ya da
// metottan fonksiyona; definitions.json'da "taşındı") eski adları kabul edilmez; yazılınca hata
// yeni biçimi gösterir: uzunluk(x) → x.uzunluk, liste.toplam() → toplam(liste). Kural:
// bir değeri dönüştüren ya da ona soru soran iş metottur, matematik ve tensör işi
// fonksiyondur (bkz. docs/spec/13_yerlesikler.md 13.6). Bu işlevler yalnızca hata yolunda çalışır.

// tasinanUyeHatasi prop, obj'nin tipinde taşınmış bir üyenin adıysa (ve yeni fonksiyon bu
// tipi alıyorsa) yeni biçimi gösteren hata metnini verir; değilse boş metin.
// ör. "'liste.toplam()' TR'de yok; 'toplam(liste)' yazın".
func movedMemberError(obj Value, prop string) string {
	prevVal, ok := codes.FindDead(codes.NamespaceMember, prop)
	if !ok {
		return ""
	}
	if codes.Removed(prevVal) {
		return hint.RemovedError(prop)
	}
	fresh, _ := codes.MovedTo(prevVal)
	typ, ok := codes.Find(codes.NamespaceType, string(obj.Type()))
	if !ok {
		return ""
	}
	t, _ := codes.DefinitionOf(fresh)
	if t.Kind != codes.KindFunction || len(t.Signature.Params) == 0 || !slices.Contains(t.Signature.Params[0].Types, typ) {
		return ""
	}
	a := codes.ReceiverName(typ)
	return fmt.Sprintf("'%s.%s()' TR'de yok; '%s(%s)' yazın", a, prop, codes.Name(fresh), a)
}
