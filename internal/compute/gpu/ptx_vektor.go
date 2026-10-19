package gpu

import "strings"

// PTX yazıcısı: vektörel bellek erişimleri (ld/st .v2/.v4) ve eşzamansız paylaşımlı bellek
// kopyası (cp.async, sm_80+). Vektörel erişimin adresi erişim boyuna (4 float32 için 16
// bayt) hizalı olmalıdır; benzetim hizayı denetler.

// vliste yazmaçları PTX vektör işleneni olarak yazar: {%f1, %f2, %f3, %f4}.
func vliste(r []yazmac) string {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, y := range r {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(string(y))
	}
	sb.WriteByte('}')
	return sb.String()
}

// vekEki vektör genişliğinin komut eki (".v2", ".v4").
func vekEki(n int) string {
	switch n {
	case 2:
		return ".v2"
	case 4:
		return ".v4"
	}
	panic("gpu: vektör genişliği 2 ya da 4 olmalı (iç hata)")
}

// yukleV alan ("global" ya da "shared") belleğinden n float32'yi tek komutla yükler. p
// boş değilse yükleme koşulludur ve koşul yanlışsa yazmaçlar 0 kalır.
func (k *yazici) yukleV(alan string, p yazmac, adr yazmac, ofset, n int) []yazmac {
	r := make([]yazmac, n)
	for i := range r {
		r[i] = k.sabit(0)
	}
	op := "ld." + alan + vekEki(n) + ".f32"
	if p != "" {
		k.eger(p, op, vliste(r), bel(adr, ofset))
	} else {
		k.e(op, vliste(r), bel(adr, ofset))
	}
	return r
}

// yukleVIcine yukleV'nin var olan yazmaçlara yükleyen biçimi (koşul yanlışsa yazmaçlar
// değişmez).
func (k *yazici) yukleVIcine(alan string, p yazmac, adr yazmac, ofset int, r []yazmac) {
	op := "ld." + alan + vekEki(len(r)) + ".f32"
	if p != "" {
		k.eger(p, op, vliste(r), bel(adr, ofset))
		return
	}
	k.e(op, vliste(r), bel(adr, ofset))
}

// saklaV n float32'yi tek komutla yazar (p boş değilse koşullu).
func (k *yazici) saklaV(alan string, p yazmac, adr yazmac, ofset int, v []yazmac) {
	op := "st." + alan + vekEki(len(v)) + ".f32"
	if p != "" {
		k.eger(p, op, bel(adr, ofset), vliste(v))
		return
	}
	k.e(op, bel(adr, ofset), vliste(v))
}

// cpAsync genel bellekten paylaşımlı belleğe 16 baytlık eşzamansız kopya sıraya koyar
// (sm_80+). kaynakBoy 16'dan küçükse eksik baytlar sıfırla doldurulur (0: hiç okunmaz,
// sınır dışı döşeme elemanları için). Kopyanın bittiği ancak cpBekle'den sonra bilinir.
func (k *yazici) cpAsync(sm yazmac, smOfs int, kaynak yazmac, kaynakBoy yazmac) {
	if k.hedef < 80 {
		panic("gpu: cp.async sm_80 hedefi ister (iç hata)")
	}
	k.e("cp.async.cg.shared.global", bel(sm, smOfs), bel(kaynak, 0), 16, kaynakBoy)
}

// cpGrup sıraya konan eşzamansız kopyaları bir grupta kapatır.
func (k *yazici) cpGrup() { k.e("cp.async.commit_group") }

// cpBekle en yeni n grup dışındaki bütün kopya gruplarının bitmesini bekler (bu iş
// parçacığının kopyaları; başkalarına görünmesi için ardından engel gerekir).
func (k *yazici) cpBekle(n int) { k.e("cp.async.wait_group", n) }
