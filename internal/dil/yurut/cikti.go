package yurut

import (
	"io"
	"slices"
	"strings"
)

// Çıktı noktaları: insanın göreceği her çıktı (yazdır, girdi istemi, hata mesajları,
// uyarılar; ileride günlük ve ilerleme satırı) CiktiYaz'dan geçer. Burada iki kanca vardır:
//
//   - Gizli değer süzgeci: kayıtlı gizli değerler (ortam değişkeninden ya da komut satırından
//     gelen anahtar, parola ...) çıktıda *** ile maskelenir. Bu bir sızıntı azaltma önlemidir,
//     garanti değildir (metin dönüştürülünce eşleşme kaybolur). Veri çıktıları (dosya_yaz,
//     JSON, yazdır'ın döndürdüğü değer) değiştirilmez.
//   - Canlı satır: terminalin son satırında yeniden çizilen bir çıktı (ilerleme çubuğu)
//     etkinse, metin yazılmadan önce silinir ve sonra yeniden çizilir; çıktılar çubuğa
//     karışmaz.
//
// Kanca kurulmamışsa (gizli değer yok, canlı satır yok) maliyet iki atomik okumadır.

// GizliMaske gizli değerlerin çıktıda yerine yazılan metindir.
const GizliMaske = "***"

// GizliDegerEnAzUzunluk süzgece alınan bir gizli değerin en az karakter sayısıdır; daha kısa
// değerler sıradan metinlerde de geçeceğinden (çıktıyı bozmamak için) maskelenmez.
const GizliDegerEnAzUzunluk = 6

// CanliSatir terminalin son satırında yeniden çizilen bir çıktıdır (ör. ilerleme çubuğu).
// Sil ve Ciz kendi akışına (çoğunlukla standart hata) yazar; VM onları çıktı kilidi altında
// çağırır.
type CanliSatir interface {
	Sil() // satırı ekrandan siler
	Ciz() // satırı yeniden çizer
}

// GizliDegerEkle bir değeri gizli değer süzgecine kaydeder; değer en az
// GizliDegerEnAzUzunluk karakter değilse kaydedilmez ve yanlış döner. Herhangi bir
// goroutine'den çağrılabilir.
func (v *VM) GizliDegerEkle(deger string) bool {
	if len([]rune(deger)) < GizliDegerEnAzUzunluk {
		return false
	}
	v.gizliMu.Lock()
	defer v.gizliMu.Unlock()
	if slices.Contains(v.gizliler, deger) {
		return true
	}
	v.gizliler = append(v.gizliler, deger)
	// Uzun değerler önce denenir: biri ötekini içerirse uzunu bütünüyle maskelensin.
	sirali := slices.Clone(v.gizliler)
	slices.SortFunc(sirali, func(a, b string) int { return len(b) - len(a) })
	ciftler := make([]string, 0, 2*len(sirali))
	for _, g := range sirali {
		ciftler = append(ciftler, g, GizliMaske)
	}
	v.maske.Store(strings.NewReplacer(ciftler...))
	return true
}

// GizliDegerVar süzgece kayıtlı bir gizli değer var mı? Çıktının tamamını gezmenin pahalı
// olduğu yerler (ör. hata ayıklama iletileri) önce buna bakar.
func (v *VM) GizliDegerVar() bool { return v.maske.Load() != nil }

// Maskele metindeki kayıtlı gizli değerleri GizliMaske ile değiştirir.
func (v *VM) Maskele(metin string) string {
	if r := v.maske.Load(); r != nil {
		return r.Replace(metin)
	}
	return metin
}

// CanliSatirAyarla etkin canlı satırı ayarlar (nil: kaldırır). Aynı anda tek canlı satır
// vardır; yenisi eskisinin yerine geçer.
func (v *VM) CanliSatirAyarla(c CanliSatir) {
	v.ciktiMu.Lock()
	defer v.ciktiMu.Unlock()
	v.canli = c
	v.canliVar.Store(c != nil)
}

// CanliSatirGuncelle fn'i çıktı kilidi altında çalıştırır: canlı satırın kendisini çizmesi
// başka bir çıktıyla iç içe geçmesin.
func (v *VM) CanliSatirGuncelle(fn func()) {
	v.ciktiMu.Lock()
	defer v.ciktiMu.Unlock()
	fn()
}

// CiktiYaz insanın göreceği bir metni w'ye yazar: gizli değerler maskelenir, etkin canlı
// satır önce silinir, sonra yeniden çizilir.
func (v *VM) CiktiYaz(w io.Writer, metin string) {
	metin = v.Maskele(metin)
	if !v.canliVar.Load() {
		_, _ = io.WriteString(w, metin)
		return
	}
	v.ciktiMu.Lock()
	defer v.ciktiMu.Unlock()
	c := v.canli
	if c != nil {
		c.Sil()
	}
	_, _ = io.WriteString(w, metin)
	if c != nil {
		c.Ciz()
	}
}
