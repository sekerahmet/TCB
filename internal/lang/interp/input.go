package interp

import (
	"bufio"
	"os"
	"sync"
)

// Standart girişin süreçte tek bir tamponlu okuyucusu vardır. Her okuma (girdi(), kabuk,
// terminal hata ayıklayıcısı) bunu kullanır: ayrı okuyucular açılsaydı ilki kaynağın bir
// bölümünü kendi tamponuna çekip öbürlerinin göreceği satırları yutardı.
var stdinOkuyucu = sync.OnceValue(func() *bufio.Reader { return bufio.NewReader(os.Stdin) })

// StdinReader is the single buffered reader of standard input for the process.
func StdinReader() *bufio.Reader { return stdinOkuyucu() }

// InputReader is the single buffered reader of this VM's girdi() source: the shared reader of
// standard input (StdinReader) when Stdin is not set, otherwise a reader private to the VM,
// built on first use. Stdin must be set before the first read.
func (v *VM) InputReader() *bufio.Reader {
	if v.Stdin == nil {
		return StdinReader()
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
	r := v.InputReader()
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
