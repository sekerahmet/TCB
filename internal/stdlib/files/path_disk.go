package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// modifiedNs .TR_modified_ns'dir (18.3): dosyanın son değiştirilme zamanı, Unix zamanı,
// nanosaniye. Çözünürlük dosya sistemininkidir (FAT'te 2 saniye). Klasörde H7736; olmayan
// dosyada .TR_size'ınki gibi okuma hatası.
func (y *Path) modifiedNs(m *interp.VM) (interp.Value, error) {
	fn := codes.CallNameRef(codes.MemModifiedNs)
	c, tried := y.resolve(m)
	b, err := os.Stat(c)
	if err != nil {
		return nil, interp.ReadError(fn, y.fpath, messages.H2100, err, tried)
	}
	if b.IsDir() {
		return nil, messages.New(messages.H7736, fn, y.fpath)
	}
	return interp.NewInt(b.ModTime().UnixNano()), nil
}

// diskAvailableBytes .TR_disk_available_bytes'tır (18.3): yolun diskinde bu kullanıcıya açık boş
// yer (kota ve yöneticiye ayrılan pay düşülmüş). Dosyada ve klasörde çalışır; olmayan yolda
// H7737 üst klasörü önerir, kendisi üst klasöre çıkmaz (yazım yanlışı sessizce başka diskin
// değerini verirdi).
func (y *Path) diskAvailableBytes(m *interp.VM) (interp.Value, error) {
	fn := codes.CallNameRef(codes.MemDiskAvailableBytes)
	c, tried := y.resolve(m)
	b, err := os.Stat(c)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, messages.New(messages.H7737, fn, y.fpath, y.parent().fpath)
	}
	if err != nil {
		return nil, interp.ReadError(fn, y.fpath, messages.H2100, err, tried)
	}
	dir := c
	if !b.IsDir() {
		dir = filepath.Dir(c) // Windows'un çağrısı klasör ister; aynı disk
	}
	n, err := freeBytesForUser(dir)
	if err != nil {
		return nil, interp.ReadError(fn, y.fpath, messages.H2100, err, nil)
	}
	if n > 1<<63-1 {
		n = 1<<63 - 1
	}
	return interp.NewInt(int64(n)), nil
}
