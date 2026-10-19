package profil

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// ornekGoruntu canlı ekranın sınanması için elle kurulmuş bir görüntüdür: 50 adımlık
// pencerede 2 sn, GPU'nun 1,5 sn'si meşgul.
func ornekGoruntu() *Goruntu {
	ms := 20.0
	return &Goruntu{
		Program: "egit.tr", SureSn: 65, Adim: &AdimBilgisi{Sayi: 300, Toplam: 1000, MsAdim: 41, AdimSn: 24},
		Tensor:    TensorBilgisi{Pay: 0.8},
		GPU:       GPUBilgisi{Kullanildi: true, Olcum: true, Bellek: []GPUBellegi{{Canli: 3 << 30, Ayrilmis: 4 << 30, TepeCanli: 3 << 30}}},
		Ornekleme: &OrneklemeBilgisi{}, Bellek: BellekBilgisi{GoYigini: 200 << 20},
		Pencere: &PencereBilgisi{
			SureSn: 2, Adim: 50, MsAdim: 40, AdimSn: 25, TensorPayi: 0.75, CPUTensorAyrilan: 50 << 20,
			Islemler:  []IslemBilgisi{{Ad: "matris çarpımı (@)", Pay: 0.5, SureSn: 1, Cagri: 100, Cihaz: "gpu", MsAdim: &ms}},
			Satirlar:  []SatirBilgisi{{Dosya: "egit.tr", Satir: 12, Pay: 0.6, TensorPayi: 0.9, Metin: "geri_yayılım(kayıp)"}},
			GPUSureSn: 1.5, GPUPayi: 0.75, GPUAileler: []CekirdekBilgisi{{Ad: "carpim", Pay: 0.6}, {Ad: "dikkat", Pay: 0.3}},
		},
	}
}

// Canlı ekran: adım hızı penceredendir; GPU satırı meşguliyet, adım başına GPU süresi ve
// aileler; bellek satırı Go yığını, CPU tensör ayırması (adım başına) ve GPU belleği.
func TestEkranSatirlari(t *testing.T) {
	satirlar := EkranSatirlari(ornekGoruntu(), Ekran{Genislik: 200, CiktiDosyasi: "c.txt", SonCikti: []string{"adım 300"}})
	metin := strings.Join(satirlar, "\n")
	for _, p := range []string{
		"300/1 000 · 40 ms/adım · 25 adım/sn · kalan ~28 sn",
		"tensör işlemleri %75",
		"matris çarpımı (@)", "ms/adım", "çağrı/adım",
		"çekirdekler %75 meşgul · 30,0 ms/adım · carpim %60 · dikkat %30",
		"egit.tr:12", "geri_yayılım(kayıp)",
		"Go yığını 200 MB", "CPU tensör ayırması 1 MB/adım", "GPU canlı 3 GB (ayrılmış 4 GB, tepe 3 GB)",
		"Programın çıktısı (tamamı: c.txt)", "│ adım 300",
	} {
		if !strings.Contains(metin, p) {
			t.Errorf("ekranda %q yok:\n%s", p, metin)
		}
	}
	// Dar terminalde satırlar genişliği aşmaz.
	for _, s := range EkranSatirlari(ornekGoruntu(), Ekran{Genislik: 60}) {
		if n := utf8.RuneCountInString(s); n > 60 {
			t.Errorf("%d karakterlik satır 60 sütuna sığmıyor: %q", n, s)
		}
	}
}

// Ölçüm kapalıyken GPU satırı nedenini söyler; GPU kullanılmadıysa GPU satırı yoktur.
func TestEkranGPUDurumlari(t *testing.T) {
	g := ornekGoruntu()
	g.GPU.Olcum, g.GPU.Not = false, "çekirdek süreleri ölçülmüyor"
	if m := strings.Join(EkranSatirlari(g, Ekran{Genislik: 200}), "\n"); !strings.Contains(m, "GPU        çekirdek süreleri ölçülmüyor") {
		t.Errorf("ölçüm kapalı notu yok:\n%s", m)
	}
	g.GPU = GPUBilgisi{Not: "GPU kullanılmadı"}
	for _, s := range EkranSatirlari(g, Ekran{Genislik: 200}) {
		if strings.HasPrefix(s, "GPU ") || strings.Contains(s, "GPU canlı") {
			t.Errorf("GPU kullanılmadığı halde GPU satırı: %q", s)
		}
	}
}
