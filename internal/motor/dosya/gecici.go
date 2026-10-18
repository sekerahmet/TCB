package dosya

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
)

// geciciKlasorler bu sürecin geçici_klasör() ile oluşturduğu klasörlerdir; program bitince
// (TumunuKapat) içindekilerle silinirler. VM, silme korumasının programın ve modüllerin
// klasörlerini bilmesi içindir (silmeKorumasi).
var geciciKlasorler = struct {
	sync.Mutex
	l []geciciKlasor
}{}

type geciciKlasor struct {
	yol string // oluşturulduğu mutlak yol
	m   *yurut.VM
}

// geciciKlasorYerlesik geçici_klasör() yerleşiğidir: işletim sisteminin geçici klasörünün
// altında bu süreç için yeni, boş ve yalnızca kullanıcıya açık (0700) bir klasör oluşturur ve
// Yol olarak döndürür.
func geciciKlasorYerlesik(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := yurut.Cagri(kod.G126)
	if len(args) != 0 {
		return nil, yurut.ArgumanSayisiHatasi(kod.G126, "0", len(args))
	}
	ust := os.TempDir()
	yol, err := os.MkdirTemp(ust, fmt.Sprintf("tr_%d_*", os.Getpid()))
	if err != nil {
		return nil, turkce.DosyaHatasi(fn, filepath.ToSlash(ust), "içinde geçici klasör oluşturulamadı", err)
	}
	if mutlak, err := filepath.Abs(yol); err == nil {
		yol = mutlak
	}
	geciciKlasorler.Lock()
	geciciKlasorler.l = append(geciciKlasorler.l, geciciKlasor{yol: yol, m: m})
	geciciKlasorler.Unlock()
	return YeniYol(filepath.ToSlash(yol)), nil
}

// geciciKlasorleriSil geçici_klasör() ile oluşturulan klasörleri içindekilerle siler (program
// bitince; açık satır okuyucuları önce kapatılmış olmalıdır). sil'in kuralları geçerlidir:
// korunan bir yer silinmez, bağlantıların gösterdiği yere dokunulmaz, salt okunur bir dosya
// bulunan klasör silinmez. Silinemeyen klasörler için Türkçe bir uyarı döner.
func geciciKlasorleriSil() []error {
	geciciKlasorler.Lock()
	liste := geciciKlasorler.l
	geciciKlasorler.l = nil
	geciciKlasorler.Unlock()
	var hatalar []error
	fn := yurut.Cagri(kod.G126)
	for _, g := range liste {
		bilgi, err := os.Lstat(g.yol)
		if errors.Is(err, fs.ErrNotExist) {
			continue // program klasörü kendisi silmiş ya da taşımış
		}
		ham := filepath.ToSlash(g.yol)
		if err != nil {
			hatalar = append(hatalar, turkce.DosyaHatasi(fn, ham, "silinemedi", err))
			continue
		}
		if neden := silmeKorumasi(g.m, gercekYol(g.yol)); neden != "" {
			hatalar = append(hatalar, fmt.Errorf("%s: %q %s; TR bunu silmez", fn, ham, neden))
			continue
		}
		if _, err := agaciSil(g.yol, bilgi); err != nil {
			hatalar = append(hatalar, turkce.DosyaHatasi(fn, ham, "silinemedi", err))
		}
	}
	return hatalar
}
