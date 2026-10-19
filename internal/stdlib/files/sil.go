package files

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// silYerlesik sil(yol, içindekilerle: yanlış) yerleşiğidir: dosyayı, bağlantıyı ya da boş
// klasörü (içindekilerle: doğru ile dolu klasörü) siler, silinen dosya sayısını verir.
func silYerlesik(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G123)
	if err := bind.AdliDenetle(fn, named, adIcindekilerle); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.G123, "1", len(args), "dosya ya da klasör")
	}
	ham, err := yolOku(codes.G123, 1, args[0])
	if err != nil {
		return nil, err
	}
	icindekilerle, err := mantikAdli(codes.G123, named, adIcindekilerle)
	if err != nil {
		return nil, err
	}
	cozulen, bakilanlar := m.ResolvePath(filepath.FromSlash(ham))
	bilgi, err := os.Lstat(cozulen)
	if err != nil {
		return nil, interp.ReadError(fn, ham, "silinemedi", err, bakilanlar)
	}
	gercek := gercekYol(cozulen)
	if neden := silmeKorumasi(m, gercek); neden != "" {
		return nil, fmt.Errorf("%s: %q (%s) %s; TR bunu silmez", fn, ham, filepath.ToSlash(gercek), neden)
	}
	if err := acikDosyaHatasi(fn, ham, gercek); err != nil {
		return nil, err
	}
	if bilgi.IsDir() && !icindekilerle {
		bos, err := bosKlasorMu(cozulen)
		if err != nil {
			return nil, turkish.FileError(fn, ham, "silinemedi", err)
		}
		if !bos {
			return nil, fmt.Errorf("%s: %q klasörü boş değil; içindekilerle birlikte silmek için %s doğru verin", fn, ham, adIcindekilerle.TR+":")
		}
	}
	var sayi int
	m.Blocking(func() { sayi, err = agaciSil(cozulen, bilgi) })
	if err != nil {
		return nil, turkish.FileError(fn, ham, "silinemedi", err)
	}
	return interp.NewInt(int64(sayi)), nil
}

func bosKlasorMu(yol string) (bool, error) {
	f, err := os.Open(yol)
	if err != nil {
		return false, err
	}
	defer f.Close()
	adlar, err := f.Readdirnames(1)
	if len(adlar) > 0 {
		return false, nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}

// saltOkunurHatasi salt okunur bir dosyanın silinmediğini bildirir. Salt okunur dosyalar zorla
// silinmez (Windows'ta işletim sisteminin silme çağrısı salt okunur bayrağını kendisi
// kaldırırdı); klasörde böyle bir dosya varsa hiçbir şey silinmez.
func saltOkunurHatasi(yol, ic string) error {
	neden := "salt okunur bir dosya; TR salt okunur dosyaları zorla silmez (önce yazma iznini açın)"
	if ic != "" {
		neden = fmt.Sprintf("içinde salt okunur bir dosya var (%q); TR salt okunur dosyaları zorla silmez, hiçbir şey silinmedi (önce yazma iznini açın)", ic)
	}
	return &fs.PathError{Op: "remove", Path: yol, Err: turkish.Reason(neden)}
}

// agaciSil yolu siler; klasörse içindekilerle birlikte. Bağlantıların gösterdiği yere
// dokunulmaz: klasörün içi os.Root ile gezilir ve silinir, gezinme sırasında bir alt klasör
// bağlantıyla değiştirilse de işlem klasörün dışına çıkmaz. Önce bütün ağaç denetlenir:
// salt okunur bir dosya varsa hiçbir şey silinmez. Silinen dosya (klasör olmayan girdi)
// sayısını verir.
func agaciSil(yol string, bilgi fs.FileInfo) (int, error) {
	if !bilgi.IsDir() {
		if saltOkunur(bilgi) {
			return 0, saltOkunurHatasi(yol, "")
		}
		return 1, os.Remove(yol)
	}
	kok, err := os.OpenRoot(yol)
	if err != nil {
		return 0, err
	}
	sayi, ustler, err := agaciDenetle(kok, yol)
	if err == nil {
		for _, ad := range ustler {
			if err = kok.RemoveAll(ad); err != nil {
				break
			}
		}
	}
	kok.Close()
	if err != nil {
		return 0, err
	}
	return sayi, os.Remove(yol)
}

// agaciDenetle kökün altını (bağlantıların içine girmeden) gezer: klasör olmayan girdileri
// sayar, salt okunur dosya arar ve kökün doğrudan altındaki adları verir.
func agaciDenetle(kok *os.Root, yol string) (int, []string, error) {
	sayi := 0
	var ustler []string
	err := fs.WalkDir(kok.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if filepath.Dir(filepath.FromSlash(p)) == "." {
			ustler = append(ustler, p)
		}
		if d.IsDir() {
			return nil
		}
		sayi++
		if d.Type().IsRegular() {
			b, err := d.Info()
			if err != nil {
				return err
			}
			if saltOkunur(b) {
				return saltOkunurHatasi(yol, p)
			}
		}
		return nil
	})
	return sayi, ustler, err
}

// saltOkunur sıradan bir dosya yazmaya kapalı mı? Windows'ta salt okunur bayrağı, öteki
// sistemlerde sahibin yazma izni (kabukta da silmeden önce onay sorulan durum).
func saltOkunur(b fs.FileInfo) bool {
	return b.Mode().IsRegular() && b.Mode().Perm()&0o200 == 0
}
