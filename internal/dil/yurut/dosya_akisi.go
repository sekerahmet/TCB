package yurut

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
)

// .gz saydamlığı: yolu .gz ile biten (büyük/küçük harf fark etmez) bir dosya dosya_oku,
// dosya_satırları ve csv_oku'da açılarak okunur, dosya_yaz'da sıkıştırılarak yazılır. Kural
// yalnızca uzantıya bakar; içerik gzip imzası taşımıyorsa okuma açık bir hatayla durur
// (sessizce sıkıştırılmamış okumaya düşülmez). Bütün dosya okuyan ve yazan yerleşikler
// dosyayı buradaki yardımcılarla açar; aynı kural iki yerde yazılmaz.

// gzipIcerikSiniri dosya_oku'nun bir .gz dosyadan açacağı en büyük içeriktir (1 GB); daha
// büyük dosyalar dosya_satırları ile satır satır okunur.
var gzipIcerikSiniri = 1 << 30

// GzipMi yol .gz uzantılı mı (saydam sıkıştırma kuralı)?
func GzipMi(yol string) bool { return strings.EqualFold(filepath.Ext(yol), ".gz") }

// DosyaOkuyucuAc dosyayı okumak için açar; .gz uzantılıysa açılmış içeriği veren bir okuyucu
// döndürür. Hatalar turkce.DosyaHatasi ile anlatılmaya uygundur (yol işletim sistemi
// hatasında, gzip sorunlarında turkce.Neden).
func DosyaOkuyucuAc(yol string) (io.ReadCloser, error) {
	f, err := os.Open(yol)
	if err != nil {
		return nil, err
	}
	if !GzipMi(yol) {
		return f, nil
	}
	tampon := bufio.NewReaderSize(f, 1<<16)
	imza, err := tampon.Peek(2)
	if err != nil || imza[0] != 0x1f || imza[1] != 0x8b {
		f.Close()
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		return nil, turkce.Neden("uzantısı .gz ama içerik gzip değil (dosya sıkıştırılmamışsa adını .gz olmadan verin)")
	}
	gz, err := gzip.NewReader(tampon)
	if err != nil {
		f.Close()
		return nil, gzipHatasi(err)
	}
	return &gzipOkuyucu{gz: gz, f: f}, nil
}

// gzipOkuyucu açılmış içeriği okur; gzip hatalarını Türkçe nedenlere çevirir.
type gzipOkuyucu struct {
	gz *gzip.Reader
	f  *os.File
}

func (g *gzipOkuyucu) Read(p []byte) (int, error) {
	n, err := g.gz.Read(p)
	if err != nil && err != io.EOF {
		err = gzipHatasi(err)
	}
	return n, err
}

func (g *gzipOkuyucu) Close() error {
	g.gz.Close()
	return g.f.Close()
}

// gzipHatasi gzip okuyucusunun hatasını Türkçe bir nedene çevirir; işletim sistemi hatalarını
// olduğu gibi bırakır.
func gzipHatasi(err error) error {
	switch {
	case errors.Is(err, gzip.ErrChecksum):
		return turkce.Neden("gzip içeriği bozuk (sağlama toplamı tutmuyor)")
	case errors.Is(err, gzip.ErrHeader):
		return turkce.Neden("gzip başlığı bozuk")
	case errors.Is(err, io.ErrUnexpectedEOF):
		return turkce.Neden("gzip içeriği yarıda kesilmiş (dosya eksik indirilmiş olabilir)")
	}
	var bozuk flate.CorruptInputError
	if errors.As(err, &bozuk) {
		return turkce.Neden("gzip içeriği bozuk (sıkıştırılmış veri çözülemedi)")
	}
	return err
}

// DosyaIceriginiOku dosyanın bütün içeriğini okur (.gz uzantılıysa açılmış içeriği, en çok
// gzipIcerikSiniri bayt).
func DosyaIceriginiOku(yol string) ([]byte, error) {
	if !GzipMi(yol) {
		return os.ReadFile(yol)
	}
	r, err := DosyaOkuyucuAc(yol)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	veri, err := io.ReadAll(io.LimitReader(r, int64(gzipIcerikSiniri)+1))
	if err != nil {
		return nil, err
	}
	if len(veri) > gzipIcerikSiniri {
		return nil, turkce.Neden(fmt.Sprintf("açılmış içerik 1 GB'tan büyük; büyük dosyaları satır satır okumak için %s(...) kullanın", kod.Ad(kod.G112)))
	}
	return veri, nil
}

// DosyaIceriginiYaz içeriği dosyaya atomik olarak yazar (bkz. AtomikYaz); yol .gz uzantılıysa
// içerik sıkıştırılır.
func DosyaIceriginiYaz(yol string, veri []byte) error {
	return AtomikYaz(yol, func(w io.Writer) error {
		if !GzipMi(yol) {
			_, err := w.Write(veri)
			return err
		}
		gz := gzip.NewWriter(w)
		if _, err := gz.Write(veri); err != nil {
			return err
		}
		return gz.Close()
	})
}
