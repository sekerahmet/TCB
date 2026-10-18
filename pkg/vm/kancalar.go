package vm

import (
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

// satirEki VM dışından gelen bir hatanın başına (yoksa) "Satır N: " ekler; böylece hata
// mesajı fonksiyon hatalarıyla tutarlı olur ve kod çerçevesiyle gösterilebilir.
func satirEki(line int, err error) error {
	if err == nil || line <= 0 || strings.HasPrefix(err.Error(), "Satır ") {
		return err
	}
	return fmt.Errorf("Satır %d: %w", line, err)
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
