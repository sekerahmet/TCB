package profiler

import (
	"fmt"
	"strings"
)

// Görüntünün metin biçimleri: canlı ekran (terminal), düz özet satırı (terminal değilken
// 10 sn'de bir) ve son rapor (tr profil, tr izle'nin sonu).

// Ekran canlı ekranın görüntü dışındaki parçalarıdır.
type Ekran struct {
	Genislik     int      // terminal sütunu
	CiktiDosyasi string   // programın çıktısının tamamı (boşsa gösterilmez)
	SonCikti     []string // programın son çıktı satırları
}

// EkranSatirlari canlı ekranın satırlarıdır; her satır genişliğe kırpılır.
func EkranSatirlari(g *Goruntu, e Ekran) []string {
	w := e.Genislik
	if w < 40 {
		w = 80
	}
	var s []string
	ekle := func(f string, a ...any) { s = append(s, kirp(fmt.Sprintf(f, a...), w)) }
	ekle("⏱ izleniyor: %s · %s · durdurmak için Ctrl+C", g.Program, sureMetni(g.SureSn))
	p := g.Pencere
	if p == nil {
		p = &PencereBilgisi{}
	}
	switch {
	case g.Adim == nil:
		ekle("Adım       %s", adimYok())
	default:
		a := g.Adim
		ad := tam(a.Sayi)
		if a.Toplam > 0 {
			ad += "/" + tam(a.Toplam)
		}
		ms, hiz := a.MsAdim, a.AdimSn
		if p.Adim > 0 {
			ms, hiz = p.MsAdim, p.AdimSn
		}
		kalan := ""
		if a.Toplam > a.Sayi && hiz > 0 {
			kalan = " · kalan ~" + sureMetni(float64(a.Toplam-a.Sayi)/hiz)
		}
		ekle("Adım       %s · %s ms/adım · %s adım/sn%s", ad, sayi(ms), sayi(hiz), kalan)
	}
	ekle("Zaman      tensör işlemleri %s · geri kalanı (TR kodu, bekleme) %s   (son %s)",
		yuzde(p.TensorPayi), yuzde(max(0, 1-p.TensorPayi)), sureMetni(p.SureSn))
	s = append(s, "")
	s = append(s, islemTablosu(p.Islemler, p.Adim, p.SureSn, w, 10)...)
	if g.GPU.Kullanildi {
		ekle("GPU        %s", gpuPencereMetni(g.GPU, p))
	}
	if g.Ornekleme != nil {
		s = append(s, "")
		s = append(s, satirTablosu(p.Satirlar, w, 5)...)
	}
	s = append(s, "")
	ekle("Bellek     Go yığını %s · CPU tensör ayırması %s%s", bayt(int64(g.Bellek.GoYigini)),
		ayirmaHizi(p.CPUTensorAyrilan, p.Adim, p.SureSn), gpuBellegi(g.GPU))
	ekle("Çöp top.   %d kez, toplam duraklama %s (son %s: %d kez, %s)", g.Cop.Sayi, sureMetni(g.Cop.DuraklamaSn),
		sureMetni(p.SureSn), p.CopSayi, sureMetni(p.DuraklamaSn))
	if e.CiktiDosyasi != "" || len(e.SonCikti) > 0 {
		s = append(s, "")
		ekle("Programın çıktısı (tamamı: %s)", e.CiktiDosyasi)
		for _, c := range e.SonCikti {
			ekle("  │ %s", c)
		}
	}
	return s
}

// islemTablosu en pahalı n işlemin tablosudur (pencere): adım varsa adım başına.
func islemTablosu(is []IslemBilgisi, adim int64, sure float64, w, n int) []string {
	if len(is) == 0 {
		return []string{tensorIslemleri() + ": yok"}
	}
	adW := max(20, min(36, w-40))
	baslik := tensorIslemleri() + " (en pahalı)"
	birim, cagriBirim := "ms", "çağrı/sn"
	if adim > 0 {
		birim, cagriBirim = "ms/adım", "çağrı/adım"
	}
	s := []string{kirp(sola(baslik, adW+2)+saga("pay", 6)+saga(birim, 10)+saga(cagriBirim, 12)+saga("cihaz", 7), w)}
	for i, b := range is {
		if i == n {
			break
		}
		ms := b.SureSn * 1000
		cagri := float64(b.Cagri) / max(sure, 1e-9)
		if adim > 0 {
			ms /= float64(adim)
			cagri = float64(b.Cagri) / float64(adim)
		}
		s = append(s, kirp("  "+sola(b.Ad, adW)+saga(yuzde(b.Pay), 6)+saga(msMetni(ms), 10)+saga(sayi(cagri), 12)+saga(b.Cihaz, 7), w))
	}
	return s
}

// satirTablosu en sıcak n satırdır.
func satirTablosu(ss []SatirBilgisi, w, n int) []string {
	if len(ss) == 0 {
		return []string{"Sıcak satırlar: henüz örnek yok"}
	}
	s := []string{kirp(sola("Sıcak satırlar (öz süre)", 34)+saga("pay", 6)+saga("tensörde", 9), w)}
	for i, b := range ss {
		if i == n {
			break
		}
		konum := satirKonumu(b)
		s = append(s, kirp("  "+sola(konum, 32)+saga(yuzde(b.Pay), 6)+saga(yuzde(b.TensorPayi), 9)+"  "+satirMetni(b), w))
	}
	return s
}

// gpuPencereMetni canlı ekranın GPU satırıdır: çekirdeklerin pencerede cihazda geçen
// süresinin payı (GPU'nun meşgul olduğu zaman), adım başına süresi ve en pahalı aileler.
func gpuPencereMetni(g GPUBilgisi, p *PencereBilgisi) string {
	if !g.Olcum {
		return g.Not
	}
	if p.GPUSureSn <= 0 {
		return "çekirdekler: bu pencerede ölçüm yok"
	}
	m := "çekirdekler " + yuzde(p.GPUPayi) + " meşgul"
	if p.Adim > 0 {
		m += " · " + msMetni(p.GPUSureSn*1000/float64(p.Adim)) + " ms/adım"
	}
	return m + " · " + aileOzeti(p.GPUAileler, 3)
}

// aileOzeti en pahalı n çekirdek ailesinin kısa yazımıdır: "carpim %41 · dikkat %20".
func aileOzeti(aileler []CekirdekBilgisi, n int) string {
	var p []string
	for i, c := range aileler {
		if i == n {
			break
		}
		p = append(p, c.Ad+" "+yuzde(c.Pay))
	}
	return strings.Join(p, " · ")
}

// ayirmaHizi ayrılan baytı adım başına (adım varsa) ya da saniye başına yazar.
func ayirmaHizi(n, adim int64, sure float64) string {
	if adim > 0 {
		return bayt(n/adim) + "/adım"
	}
	if sure > 0 {
		return bayt(int64(float64(n)/sure)) + "/sn"
	}
	return bayt(n)
}

// gpuBellegi GPU belleğinin kısa yazımıdır (GPU kullanılmadıysa boş).
func gpuBellegi(g GPUBilgisi) string {
	var p []string
	for _, b := range g.Bellek {
		ad := "GPU"
		if len(g.Bellek) > 1 {
			ad = fmt.Sprintf("GPU %d", b.Cihaz)
		}
		p = append(p, fmt.Sprintf("%s canlı %s (ayrılmış %s, tepe %s)", ad, bayt(int64(b.Canli)),
			bayt(int64(b.Ayrilmis)), bayt(int64(b.TepeCanli))))
	}
	if len(p) == 0 {
		return ""
	}
	return " · " + strings.Join(p, " · ")
}

// OzetSatiri terminal değilken (10 sn'de bir) yazılan tek satırlık özettir.
func OzetSatiri(g *Goruntu) string {
	var b strings.Builder
	fmt.Fprintf(&b, "⏱ %s", sureMetni(g.SureSn))
	if g.Adim != nil {
		fmt.Fprintf(&b, " · adım %s", tam(g.Adim.Sayi))
		if g.Adim.Toplam > 0 {
			fmt.Fprintf(&b, "/%s", tam(g.Adim.Toplam))
		}
		ms := g.Adim.MsAdim
		if g.Pencere != nil && g.Pencere.Adim > 0 {
			ms = g.Pencere.MsAdim
		}
		fmt.Fprintf(&b, " · %s ms/adım", sayi(ms))
	}
	pay, is := g.Tensor.Pay, g.Tensor.Islemler
	if g.Pencere != nil {
		pay, is = g.Pencere.TensorPayi, g.Pencere.Islemler
	}
	fmt.Fprintf(&b, " · tensör %s", yuzde(pay))
	var p []string
	for i, x := range is {
		if i == 3 {
			break
		}
		p = append(p, x.Ad+" "+yuzde(x.Pay))
	}
	if len(p) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(p, ", "))
	}
	if g.Pencere != nil && len(g.Pencere.Satirlar) > 0 {
		fmt.Fprintf(&b, " · satır %s %s", satirKonumu(g.Pencere.Satirlar[0]), yuzde(g.Pencere.Satirlar[0].Pay))
	}
	fmt.Fprintf(&b, " · Go yığını %s · çöp toplama %d (%s)", bayt(int64(g.Bellek.GoYigini)), g.Cop.Sayi, sureMetni(g.Cop.DuraklamaSn))
	return b.String()
}
