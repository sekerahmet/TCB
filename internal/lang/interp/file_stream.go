package interp

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

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// .gz saydamlığı: yolu .gz ile biten (büyük/küçük harf fark etmez) bir dosya dosya_oku,
// dosya_satırları ve csv_oku'da açılarak okunur, dosya_yaz'da sıkıştırılarak yazılır. Kural
// yalnızca uzantıya bakar; içerik gzip imzası taşımıyorsa okuma açık bir hatayla durur
// (sessizce sıkıştırılmamış okumaya düşülmez). Bütün dosya okuyan ve yazan yerleşikler
// dosyayı buradaki yardımcılarla açar; aynı kural iki yerde yazılmaz.

// gzipContentLimit dosya_oku'nun bir .gz dosyadan açacağı en büyük içeriktir (1 GB); daha
// büyük dosyalar dosya_satırları ile satır satır okunur.
var gzipContentLimit = 1 << 30

// IsGzip reports whether the path has a .gz extension (the transparent compression rule).
func IsGzip(filePath string) bool { return strings.EqualFold(filepath.Ext(filePath), ".gz") }

// OpenFileReader opens a file for reading; for a .gz path it returns a reader of the
// decompressed content. Errors are suitable for turkish.FileError (an OS error for the path,
// turkish.Reason for gzip problems).
func OpenFileReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	if !IsGzip(filePath) {
		return f, nil
	}
	buffer := bufio.NewReaderSize(f, 1<<16)
	signature, err := buffer.Peek(2)
	if err != nil || signature[0] != 0x1f || signature[1] != 0x8b {
		f.Close()
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		return nil, turkish.Reason("uzantısı .gz ama içerik gzip değil (dosya sıkıştırılmamışsa adını .gz olmadan verin)")
	}
	gz, err := gzip.NewReader(buffer)
	if err != nil {
		f.Close()
		return nil, gzipError(err)
	}
	return &gzipReader{gz: gz, f: f}, nil
}

// gzipReader açılmış içeriği okur; gzip hatalarını Türkçe nedenlere çevirir.
type gzipReader struct {
	gz *gzip.Reader
	f  *os.File
}

func (g *gzipReader) Read(p []byte) (int, error) {
	n, err := g.gz.Read(p)
	if err != nil && err != io.EOF {
		err = gzipError(err)
	}
	return n, err
}

func (g *gzipReader) Close() error {
	g.gz.Close()
	return g.f.Close()
}

// gzipError gzip okuyucusunun hatasını Türkçe bir nedene çevirir; işletim sistemi hatalarını
// olduğu gibi bırakır.
func gzipError(err error) error {
	switch {
	case errors.Is(err, gzip.ErrChecksum):
		return turkish.Reason("gzip içeriği bozuk (sağlama toplamı tutmuyor)")
	case errors.Is(err, gzip.ErrHeader):
		return turkish.Reason("gzip başlığı bozuk")
	case errors.Is(err, io.ErrUnexpectedEOF):
		return turkish.Reason("gzip içeriği yarıda kesilmiş (dosya eksik indirilmiş olabilir)")
	}
	var broken flate.CorruptInputError
	if errors.As(err, &broken) {
		return turkish.Reason("gzip içeriği bozuk (sıkıştırılmış veri çözülemedi)")
	}
	return err
}

// ReadFileContent reads a file's whole content (decompressed for .gz, at most
// gzipIcerikSiniri bytes).
func ReadFileContent(filePath string) ([]byte, error) {
	if !IsGzip(filePath) {
		return os.ReadFile(filePath)
	}
	r, err := OpenFileReader(filePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	payload, err := io.ReadAll(io.LimitReader(r, int64(gzipContentLimit)+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > gzipContentLimit {
		return nil, turkish.Reason(fmt.Sprintf("açılmış içerik 1 GB'tan büyük; büyük dosyaları satır satır okumak için %s(...) kullanın", codes.Name(codes.G112)))
	}
	return payload, nil
}

// WriteFileContent writes content to a file atomically (see WriteAtomic); for a .gz path the
// content is compressed.
func WriteFileContent(filePath string, payload []byte) error {
	return WriteAtomic(filePath, func(w io.Writer) error {
		if !IsGzip(filePath) {
			_, err := w.Write(payload)
			return err
		}
		gz := gzip.NewWriter(w)
		if _, err := gz.Write(payload); err != nil {
			return err
		}
		return gz.Close()
	})
}
