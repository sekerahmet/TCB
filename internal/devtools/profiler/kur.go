package profiler

import (
	"sort"
	"strings"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Anlık görüntünün (Goruntu) ham ölçülerden kurulması: süreç başından beri biriken
// değerler ve son pencerenin (en çok pencereSuresi) farkları.

// Listelerin uzunlukları: dosyaya yazılan görüntü ekrandakinden fazlasını taşır.
const (
	enCokIslem     = 30
	enCokSatir     = 30
	enCokFonksiyon = 20
)

// zamanBicimi görüntüdeki zaman damgalarıdır (günlükle aynı biçim).
const zamanBicimi = "2006-01-02T15:04:05.000Z07:00"

// kur a anından (once pencerenin başı) bir görüntü kurar; son ise program bitmiştir.
func (p *Profil) kur(a anlik, once *anlik, son bool) *Goruntu {
	gecen := a.zaman.Sub(p.baslangic)
	g := &Goruntu{
		Bicim: BicimAdi, Surum: BicimSurumu, Tur: "canlı", Program: p.sec.Program,
		Baslangic: p.baslangic.Format(zamanBicimi), Zaman: a.zaman.Format(zamanBicimi),
		SureSn: sn(gecen), Bitti: son,
	}
	if son {
		g.Tur = "rapor"
	}
	// Tensör sayaçları ve çöp toplayıcı süreç geneldir: profil başladığından beri olanlar.
	g.Adim = adimBilgisi(a.adim)
	g.Tensor = p.tensorBilgisi(a.sayaclar, p.taban.sayaclar, gecen, a.adim)
	g.GPU = p.gpuBilgisi(a.gpu)
	g.Bellek = p.bellekBilgisi(a)
	g.Cop = CopBilgisi{Sayi: a.bellek.NumGC - p.taban.bellek.NumGC,
		DuraklamaSn: float64(a.bellek.PauseTotalNs-p.taban.bellek.PauseTotalNs) / 1e9}
	if p.orn != nil {
		g.Ornekleme = p.orneklemeBilgisi(a.konumlar, nil, a.ornekSure, a.ornek)
	}
	if !son && once != nil && a.zaman.Sub(once.zaman) > 0 {
		g.Pencere = p.pencere(a, once)
	}
	return g
}

// adimBilgisi adım sınırlarından ms/adım hesaplar; adım yoksa nil.
func adimBilgisi(d adimDurumu) *AdimBilgisi {
	if !d.var_ {
		return nil
	}
	b := &AdimBilgisi{Sayi: d.son, Toplam: d.toplam}
	if n := d.son - d.ilk; n > 0 {
		s := d.sonZaman.Sub(d.ilkZaman)
		b.MsAdim = float64(s.Nanoseconds()) / 1e6 / float64(n)
		b.AdimSn = oran(float64(n), s.Seconds())
	}
	return b
}

// pencere son pencerenin farklarıdır.
func (p *Profil) pencere(a anlik, once *anlik) *PencereBilgisi {
	sure := a.zaman.Sub(once.zaman)
	w := &PencereBilgisi{SureSn: sn(sure)}
	adim := adimDurumu{}
	if a.adim.var_ {
		basAdim, basZaman := once.adim.son, once.adim.sonZaman
		if !once.adim.var_ {
			basAdim, basZaman = a.adim.ilk, a.adim.ilkZaman
		}
		if n := a.adim.son - basAdim; n > 0 {
			w.Adim = n
			s := a.adim.sonZaman.Sub(basZaman)
			w.MsAdim = float64(s.Nanoseconds()) / 1e6 / float64(n)
			w.AdimSn = oran(float64(n), s.Seconds())
			adim = adimDurumu{var_: true, ilk: basAdim, son: a.adim.son}
		}
	}
	t := p.tensorBilgisi(a.sayaclar, once.sayaclar, sure, adim)
	w.TensorPayi, w.Islemler = t.Pay, t.Islemler
	if p.orn != nil {
		o := p.orneklemeBilgisi(a.konumlar, once.konumlar, a.ornekSure-once.ornekSure, a.ornek-once.ornek)
		w.Satirlar = o.Satirlar
	}
	w.CPUTensorAyrilan = cpuBayti(a.sayaclar, once.sayaclar)
	gpuPenceresi(w, a.gpu, once.gpu, sure)
	w.CopSayi = a.bellek.NumGC - once.bellek.NumGC
	w.DuraklamaSn = float64(a.bellek.PauseTotalNs-once.bellek.PauseTotalNs) / 1e9
	return w
}

// sayacAnahtari bir sayaç kaydının kimliğidir.
type sayacAnahtari struct {
	c tensor.DeviceKind
	g bool
	i tensor.OpKind
}

// tensorBilgisi sayaçlardan (eksi varsa onların) işlem listesini kurar; sure payların
// paydasıdır, adim varsa adım başına süre eklenir.
func (p *Profil) tensorBilgisi(sayaclar, eksi []tensor.CounterRecord, sure time.Duration, adim adimDurumu) TensorBilgisi {
	onceki := map[sayacAnahtari]tensor.CounterRecord{}
	for _, k := range eksi {
		onceki[sayacAnahtari{k.Device, k.Backward, k.OpKind}] = k
	}
	var t TensorBilgisi
	adimSayisi := adim.son - adim.ilk
	for _, k := range sayaclar {
		o := onceki[sayacAnahtari{k.Device, k.Backward, k.OpKind}]
		k.Calls, k.Measured, k.Duration, k.Bytes = k.Calls-o.Calls, k.Measured-o.Measured, k.Duration-o.Duration, k.Bytes-o.Bytes
		if k.Calls == 0 && k.Bytes == 0 {
			continue
		}
		b := IslemBilgisi{
			Kod: k.OpKind.Kod(), Ad: tamAd(k.OpKind, k.Backward, k.Calls), Yon: "ileri", Cihaz: "cpu",
			Cagri: k.Calls, Olcum: k.Measured, SureSn: sn(k.Duration), Bayt: k.Bytes,
		}
		if k.Backward {
			b.Yon = "geri"
		}
		if k.Device != tensor.KindCPU {
			b.Cihaz = "gpu"
		}
		if adim.var_ && adimSayisi > 0 {
			ms := b.SureSn * 1000 / float64(adimSayisi)
			b.MsAdim = &ms
		}
		t.SureSn += b.SureSn
		t.Cagri += b.Cagri
		t.Bayt += b.Bayt
		t.Islemler = append(t.Islemler, b)
	}
	for i := range t.Islemler {
		t.Islemler[i].Pay = oran(t.Islemler[i].SureSn, t.SureSn)
	}
	sort.SliceStable(t.Islemler, func(i, j int) bool { return t.Islemler[i].SureSn > t.Islemler[j].SureSn })
	if len(t.Islemler) > enCokIslem {
		t.Islemler = t.Islemler[:enCokIslem]
	}
	t.Pay = oran(t.SureSn, sn(sure))
	return t
}

// orneklemeBilgisi konum örneklerinden (eksi varsa farkından) satır ve fonksiyon listesini
// kurar.
func (p *Profil) orneklemeBilgisi(konumlar, eksi []konumOrnegi, toplam time.Duration, ornek int64) *OrneklemeBilgisi {
	type anahtar struct {
		f string
		s int
	}
	onceki := map[anahtar]konumOrnegi{}
	for _, k := range eksi {
		onceki[anahtar{k.fonksiyon, k.satir}] = k
	}
	o := &OrneklemeBilgisi{AralikMs: float64(ornekAraligi) / float64(time.Millisecond), Ornek: ornek, SureSn: sn(toplam)}
	fonk := map[string]time.Duration{}
	var tensorde time.Duration
	for _, k := range konumlar {
		e := onceki[anahtar{k.fonksiyon, k.satir}]
		k.sure, k.tensor = k.sure-e.sure, k.tensor-e.tensor
		if k.sure <= 0 {
			continue
		}
		tensorde += k.tensor
		fonk[k.fonksiyon] += k.sure
		o.Satirlar = append(o.Satirlar, p.satirBilgisi(k, toplam))
	}
	o.TensorPayi = oran(sn(tensorde), sn(toplam))
	sort.SliceStable(o.Satirlar, func(i, j int) bool { return o.Satirlar[i].SureSn > o.Satirlar[j].SureSn })
	if len(o.Satirlar) > enCokSatir {
		o.Satirlar = o.Satirlar[:enCokSatir]
	}
	for ad, s := range fonk {
		o.Fonksiyonlar = append(o.Fonksiyonlar, FonksiyonBilgi{Ad: ad, SureSn: sn(s), Pay: oran(sn(s), sn(toplam))})
	}
	sort.Slice(o.Fonksiyonlar, func(i, j int) bool {
		if o.Fonksiyonlar[i].SureSn != o.Fonksiyonlar[j].SureSn {
			return o.Fonksiyonlar[i].SureSn > o.Fonksiyonlar[j].SureSn
		}
		return o.Fonksiyonlar[i].Ad < o.Fonksiyonlar[j].Ad
	})
	if len(o.Fonksiyonlar) > enCokFonksiyon {
		o.Fonksiyonlar = o.Fonksiyonlar[:enCokFonksiyon]
	}
	return o
}

// satirBilgisi bir konumun satırıdır; ana dosyadaysa metniyle.
func (p *Profil) satirBilgisi(k konumOrnegi, toplam time.Duration) SatirBilgisi {
	s := SatirBilgisi{
		Satir: k.satir, Fonksiyon: k.fonksiyon, SureSn: sn(k.sure),
		Pay: oran(sn(k.sure), sn(toplam)), TensorPayi: oran(sn(k.tensor), sn(k.sure)),
	}
	if k.fonksiyon == "" || p.anaFonk[k.fonksiyon] {
		s.Dosya = p.sec.Program
		if k.satir >= 1 && k.satir <= len(p.satirlar) {
			s.Metin = strings.TrimSpace(strings.TrimSuffix(p.satirlar[k.satir-1], "\r"))
		}
	}
	return s
}

// bellekBilgisi konak belleğidir; CPU tensör ayırması profil başından beri.
func (p *Profil) bellekBilgisi(a anlik) BellekBilgisi {
	return BellekBilgisi{GoYigini: a.bellek.HeapAlloc, GoSistem: a.bellek.Sys,
		CPUTensorAyrilan: cpuBayti(a.sayaclar, p.taban.sayaclar)}
}

// cpuBayti CPU tensör işlemlerinin (eksi'den bu yana) ayırdığı bayttır.
func cpuBayti(sayaclar, eksi []tensor.CounterRecord) int64 {
	var n int64
	for _, k := range sayaclar {
		if k.Device == tensor.KindCPU {
			n += k.Bytes
		}
	}
	for _, k := range eksi {
		if k.Device == tensor.KindCPU {
			n -= k.Bytes
		}
	}
	return n
}
