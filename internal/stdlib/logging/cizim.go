package logging

import (
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Çubuğun görünümü (Türkçe sayı yazımı):
//
//	adım 1200/5000 ▕████▌░░░░░░░░░░░▏ %24 · kayıp 2,31 · öğrenme_oranı 3e-4 · 85,2 adım/sn · kalan 14 dk
//	adım 5000/5000 · kayıp 1,84 · 61 dk 3 sn sürdü
//
// Toplamsız sayaçta çubuk ve kalan süre yerine geçen süre gösterilir.

// cubukGenisligi çubuğun hücre sayısıdır.
const cubukGenisligi = 16

// kismiHucre bir hücrenin sekizde biri kadar dolu hâlleridir.
var kismiHucre = []rune(" ▏▎▍▌▋▊▉")

// adimYazimi "adım 1200/5000" ya da "adım 1200". c.mu tutulur.
func (c *Ilerleme) adimYazimi() string {
	s := aAdim + " " + strconv.FormatInt(c.adim, 10)
	if c.toplam > 0 {
		s += "/" + strconv.FormatInt(c.toplam, 10)
	}
	return s
}

// cubuk dolu oranı (0–1) hücrelerle çizer.
func cubuk(oran float64) string {
	oran = math.Max(0, math.Min(1, oran))
	sekizde := int(oran * cubukGenisligi * 8)
	tam, kalan := sekizde/8, sekizde%8
	var b strings.Builder
	b.WriteString("▕")
	b.WriteString(strings.Repeat("█", tam))
	bos := cubukGenisligi - tam
	if kalan > 0 {
		b.WriteRune(kismiHucre[kalan])
		bos--
	}
	b.WriteString(strings.Repeat("░", bos))
	b.WriteString("▏")
	return b.String()
}

// olcuParcalari ölçülerin "ad değer" yazımlarıdır. c.mu tutulur.
func (c *Ilerleme) olcuParcalari() []string {
	out := make([]string, 0, len(c.olculer))
	for _, o := range c.olculer {
		out = append(out, o.ad+" "+olcuYazimi(o.deger))
	}
	return out
}

// cubukSatiri çubuğun şimdiki satırıdır. c.mu tutulur.
func (c *Ilerleme) cubukSatiri(simdi time.Time) string {
	bas := c.adimYazimi()
	if c.toplam > 0 {
		oran := float64(c.adim) / float64(c.toplam)
		bas += " " + cubuk(oran) + " %" + strconv.Itoa(int(math.Floor(oran*100)))
	}
	parcalar := append([]string{bas}, c.olcuParcalari()...)
	if c.hiz > 0 {
		parcalar = append(parcalar, hizYazimi(c.hiz))
	}
	switch {
	case c.toplam > 0 && c.hiz > 0:
		parcalar = append(parcalar, "kalan "+kalanYazimi(float64(c.toplam-c.adim)/c.hiz))
	case c.toplam == 0:
		parcalar = append(parcalar, "geçen "+sureYazimi(simdi.Sub(c.baslangic).Seconds()))
	}
	return strings.Join(parcalar, " · ")
}

// sonSatir çubuk bitince yazılan özet satırdır. c.mu tutulur.
func (c *Ilerleme) sonSatir(simdi time.Time) string {
	parcalar := append([]string{c.adimYazimi()}, c.olcuParcalari()...)
	parcalar = append(parcalar, sureYazimi(simdi.Sub(c.baslangic).Seconds())+" sürdü")
	return strings.Join(parcalar, " · ")
}

// hizYazimi "85,2 adım/sn"; saniyede birden yavaşsa "12,3 sn/adım".
func hizYazimi(h float64) string {
	if h < 1 {
		return sayiYazimi(1/h) + " sn/" + aAdim
	}
	return sayiYazimi(h) + " " + aAdim + "/sn"
}

// goster çubuğu sıklık sınırıyla gösterir: terminalde canlı satırı yeniden çizer, değilse
// arada bir düz satır yazar. Yorumlayıcı kilidi altında çağrılır.
func (c *Ilerleme) goster(simdi time.Time) {
	c.isDurumunuYaz(simdi, false)
	if c.terminal {
		if c.d.etkin != c || (!c.sonCizim.IsZero() && simdi.Sub(c.sonCizim) < terminalAraligi) {
			c.kirli.Store(true) // ekrandaki satır eski: bir çıktıdan sonra yeniden çizilirken tazelenir
			return
		}
		c.sonCizim = simdi
		c.kirli.Store(false)
		c.mu.Lock()
		s := c.cubukSatiri(simdi)
		c.mu.Unlock()
		c.m.CanliSatirGuncelle(func() {
			c.satir = s
			c.ciz()
		})
		return
	}
	if simdi.Sub(c.sonDuz) < duzSatirAraligi {
		return
	}
	c.sonDuz = simdi
	c.mu.Lock()
	s := c.cubukSatiri(simdi)
	c.mu.Unlock()
	c.m.CiktiYaz(c.w, s+"\n")
}

// sonuYaz bitiş satırını yazar. Terminalde görünen çubuğun satırı aynı yerde özet satırına
// dönüşür ve (varsa) önce açılmış, bitmemiş çubuk yeniden görünür.
func (c *Ilerleme) sonuYaz(son string) {
	if !c.terminal || c.d.etkin != c {
		c.m.CiktiYaz(c.w, son+"\n")
		return
	}
	d := c.d
	d.etkin = c.onceki
	for d.etkin != nil && d.etkin.bittiMi() {
		d.etkin = d.etkin.onceki
	}
	c.m.CanliSatirAyarla(nil)
	c.m.CanliSatirGuncelle(func() {
		c.satir = son
		c.ciz()
		_, _ = io.WriteString(c.w, "\n")
		d.ekrandaki = 0
	})
	if e := d.etkin; e != nil {
		c.m.CanliSatirAyarla(e)
		c.m.CanliSatirGuncelle(e.ciz)
	}
}

// bittiMi çubuk bitti mi?
func (c *Ilerleme) bittiMi() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.durdu
}

// Sil canlı satırı siler (yurut.CanliSatir); yorumlayıcı çıktı kilidi altında çağırır.
func (c *Ilerleme) Sil() {
	if n := c.d.ekrandaki; n > 0 {
		_, _ = io.WriteString(c.w, "\r"+strings.Repeat(" ", n)+"\r")
		c.d.ekrandaki = 0
	}
}

// Ciz canlı satırı yeniden çizer (yurut.CanliSatir); yorumlayıcı çıktı kilidi altında
// çağırır (ör. yazdır'dan sonra). Son güncelleme sıklık sınırı yüzünden çizilmediyse satır
// önce tazelenir.
func (c *Ilerleme) Ciz() {
	if c.kirli.Load() {
		c.mu.Lock()
		c.satir = c.cubukSatiri(saat())
		c.mu.Unlock()
		c.kirli.Store(false)
	}
	c.ciz()
}

// ciz satırı terminal genişliğine kırparak satır başından yazar; önceki satır daha uzunsa
// artanı boşlukla siler. Çıktı kilidi tutulur.
func (c *Ilerleme) ciz() {
	if c.satir == "" {
		return
	}
	s := c.m.Maskele(c.satir)
	if r := []rune(s); c.genislik > 1 && len(r) > c.genislik-1 {
		s = string(r[:c.genislik-2]) + "…"
	}
	n := utf8.RuneCountInString(s)
	ek := ""
	if c.d.ekrandaki > n {
		ek = strings.Repeat(" ", c.d.ekrandaki-n)
	}
	_, _ = io.WriteString(c.w, "\r"+s+ek)
	c.d.ekrandaki = n
}
