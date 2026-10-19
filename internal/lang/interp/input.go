package interp

import (
	"bufio"
	"os"
	"sync"
)

// Standart girişin süreçte tek bir tamponlu okuyucusu vardır. Her okuma (girdi(), kabuk,
// terminal hata ayıklayıcısı) bunu kullanır: ayrı okuyucular açılsaydı ilki kaynağın bir
// bölümünü kendi tamponuna çekip öbürlerinin göreceği satırları yutardı.
var stdinReader = sync.OnceValue(func() *bufio.Reader { return bufio.NewReader(os.Stdin) })

// StdinReader is the single buffered reader of standard input for the process.
func StdinReader() *bufio.Reader { return stdinReader() }

// InputReader is the single buffered reader of this VM's girdi() source: the shared reader of
// standard input (StdinReader) when Stdin is not set, otherwise a reader private to the VM,
// built on first use. Stdin must be set before the first read.
func (v *VM) InputReader() *bufio.Reader {
	if v.Stdin == nil {
		return StdinReader()
	}
	v.isInput.Lock()
	defer v.isInput.Unlock()
	if v.inputReader == nil {
		v.inputReader = bufio.NewReader(v.Stdin)
	}
	return v.inputReader
}

// readLine girdi() için bir satır okur. Okuma yorumlayıcı kilidi bırakılarak yapılır;
// aynı anda okuyan görevler okuyucuyu sırayla kullanır.
func (v *VM) readLine() (string, error) {
	r := v.InputReader()
	var lineNo string
	var err error
	v.Blocking(func() {
		inputOrder.Lock()
		defer inputOrder.Unlock()
		lineNo, err = r.ReadString('\n')
	})
	return lineNo, err
}

// inputOrder aynı okuyucudan aynı anda okuyan görevleri sıraya koyar.
var inputOrder sync.Mutex
