package interp

import "github.com/sekerahmet/TCB/internal/lang/ast"

// --- ÇAĞRIDA BİLİNEN ÖĞE TİPİ (15.18, 13) ---
//
// Boş listenin öğelerinden tipi çıkmaz: TR_sum'ın sonucu (tensör, TR_Float ya da TR_Int) ve boş tensör
// listesinin hatası öğe tipine bağlıdır. Denetleyici TR_sum ve TR_mean çağrısına argümanın bildiği
// öğe tipini yazar (ast.CallExpression.ListElem); yazılı tipli yer (değişken, parametre, dönüş,
// alan) ile çıkarımla gelen tip (süzgeçli liste, fonksiyonun dönüşü) aynı yoldan geçer. Çağrı boş
// listeyi o tipi taşıyan yeni bir boş listeyle verir; yerleşik öğe tipini TensorList ve FloatList ile
// okur. Liste değişmez, başka yere kaçmaz: yerleşik boş listeyi saklamaz.

// knownListElem argümanın kodunu, boş liste geldiğinde elem tipini taşıyan boş listeyi veren kodla sarar.
func knownListElem(arg exprCode, elem ast.ListElem) exprCode {
	return func(v *VM, env *Environment) (Value, error) {
		val, err := arg(v, env)
		if l, ok := val.(*ListVal); ok && err == nil && len(l.Elements) == 0 {
			return &ListVal{elem: elem}, nil
		}
		return val, err
	}
}

// TensorList denetleyicinin bu boş listeyi tensör listesi bildiğini söyler: toplamı hatadır (15.18).
func (l *ListVal) TensorList() bool { return l.elem == ast.ListElemTensor }
