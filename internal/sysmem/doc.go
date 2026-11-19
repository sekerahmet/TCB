// Package sysmem, makinenin toplam fiziksel belleğini (Physical) ve şu an kullanılabilir
// belleğini (Available) işletim sisteminden okur. Dil çekirdeği ve hesap çekirdeği ayırma
// sınırlarında toplamı, TR_machine() (19.12) ikisini kullanır. Sürecin bellek tavanını da tutar
// (TR_MEMORY_LIMIT, 19.12a; limit.go): Go'nun sınırı tavandan öbek dışı blokların düşüldüğü kadardır.
//
// Layer: system memory (internal/sysmem); hiçbir iç paketi içe aktarmaz.
package sysmem
