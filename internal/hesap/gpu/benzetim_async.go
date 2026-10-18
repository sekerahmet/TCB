package gpu

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ============================================================================
// PTX benzetimi: vektörel erişimler ve cp.async
// ============================================================================
//
// Vektörel ld/st (.v2/.v4) tek erişimdir: adres toplam boya (ör. 16 bayt) hizalı olmalı.
//
// cp.async donanımdaki gibi eşzamansız ele alınır: kopya iş parçacığının bekleyen
// listesine konur ve ancak o iş parçacığı cp.async.wait_group ile beklediğinde yapılır
// (kaynak o anda okunur, hedef o anda yazılır ve yarış dedektörüne yazma olarak
// bildirilir). Böylece beklemeden ya da beklemeden sonraki engel olmadan okunan kopya
// hedefi benzetimde de eski değeri verir ve sınamada yanlış sonuç olarak görünür. Bekleyen
// kopyası kalan bir iş parçacığının bitmesi hatadır.

// bzCpIs bekleyen bir eşzamansız kopya.
type bzCpIs struct {
	grup     int32
	smAdr    int
	kaynak   uint64
	boy, oku int
}

// vektor vektör işlenenini (yer tutucu "@V") komutun yazmaç ya da terim listesine çözer.
func (a *bzAyristirici) vektor(k *bzKomut, t, liste []string) error {
	if liste == nil {
		if k.vek != 0 {
			return fmt.Errorf("vektörel komutta vektör işleneni yok")
		}
		return nil
	}
	if int(k.vek) != len(liste) || (k.islem != bzLd && k.islem != bzSt) {
		return fmt.Errorf("vektör işleneni (%d eleman) komutla uyuşmuyor", len(liste))
	}
	if k.tur != bzF32 && k.tur != bzU32 {
		return fmt.Errorf("vektörel erişim yalnızca 32 bitlik türlerle")
	}
	for i, s := range liste {
		if k.islem == bzLd {
			y, ok := a.yazmac[s]
			if !ok {
				return fmt.Errorf("tanımsız hedef yazmaç %s", s)
			}
			k.vy[i] = y
			continue
		}
		v, err := a.terim(s, k.tur)
		if err != nil {
			return err
		}
		k.vo[i] = v
	}
	yer := 0
	if k.islem == bzSt {
		yer = 1
	}
	if len(t) != 2 || t[yer] != "@V" {
		return fmt.Errorf("vektör işleneninin yeri yanlış")
	}
	return nil
}

// cpTerimleri cp.async.cg.shared.global [hedef], [kaynak], boy[, kaynak_boyu].
func (a *bzAyristirici) cpTerimleri(k *bzKomut, t []string) error {
	if len(t) != 3 && len(t) != 4 {
		return fmt.Errorf("cp.async 3 ya da 4 işlenen ister")
	}
	k.alan = alanOrtak
	if err := a.bellek(k, t[0]); err != nil {
		return err
	}
	kaynak := strings.TrimSuffix(strings.TrimPrefix(t[1], "["), "]")
	if kaynak == t[1] {
		return fmt.Errorf("cp.async kaynağı bellek işleneni olmalı")
	}
	if i := strings.IndexByte(kaynak, '+'); i >= 0 {
		var n int64
		if _, err := fmt.Sscan(kaynak[i+1:], &n); err != nil {
			return err
		}
		k.ofset2, kaynak = n, kaynak[:i]
	}
	var err error
	if k.taban2, err = a.terim(kaynak, bzU64); err != nil {
		return err
	}
	if k.o[0], err = a.terim(t[2], bzU32); err != nil {
		return err
	}
	if k.o[0].cesit != tmSabit || (k.o[0].s != 4 && k.o[0].s != 8 && k.o[0].s != 16) {
		return fmt.Errorf("cp.async boyu 4, 8 ya da 16 sabiti olmalı")
	}
	k.o[1] = k.o[0]
	if len(t) == 4 {
		k.o[1], err = a.terim(t[3], bzU32)
	}
	return err
}

// vekYukle vektörel ld.
func (b *bzBlok) vekYukle(t *bzIs, k *bzKomut) error {
	n := int(k.vek)
	d, err := b.adresDilim(t, k, 4*n, false)
	if err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		t.y[k.vy[i]] = uint64(binary.LittleEndian.Uint32(d[4*i:]))
	}
	return nil
}

// vekSakla vektörel st.
func (b *bzBlok) vekSakla(t *bzIs, k *bzKomut) error {
	if k.alan == alanParam {
		return fmt.Errorf("parametre alanına yazılamaz")
	}
	n := int(k.vek)
	d, err := b.adresDilim(t, k, 4*n, true)
	if err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(d[4*i:], uint32(b.oku(t, &k.vo[i])))
	}
	return nil
}

// cpEkle eşzamansız kopyayı iş parçacığının bekleyen listesine koyar.
func (b *bzBlok) cpEkle(t *bzIs, k *bzKomut) error {
	boy := int(k.o[0].s)
	oku := int(uint32(b.oku(t, &k.o[1])))
	if oku > boy {
		return fmt.Errorf("cp.async kaynak boyu %d kopya boyundan (%d) büyük", oku, boy)
	}
	sm := int64(uint32(b.oku(t, &k.taban))) + k.ofset
	if sm < 0 || int(sm)+boy > len(b.ortak) || sm%int64(boy) != 0 {
		return fmt.Errorf("cp.async hedefi geçersiz ya da hizasız: %d", sm)
	}
	kaynak := b.oku(t, &k.taban2) + uint64(k.ofset2)
	if oku > 0 && kaynak%uint64(boy) != 0 {
		return fmt.Errorf("cp.async kaynağı hizasız: 0x%x", kaynak)
	}
	t.cp = append(t.cp, bzCpIs{grup: t.cpGrup, smAdr: int(sm), kaynak: kaynak, boy: boy, oku: oku})
	return nil
}

// cpBekle en yeni n grup dışındaki kopyaları yapar.
func (b *bzBlok) cpBekle(t *bzIs, n int32) error {
	kalan := t.cp[:0]
	for _, c := range t.cp {
		if c.grup >= t.cpGrup-n {
			kalan = append(kalan, c)
			continue
		}
		if err := b.yaris(t, c.smAdr, c.boy, true); err != nil {
			return err
		}
		hedef := b.ortak[c.smAdr : c.smAdr+c.boy]
		clear(hedef)
		if c.oku > 0 {
			d, err := b.bel.dilim(c.kaynak, c.oku)
			if err != nil {
				return fmt.Errorf("cp.async kaynağı: %v", err)
			}
			copy(hedef, d)
		}
	}
	t.cp = kalan
	return nil
}

// cpYurut cp.async ailesinin komutlarını yürütür.
func (b *bzBlok) cpYurut(t *bzIs, k *bzKomut) error {
	switch k.islem {
	case bzCp:
		return b.cpEkle(t, k)
	case bzCpGrup:
		t.cpGrup++
		return nil
	}
	return b.cpBekle(t, int32(k.o[0].s))
}
