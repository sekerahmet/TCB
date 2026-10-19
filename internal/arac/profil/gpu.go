package profil

import (
	"sort"
	"strings"
	"time"

	"tr_dili/internal/hesap/gpu"
)

// GPU bağlantısı: çekirdek süreleri ve GPU belleği GPU arka ucunun kendi profilinden okunur
// (internal/hesap/gpu, profil_arayuz.go). Tensör sayaçları GPU işlemlerinde yalnızca
// konakta geçen başlatma süresini görür; cihazda geçen süre buradan gelir.
//
// Çekirdek ölçümü (her başlatmanın iki zamanlama olayı arasına alınması) tr izle ve
// tr profil altında açılır, arka plan işinin profilinde kapalıdır (her zaman açık
// sayaçlar dışında yük eklenmez; GPU belleği yine görünür). Okumalar arka ucu beklemez.
// Program bitmeden (motorlar kapanmadan) KapanmadanOnce çağrılırsa son ölçümler cihaz
// eşzamanlanarak çözülür ve arka ucun kendi raporu metin raporuna eklenir.

// gpuAnlik bir anın GPU ölçüleridir.
type gpuAnlik struct {
	cekirdek map[string]gpu.CekirdekOlcumu
	bellek   []gpu.BellekOlcumu
}

// gpuAnlikAl ölçüleri okur (GPU kurulmamışsa ikisi de boş; sürücü yüklenmez).
func gpuAnlikAl() gpuAnlik {
	a := gpuAnlik{bellek: gpu.BellekOlcumleri()}
	if len(a.bellek) == 0 {
		return a
	}
	a.cekirdek = map[string]gpu.CekirdekOlcumu{}
	for _, c := range gpu.CekirdekOlcumleri() {
		a.cekirdek[c.Ad] = c
	}
	return a
}

// gpuOlcumunuAc çekirdek ölçümünü açar; önceki durumu verir (Bitir geri yükler).
func gpuOlcumunuAc() (onceki bool) {
	onceki = gpu.ProfilAcik()
	gpu.ProfilAc(true)
	return onceki
}

// gpuFarki a ile once arasındaki çekirdek ölçümlerini (ad başına) ve aile toplamlarını
// süreye göre sıralı verir; toplam cihazda geçen süredir.
func gpuFarki(a, once map[string]gpu.CekirdekOlcumu) (cekirdekler, aileler []CekirdekBilgisi, toplam time.Duration) {
	aile := map[string]*CekirdekBilgisi{}
	type ham struct {
		ad    string
		cagri int64
		sure  time.Duration
	}
	var hamlar []ham
	for ad, c := range a {
		o := once[ad]
		n, d := c.Cagri-o.Cagri, c.GPUSuresi-o.GPUSuresi
		if n <= 0 && d <= 0 {
			continue
		}
		toplam += d
		hamlar = append(hamlar, ham{ad, n, d})
		ai := c.Aile
		if ai == "" {
			ai = ad
		}
		x := aile[ai]
		if x == nil {
			x = &CekirdekBilgisi{Ad: ai}
			aile[ai] = x
		}
		x.Cagri += n
		x.SureSn += sn(d)
	}
	for _, h := range hamlar {
		cekirdekler = append(cekirdekler, CekirdekBilgisi{Ad: h.ad, Cagri: h.cagri, SureSn: sn(h.sure), Pay: oran(sn(h.sure), sn(toplam))})
	}
	for _, x := range aile {
		x.Pay = oran(x.SureSn, sn(toplam))
		aileler = append(aileler, *x)
	}
	sirala := func(l []CekirdekBilgisi) {
		sort.Slice(l, func(i, j int) bool {
			if l[i].SureSn != l[j].SureSn {
				return l[i].SureSn > l[j].SureSn
			}
			return l[i].Ad < l[j].Ad
		})
	}
	sirala(cekirdekler)
	sirala(aileler)
	return cekirdekler, aileler, toplam
}

// gpuBilgisi profil başından (taban) a anına kadarki GPU ölçümleridir.
func (p *Profil) gpuBilgisi(a gpuAnlik) GPUBilgisi {
	if len(a.bellek) == 0 {
		return GPUBilgisi{Not: "GPU kullanılmadı"}
	}
	p.mu.Lock()
	olculdu, ozet := p.gpuOlculdu, p.gpuOzet
	p.mu.Unlock()
	g := GPUBilgisi{Kullanildi: true, Olcum: olculdu || gpu.ProfilAcik(), ozet: ozet}
	var toplam time.Duration
	g.Cekirdekler, g.Aileler, toplam = gpuFarki(a.cekirdek, p.taban.gpu.cekirdek)
	g.SureSn = sn(toplam)
	g.Cekirdekler = g.Cekirdekler[:min(len(g.Cekirdekler), enCokCekirdek)]
	switch {
	case !g.Olcum:
		g.Not = "çekirdek süreleri ölçülmüyor (tr izle ve tr profil ölçer; ya da TR_GPU_PROFIL=1)"
	case len(g.Cekirdekler) == 0:
		g.Not = "henüz çekirdek ölçümü yok"
	}
	for _, b := range a.bellek {
		g.Bellek = append(g.Bellek, GPUBellegi{
			Cihaz: b.Cihaz, Canli: b.Canli, Ayrilmis: b.Ayrilmis, Havuzda: b.Havuzda,
			TepeCanli: b.TepeCanli, TepeAyrilmis: b.TepeAyrilmis, CopToplama: b.CopToplama,
		})
	}
	return g
}

// enCokCekirdek dosyaya yazılan çekirdek sayısıdır (aileler hep tam).
const enCokCekirdek = 30

// KapanmadanOnce program bittiğinde, motorlar (GPU arka ucu) kapanmadan önce çağrılır.
// Çekirdek ölçümünü profil açtıysa: cihaz eşzamanlanır, bekleyen ölçümler çözülür, arka
// ucun kendi raporu son rapora eklenmek üzere alınır ve ölçüm eski durumuna döner (arka
// uç kapanırken raporu ikinci kez yazmasın). Birden çok kez çağrılabilir.
func (p *Profil) KapanmadanOnce() {
	p.mu.Lock()
	ac := p.gpuActi
	p.gpuActi = false
	p.mu.Unlock()
	if !ac {
		return
	}
	if len(gpu.BellekOlcumleri()) > 0 {
		var b strings.Builder
		gpu.ProfilOzetiYaz(&b)
		p.mu.Lock()
		p.gpuOzet = strings.TrimRight(b.String(), "\n")
		p.mu.Unlock()
	}
	gpu.ProfilAc(p.gpuOnceki)
}

// gpuPenceresi pencerenin GPU ölçüleridir.
func gpuPenceresi(w *PencereBilgisi, a, once gpuAnlik, sure time.Duration) {
	if len(a.bellek) == 0 {
		return
	}
	_, aileler, toplam := gpuFarki(a.cekirdek, once.cekirdek)
	w.GPUSureSn, w.GPUPayi, w.GPUAileler = sn(toplam), oran(sn(toplam), sn(sure)), aileler
}
