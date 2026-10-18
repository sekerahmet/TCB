package vm

import (
	"errors"
	"fmt"
	"strings"
)

// Bu dosyadaki arayüzler, VM dışında tanımlanan değerlerin (ör. pkg/motor/tensor'daki
// tensör) dilin işleçlerine ve indekslemesine katılabilmesi için kancalardır. Bir değer
// arayüzü uygulamıyorsa VM'nin eski davranışı geçerlidir.

// Esitlik, '==' ve '!=' için değerin kendi eşitlik kuralını sağlar. Uygulamayan değerler
// tip ve görünen metinle karşılaştırılır.
type Esitlik interface {
	Esit(diger Value) bool
}

// IslecKancasi, MatrixOperator'ın karşılamadığı ikili işleçleri sağlar: sağ işlenenken
// '-', '/', '**', '%' ile '<', '>', '<=', '>=' gibi işleçler. op işlecin metnidir ("-",
// "**", "<" ...). sagda=false ise değer sol işlenendir (değer op diger), sagda=true ise
// sağ işlenendir (diger op değer). İşleç desteklenmiyorsa ok=false döner ve VM genel
// hatayı üretir; değer daha açıklayıcı bir hata vermek isterse ok=true ile hata döndürür.
type IslecKancasi interface {
	Islec(op string, diger Value, sagda bool) (sonuc Value, ok bool, err error)
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

// SatirliHata satır bilgisi eklenmiş bir çalışma zamanı hatasıdır. Yerleşiklerin ve dış
// değerlerin (tensör, matris) hataları bununla sarılır; zaten satır bilgisi taşıyan bir
// hata (ör. yerleşiğin geri çağırdığı fonksiyonun hatası) ikinci kez sarılmaz, böylece
// mesaj ve kod çerçevesi asıl hatanın satırını gösterir.
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
	return errors.As(err, &sh) || strings.HasPrefix(err.Error(), "Satır ")
}

// satirEki hatayı (satır bilgisi yoksa) "Satır N: " önekli bir SatirliHata'ya çevirir.
func satirEki(line int, err error) error {
	if err == nil || line <= 0 || satirliMi(err) {
		return err
	}
	return &SatirliHata{Satir: line, Hata: err}
}

// kancaIslec ikili işlemi (varsa) değerlerin işleç kancalarıyla yapar.
func kancaIslec(op string, left, right Value, line int) (Value, bool, error) {
	if k, ok := left.(IslecKancasi); ok {
		if val, ok, err := k.Islec(op, right, false); ok {
			return val, true, satirEki(line, err)
		}
	}
	if k, ok := right.(IslecKancasi); ok {
		if val, ok, err := k.Islec(op, left, true); ok {
			return val, true, satirEki(line, err)
		}
	}
	return nil, false, nil
}
