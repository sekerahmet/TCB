package gpu

import "strings"

// PTX yazıcısı: vektörel bellek erişimleri (ld/st .v2/.v4) ve eşzamansız paylaşımlı bellek
// kopyası (cp.async, sm_80+). Vektörel erişimin adresi erişim boyuna (4 float32 için 16
// bayt) hizalı olmalıdır; benzetim hizayı denetler.

// vliste yazmaçları PTX vektör işleneni olarak yazar: {%f1, %f2, %f3, %f4}.
func vlist(r []reg) string {
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
func vecSuffix(n int) string {
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
func (k *emitter) loadV(space string, p reg, addr reg, offset, n int) []reg {
	r := make([]reg, n)
	for i := range r {
		r[i] = k.constant(0)
	}
	op := "ld." + space + vecSuffix(n) + ".f32"
	if p != "" {
		k.ifCase(p, op, vlist(r), mem(addr, offset))
	} else {
		k.e(op, vlist(r), mem(addr, offset))
	}
	return r
}

// yukleVIcine yukleV'nin var olan yazmaçlara yükleyen biçimi (koşul yanlışsa yazmaçlar
// değişmez).
func (k *emitter) loadVInto(space string, p reg, addr reg, offset int, r []reg) {
	op := "ld." + space + vecSuffix(len(r)) + ".f32"
	if p != "" {
		k.ifCase(p, op, vlist(r), mem(addr, offset))
		return
	}
	k.e(op, vlist(r), mem(addr, offset))
}

// saklaV n float32'yi tek komutla yazar (p boş değilse koşullu).
func (k *emitter) storeV(space string, p reg, addr reg, offset int, v []reg) {
	op := "st." + space + vecSuffix(len(v)) + ".f32"
	if p != "" {
		k.ifCase(p, op, mem(addr, offset), vlist(v))
		return
	}
	k.e(op, mem(addr, offset), vlist(v))
}

// cpAsync genel bellekten paylaşımlı belleğe 16 baytlık eşzamansız kopya sıraya koyar
// (sm_80+). kaynakBoy 16'dan küçükse eksik baytlar sıfırla doldurulur (0: hiç okunmaz,
// sınır dışı döşeme elemanları için). Kopyanın bittiği ancak cpBekle'den sonra bilinir.
func (k *emitter) cpAsync(sm reg, smOfs int, source reg, sourceSize reg) {
	if k.target < 80 {
		panic("gpu: cp.async sm_80 hedefi ister (iç hata)")
	}
	k.e("cp.async.cg.shared.global", mem(sm, smOfs), mem(source, 0), 16, sourceSize)
}

// cpGrup sıraya konan eşzamansız kopyaları bir grupta kapatır.
func (k *emitter) cpGroup() { k.e("cp.async.commit_group") }

// cpBekle en yeni n grup dışındaki bütün kopya gruplarının bitmesini bekler (bu iş
// parçacığının kopyaları; başkalarına görünmesi için ardından engel gerekir).
func (k *emitter) cpWait(n int) { k.e("cp.async.wait_group", n) }
