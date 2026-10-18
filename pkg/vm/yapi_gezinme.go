package vm

import "sort"

// AlanlariGez yapı nesnesinin alanlarını sabit bir sırayla fn'e verir: önce yapı
// tanımındaki alanlar tanım sırasıyla, ardından tanımda olmayıp sonradan atanan
// alanlar (kendi.yeni = ...) ad sırasıyla. Motorlar (ör. parametreleri toplayan
// yerleşik) nesneyi bu sırayla gezer; harita gezinme sırasına bağlı kalınmaz.
func (si *StructInstanceVal) AlanlariGez(fn func(ad string, deger Value)) {
	tanimli := make(map[string]bool, len(si.Def.Fields))
	for _, f := range si.Def.Fields {
		tanimli[f.Name] = true
		if v, ok := si.Fields[f.Name]; ok {
			fn(f.Name, v)
		}
	}
	var ek []string
	for ad := range si.Fields {
		if !tanimli[ad] {
			ek = append(ek, ad)
		}
	}
	sort.Strings(ek)
	for _, ad := range ek {
		fn(ad, si.Fields[ad])
	}
}
