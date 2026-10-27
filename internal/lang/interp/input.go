package interp

import (
	"bufio"
	"os"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
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

// lineRead başlamış bir satır okumasıdır; bitince done kapanır.
type lineRead struct {
	done chan struct{}
	line string
	err  error
}

// readLine girdi() için bir satır okur. Okuma yorumlayıcı kilidi bırakılarak yapılır ve bir
// durdurma onu bırakabilir (1.5); bırakılan okuma sürer ve okuduğu satırı bir sonraki okuyan
// alır, satır yitmez (ertelenen bir iş de girdi isteyebilir). Aynı anda okuyan görevler
// okumayı paylaşır: biri satırı alır, öbürü yeni okuma başlatır. waitErr üstlenilmiş programda
// "durdurma istendi" hatasıdır (1.6).
func (v *VM) readLine() (line string, readErr, waitErr error) {
	r := v.InputReader()
	for {
		rd := v.startLineRead(r)
		if err := v.awaitReady(CallPrefix(codes.FnInput), rd.done, true); err != nil {
			return "", nil, err
		}
		v.isInput.Lock()
		mine := v.pendingRead == rd
		if mine {
			v.pendingRead = nil
		}
		v.isInput.Unlock()
		if mine {
			return rd.line, rd.err, nil
		}
	}
}

// startLineRead sürmekte olan satır okumasını verir; yoksa r'den yenisini başlatır.
func (v *VM) startLineRead(r *bufio.Reader) *lineRead {
	v.isInput.Lock()
	defer v.isInput.Unlock()
	if v.pendingRead == nil {
		rd := &lineRead{done: make(chan struct{})}
		v.pendingRead = rd
		go func() {
			rd.line, rd.err = r.ReadString('\n')
			close(rd.done)
		}()
	}
	return v.pendingRead
}
