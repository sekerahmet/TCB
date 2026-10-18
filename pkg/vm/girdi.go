package vm

import (
	"bufio"
	"os"
	"sync"
)

// Standart girişin süreçte tek bir tamponlu okuyucusu vardır. Her okuma (girdi(), kabuk,
// terminal hata ayıklayıcısı) bunu kullanır: ayrı okuyucular açılsaydı ilki kaynağın bir
// bölümünü kendi tamponuna çekip öbürlerinin göreceği satırları yutardı.
var stdinOkuyucu = sync.OnceValue(func() *bufio.Reader { return bufio.NewReader(os.Stdin) })

// StdinOkuyucu standart girişin süreç başına tek tamponlu okuyucusudur.
func StdinOkuyucu() *bufio.Reader { return stdinOkuyucu() }

// GirdiOkuyucu bu VM'in girdi() kaynağının tek tamponlu okuyucusudur: Stdin verilmemişse
// standart girişin ortak okuyucusu (StdinOkuyucu), verilmişse VM'e özel, ilk kullanımda
// kurulan bir okuyucu. Stdin ilk okumadan önce ayarlanmalıdır.
func (v *VM) GirdiOkuyucu() *bufio.Reader {
	if v.Stdin == nil {
		return StdinOkuyucu()
	}
	v.girdiMu.Lock()
	defer v.girdiMu.Unlock()
	if v.girdiOkuyucu == nil {
		v.girdiOkuyucu = bufio.NewReader(v.Stdin)
	}
	return v.girdiOkuyucu
}

// satirOku girdi() için bir satır okur. Okuma yorumlayıcı kilidi bırakılarak yapılır;
// aynı anda okuyan görevler okuyucuyu sırayla kullanır.
func (v *VM) satirOku() (string, error) {
	r := v.GirdiOkuyucu()
	var satir string
	var err error
	v.Blocking(func() {
		girdiSirasi.Lock()
		defer girdiSirasi.Unlock()
		satir, err = r.ReadString('\n')
	})
	return satir, err
}

// girdiSirasi aynı okuyucudan aynı anda okuyan görevleri sıraya koyar.
var girdiSirasi sync.Mutex
