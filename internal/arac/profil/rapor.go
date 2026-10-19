package profil

import (
	"fmt"
	"strings"
)

// RaporMetni son raporun metin tablosudur (tr profil, tr izle'nin sonu).
func RaporMetni(g *Goruntu) string {
	var b strings.Builder
	yaz := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	cikis := ""
	if g.CikisKodu != nil {
		cikis = fmt.Sprintf(" · çıkış kodu %d", *g.CikisKodu)
	}
	yaz("⏱ Profil: %s — %s%s", g.Program, sureMetni(g.SureSn), cikis)
	if g.Adim == nil {
		yaz("Adımlar: %s", adimYok())
	} else {
		yaz("Adımlar: %s adım · %s ms/adım · %s adım/sn", tam(g.Adim.Sayi), sayi(g.Adim.MsAdim), sayi(g.Adim.AdimSn))
	}
	t := g.Tensor
	yaz("Zaman: tensör işlemleri %s (%s, %s çağrı, %s ayrıldı) · geri kalanı %s (TR kodu, bekleme, çöp toplama)",
		yuzde(t.Pay), sureMetni(t.SureSn), tam(t.Cagri), bayt(t.Bayt), yuzde(max(0, 1-t.Pay)))
	if o := g.Ornekleme; o != nil {
		yaz("Örnekleme: %s aralıkla %s örnek; tensör işlemindeki payı %s", sureMetni(o.AralikMs/1000), tam(o.Ornek), yuzde(o.TensorPayi))
	}
	b.WriteString("\n")
	raporIslemleri(&b, t)
	raporGPU(&b, g)
	if o := g.Ornekleme; o != nil {
		b.WriteString("\n")
		raporSatirlari(&b, o)
	}
	b.WriteString("\n")
	yaz("Bellek: Go yığını %s (Go'nun sistemden aldığı %s) · CPU tensör ayırması %s toplam%s",
		bayt(int64(g.Bellek.GoYigini)), bayt(int64(g.Bellek.GoSistem)), bayt(g.Bellek.CPUTensorAyrilan), gpuBellegi(g.GPU))
	yaz("Çöp toplama: %d kez, toplam duraklama %s", g.Cop.Sayi, sureMetni(g.Cop.DuraklamaSn))
	return b.String()
}

// raporIslemleri tensör işlemleri tablosudur (en pahalı 15).
func raporIslemleri(b *strings.Builder, t TensorBilgisi) {
	if len(t.Islemler) == 0 {
		b.WriteString(tensorIslemleri() + ": yok\n")
		return
	}
	adimVar := t.Islemler[0].MsAdim != nil
	baslik := sola(tensorIslemleri(), 30) + saga("süre", 10) + saga("pay", 6) + saga("çağrı", 12)
	if adimVar {
		baslik += saga("ms/adım", 10)
	}
	b.WriteString(baslik + saga("ayrılan", 10) + saga("cihaz", 7) + "\n")
	for i, x := range t.Islemler {
		if i == 15 {
			fmt.Fprintf(b, "  … %d işlem daha\n", len(t.Islemler)-i)
			break
		}
		satir := "  " + sola(x.Ad, 28) + saga(sureMetni(x.SureSn), 10) + saga(yuzde(x.Pay), 6) + saga(tam(x.Cagri), 12)
		if adimVar && x.MsAdim != nil {
			satir += saga(msMetni(*x.MsAdim), 10)
		}
		b.WriteString(satir + saga(bayt(x.Bayt), 10) + saga(x.Cihaz, 7) + "\n")
	}
}

// raporSatirlari en sıcak satırlar ve fonksiyonlardır.
func raporSatirlari(b *strings.Builder, o *OrneklemeBilgisi) {
	b.WriteString("En çok zaman alan satırlar (öz süre; tensör: satırın tensör işleminde geçen payı):\n")
	for i, s := range o.Satirlar {
		if i == 10 {
			break
		}
		fmt.Fprintf(b, "  %s  %s  %s  %s\n", saga(yuzde(s.Pay), 5), sola(satirKonumu(s), 30), saga("tensör "+yuzde(s.TensorPayi), 11), kirp(satirMetni(s), 70))
	}
	b.WriteString("Fonksiyonlar (öz süre):\n")
	for i, f := range o.Fonksiyonlar {
		if i == 8 {
			break
		}
		fmt.Fprintf(b, "  %s  %s  %s\n", saga(yuzde(f.Pay), 5), sola(fonksiyonAdi(f.Ad), 30), sureMetni(f.SureSn))
	}
}

// raporGPU GPU çekirdeklerinin cihazda geçen süresidir (aileler) ve ölçümü profil açtıysa
// GPU arka ucunun kendi raporu.
func raporGPU(b *strings.Builder, g *Goruntu) {
	gp := g.GPU
	if !gp.Kullanildi {
		return
	}
	if !gp.Olcum || len(gp.Aileler) == 0 {
		fmt.Fprintf(b, "GPU çekirdekleri: %s\n", gp.Not)
		return
	}
	fmt.Fprintf(b, "GPU çekirdekleri: cihazda %s (program süresine oranı %s)", sureMetni(gp.SureSn), yuzde(oran(gp.SureSn, g.SureSn)))
	if g.Adim != nil && g.Adim.Sayi > 0 {
		fmt.Fprintf(b, " · %s ms/adım", msMetni(gp.SureSn*1000/float64(g.Adim.Sayi)))
	}
	fmt.Fprintf(b, " · %s\n", aileOzeti(gp.Aileler, 5))
	if gp.ozet != "" {
		b.WriteString("\n" + gp.ozet + "\n")
	}
}
