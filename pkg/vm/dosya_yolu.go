package vm

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"tr_dili/pkg/turkce"
)

// YolCoz TR programlarındaki göreli dosya yollarının TEK kuralını uygular; dosya
// okuyan ve yazan her yerleşik (dosya_oku, dosya_yaz, aktar ve motor paketlerinin
// yükle/kaydet işlevleri) yolu bununla çözmelidir.
//
// Kural (okuma ve yazma için aynı): göreli bir yol önce çalışma klasörüne, sonra
// (program bir dosyadan çalışıyorsa) programın klasörüne göre denenir ve dosyanın
// bulunduğu ilk yer kullanılır. Dosya hiçbir yerde yoksa, bulunacağı klasörün var
// olduğu ilk yer kullanılır (düz bir "a.txt" için bu her zaman çalışma klasörüdür);
// o da yoksa yol olduğu gibi (çalışma klasörüne göre) döner. Okuma bulunamayan
// dosyada "dosya yok" hatası verir, yazma dosyayı seçilen yerde oluşturur.
//
// Gerekçe: aynı yol okumada ve yazmada aynı dosyayı gösterir (programın yanındaki
// sayaç.txt okunup güncellenince yine o dosyaya yazılır; ayrı okuma/yazma kuralı
// birini okuyup ötekine yazmaya yol açardı), program hangi klasörden çalıştırılırsa
// çalıştırılsın yanındaki verisini bulur, yeni dosyalar ise kabuktaki alışkanlıkla
// uyumlu olarak çalışma klasöründe oluşur. Mutlak yollar olduğu gibi kullanılır.
//
// bakilanlar, hata mesajlarında "bakılan yerler" olarak gösterilmek üzere denenen
// yerlerin mutlak yollarıdır (mutlak ya da boş yolda nil).
func (v *VM) YolCoz(yol string) (cozulen string, bakilanlar []string) {
	if yol == "" || filepath.IsAbs(yol) {
		return yol, nil
	}
	adaylar := []string{yol}
	bakilanlar = []string{mutlakYol(yol)}
	for _, klasor := range v.aramaKlasorleri() {
		aday := filepath.Join(klasor, yol)
		if mutlak := mutlakYol(aday); !slices.Contains(bakilanlar, mutlak) {
			adaylar = append(adaylar, aday)
			bakilanlar = append(bakilanlar, mutlak)
		}
	}
	// 1. Dosyanın (ya da klasörün) kendisinin bulunduğu ilk yer
	for _, aday := range adaylar {
		if _, err := os.Stat(aday); err == nil {
			return aday, bakilanlar
		}
	}
	// 2. Yoksa, bulunacağı klasörün var olduğu ilk yer: yeni bir "çıktı/rapor.txt"
	// programın yanındaki çıktı/ klasörüne yazılır; okumada "dosya yok" hatası o
	// klasördeki eksik dosyayı söyler (yanıltıcı biçimde "klasör yok" demez).
	for _, aday := range adaylar {
		if bilgi, err := os.Stat(filepath.Dir(aday)); err == nil && bilgi.IsDir() {
			return aday, bakilanlar
		}
	}
	return yol, bakilanlar
}

// BakilanYerler YolCoz'un döndürdüğü yerleri hata mesajının ekine çevirir.
func BakilanYerler(bakilanlar []string) string {
	switch len(bakilanlar) {
	case 0:
		return ""
	case 1:
		return "bakılan yer: " + bakilanlar[0]
	}
	return "bakılan yerler: " + strings.Join(bakilanlar, ", ")
}

// OkumaHatasi bir okuma işleminin hatasını turkce.DosyaHatasi ile anlatır; dosya
// bulunamadıysa YolCoz'un baktığı yerleri de ekler. fn hatayı veren yerleşik, yol
// kullanıcının yazdığı yol, islem "okunamadı" gibi bir sonuçtur.
func OkumaHatasi(fn, yol, islem string, err error, bakilanlar []string) error {
	ek := ""
	if errors.Is(err, fs.ErrNotExist) {
		ek = BakilanYerler(bakilanlar)
	}
	return turkce.DosyaHatasi(fn, yol, islem, err, ek)
}

// aramaKlasorleri göreli yolların çalışma klasöründen sonra arandığı klasörlerdir: kod bir
// modülde çalışıyorsa önce o modülün klasörü, sonra programın klasörü. Program bir
// dosyadan çalışmıyorsa (etkileşimli kabuk, derlenmiş uygulama: "<...>") onun klasörü yoktur.
func (v *VM) aramaKlasorleri() []string {
	if v == nil {
		return nil
	}
	var klasorler []string
	if v.kaynak != nil && v.kaynak.Yol != "" && !strings.HasPrefix(v.kaynak.Yol, "<") {
		klasorler = append(klasorler, filepath.Dir(v.kaynak.Yol))
	}
	if v.SourceFile != "" && !strings.HasPrefix(v.SourceFile, "<") {
		klasorler = append(klasorler, filepath.Dir(v.SourceFile))
	}
	return klasorler
}

func mutlakYol(yol string) string {
	if m, err := filepath.Abs(yol); err == nil {
		return m
	}
	return yol
}
