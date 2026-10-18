package vm

import (
	"errors"
	"fmt"
	"strings"
)

// Bu dosyadaki arayüzler, VM dışında tanımlanan değerlerin (ör. internal/motor/tensor'daki
// tensör) dilin işleçlerine, indekslemesine ve dış biçimlere dönüşümüne katılabilmesi için
// kancalardır. Bir değer arayüzü uygulamıyorsa VM'nin genel davranışı geçerlidir.

// Esitlik, '==' ve '!=' için değerin kendi eşitlik kuralını sağlar. Uygulamayan değerler
// tip ve görünen metinle karşılaştırılır.
type Esitlik interface {
	Esit(diger Value) bool
}

// IslecKancasi ikili işleçleri ('+', '-', '*', '/', '@', '**', '%', '<', '>', '<=', '>='
// ...) sağlar. op işlecin metnidir ("-", "**", "<" ...). sagda=false ise değer sol
// işlenendir (değer op diger), sagda=true ise sağ işlenendir (diger op değer); önce sol
// işlenenin kancası denenir. İşleç desteklenmiyorsa ok=false döner ve VM genel hatayı
// üretir; değer daha açıklayıcı bir hata vermek isterse ok=true ile hata döndürür. '==' ve
// '!=' bu kancadan geçmez (bkz. Esitlik).
type IslecKancasi interface {
	Islec(op string, diger Value, sagda bool) (sonuc Value, ok bool, err error)
}

// GorevliIslecKancasi IslecKancasi'nın işlemi yapan görevi (VM görünümünü) de alan
// biçimidir. Sonucu göreve özel duruma bağlı olan işleçler (ör. karışık_hassasiyet bloğu
// içinde tensör '@') bunu uygular; uygulayan değerde Islec yerine bu çağrılır.
type GorevliIslecKancasi interface {
	GorevliIslec(v *VM, op string, diger Value, sagda bool) (sonuc Value, ok bool, err error)
}

// Olumsuzlanabilir tekli eksiyi ('-x') sağlar.
type Olumsuzlanabilir interface {
	Olumsuz() (Value, error)
}

// Indekslenebilir 'x[i]' okumasını sağlar (atama değil). t[i][j] iki ayrı okumadır;
// değer ilk okumada (ör. bir görünüm) yine Indekslenebilir bir değer döndürebilir.
type Indekslenebilir interface {
	Indeks(idx Value) (Value, error)
}

// IndeksAtanabilir 'x[i] = değer' atamasını sağlar. t[i][j] = değer ataması, t[i]
// okumasının döndürdüğü değere (ör. veriyi paylaşan bir görünüm) [j] ile atamadır. Bileşik
// atamada ('x[i] += 1') VM önce Indeks ile mevcut değeri okur, işlemi yapar ve sonucu
// IndekseAta ile yazar.
type IndeksAtanabilir interface {
	IndekseAta(idx Value, deger Value) error
}

// Yinelenebilir 'döngü x içinde değer', 'döngü i, x içinde değer' ve liste üreteci
// ('[ifade döngü x içinde değer]') için gezilecek öğeleri sağlar. YinelemeUzunlugu öğe
// sayısını (değer üzerinde döngü kurulamıyorsa hata) verir; VM her tur için
// YinelemeOgesi(i) ile 0 ≤ i < uzunluk olan i. öğeyi ister. Uzunluk döngü başında bir kez
// okunur.
type Yinelenebilir interface {
	YinelemeUzunlugu() (int, error)
	YinelemeOgesi(i int) (Value, error)
}

// DuzDegerli, değerin dış biçimlere (JSON, Go değerleri) yazılırken yerine
// geçen düz TR değerini verir (ör. tensör → iç içe Ondalık listesi).
type DuzDegerli interface {
	DuzDeger() Value
}

// SatirliHata satır bilgisi eklenmiş bir çalışma zamanı hatasıdır. Yerleşiklerin ve dış
// değerlerin (ör. tensör) hataları bununla sarılır; zaten satır bilgisi taşıyan bir hata
// (ör. yerleşiğin geri çağırdığı fonksiyonun hatası) ikinci kez sarılmaz, böylece mesaj ve
// kod çerçevesi asıl hatanın satırını gösterir.
type SatirliHata struct {
	Satir int
	Hata  error
}

func (h *SatirliHata) Error() string { return fmt.Sprintf("Satır %d: %v", h.Satir, h.Hata) }
func (h *SatirliHata) Unwrap() error { return h.Hata }

// satirliMi hata zaten satır bilgisi taşıyor mu? Tipli hata önceliklidir; VM'nin metinle
// ürettiği "Satır N: ..." hataları için metin öneki yedek olarak denetlenir.
func satirliMi(err error) bool {
	var sh *SatirliHata
	var mh *ModulHatasi
	return errors.As(err, &sh) || errors.As(err, &mh) || strings.HasPrefix(err.Error(), "Satır ")
}

// satirEki hatayı (satır bilgisi yoksa) "Satır N: " önekli bir SatirliHata'ya çevirir.
func satirEki(line int, err error) error {
	if err == nil || line <= 0 || satirliMi(err) {
		return err
	}
	return &SatirliHata{Satir: line, Hata: err}
}

// kancaIslec ikili işlemi (varsa) değerlerin işleç kancalarıyla yapar.
func (v *VM) kancaIslec(op string, left, right Value, line int) (Value, bool, error) {
	if k, ok := left.(GorevliIslecKancasi); ok {
		if val, ok, err := k.GorevliIslec(v, op, right, false); ok {
			return val, true, satirEki(line, err)
		}
	} else if k, ok := left.(IslecKancasi); ok {
		if val, ok, err := k.Islec(op, right, false); ok {
			return val, true, satirEki(line, err)
		}
	}
	if k, ok := right.(GorevliIslecKancasi); ok {
		if val, ok, err := k.GorevliIslec(v, op, left, true); ok {
			return val, true, satirEki(line, err)
		}
	} else if k, ok := right.(IslecKancasi); ok {
		if val, ok, err := k.Islec(op, left, true); ok {
			return val, true, satirEki(line, err)
		}
	}
	return nil, false, nil
}
