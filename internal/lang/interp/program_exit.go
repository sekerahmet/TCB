package interp

import (
	"errors"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Program çıkışı: TR_exit(kod) ve komut satırı çözümleyicisi (--yardım: 0, hatalı argüman: 2)
// programın hemen, belirli bir çıkış koduyla bitmesini ister (1.10). İstek bir hata gibi yukarı
// yayılır: yığın çözülürken 'ertele' blokları çalışır, ama 'dene/yakala' onu yakalamaz (durdurma
// gibi, bkz. interrupt.go). Kodun kendisi hatanın içinde değil VM'dedir (RequestExit): bir
// ertele bloğunda yeniden çağrılan TR_exit kodu değiştirir, son çağrı kazanır (1.13). Sürecin
// nasıl biteceğini Run'dan sonra End söyler.

// ExitRequest is a request to end the program. Message is text for a person (help or error,
// without a trailing newline); it goes to standard output when Code is 0, otherwise to
// standard error.
type ExitRequest struct {
	Code    int
	Message string
	inTest  bool // bir testin içinden istendi (1.12): süreç bitmez, test düşer
}

func (c *ExitRequest) Error() string {
	if c.inTest {
		return messages.Text(messages.H2532, CallName(codes.FnExit), c.Code, codes.NameRef(codes.FnAssert))
	}
	if c.Message == "" {
		return messages.Text(messages.H2491, c.Code)
	}
	return c.Message
}

// IsExitRequest reports whether err (or an error it wraps) is a program exit request.
func IsExitRequest(err error) (*ExitRequest, bool) {
	var c *ExitRequest
	if errors.As(err, &c) {
		return c, true
	}
	return nil, false
}

// uncatchable dene/yakala'nın yakalamadığı hatalardır: durdurma ve program çıkışı isteği.
func uncatchable(err error) bool {
	if errors.Is(err, ErrInterrupted) {
		return true
	}
	_, ok := IsExitRequest(err)
	return ok
}

// RequestExit programın code koduyla bitmesini ister; dönen hatayı yukarı yaymak çağıranın
// işidir. message boş değilse süreç bitmeden yazılır (komut satırı yardımı, kullanım hatası).
// Öbür görevler 1.5'teki gibi durdurulur (1.10). Bir testin içinden çağrılırsa süreç bitmez:
// dönen hata testi düşürür (1.12).
func (v *VM) RequestExit(code int, message string) *ExitRequest {
	if v.testing.Load() > 0 {
		return &ExitRequest{Code: code, Message: message, inTest: true}
	}
	v.exitMu.Lock()
	v.exitRequested, v.exitCode = true, code
	if message != "" {
		v.exitMessage = message
	}
	v.exitMu.Unlock()
	v.Interrupt()
	return &ExitRequest{Code: code, Message: message}
}

// maxExitCode TR_exit'in en büyük kodudur: POSIX süreç kodunun alt 8 bitini görür, 126'dan sonrası
// kabuğa aittir (1.11).
const maxExitCode = 125

// exitBuiltin sonlandır(kod) yerleşiğidir (1.10, 1.11).
func exitBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnExit, 1, len(args))
	}
	n, ok := args[0].(*IntVal)
	if !ok {
		return nil, ArgTypeError(codes.FnExit, 1, args[0], codes.TypeInt)
	}
	if n.Val < 0 || n.Val > maxExitCode {
		return nil, messages.New(messages.H2531, CallPrefix(codes.FnExit), n.Val, codes.NameRef(codes.FnExit))
	}
	return nil, y.RequestExit(int(n.Val), "")
}

// End programın nasıl bittiğini söyler (1.9, 1.13, 1.14): sürecin çıkış kodu ya da sinyali ve
// yazılacak hatalar.
type End struct {
	// Signal sıfırdan büyükse süreç bu sinyalle biter (isteğin sinyali); değilse Code ile çıkar.
	Signal int
	Code   int
	// Errors yazılacak hatalardır: programın yakalanmayan hatası ve biterken çalışan ertelenen
	// işlerin hataları. Durdurma ve TR_exit istekleri hata değildir, burada yer almaz.
	Errors []error
	// Message programın istediği çıkışın iletisidir (komut satırı yardımı, kullanım hatası);
	// MessageToOutput iletinin standart çıktıya (başarılı çıkış) yazılacağını söyler, yoksa
	// standart hataya.
	Message         string
	MessageToOutput bool
}

// End, Run'ın döndürdüğü err ile VM'nin durdurma ve çıkış durumundan sürecin nasıl biteceğini
// çıkarır:
//   - Üstlenmemiş programa istek geldiyse süreç isteğin sinyaliyle biter, kod değişmez.
//   - Ertelenen işin ya da bir görevin hatası, TR_exit çağırmamış bir programın hatası: 1.
//   - TR_exit çağrıldıysa onun (son çağrının) kodu.
//   - Üstlenen program sona geldiyse isteğin sinyali; istek yoksa 0.
func (v *VM) End(err error) End {
	deferred, main := splitProgramErrors(err)
	v.exitMu.Lock()
	exited, code, message := v.exitRequested, v.exitCode, v.exitMessage
	v.exitMu.Unlock()

	end := End{Message: message, MessageToOutput: code == 0}
	if main != nil {
		end.Errors = append(end.Errors, main)
	}
	end.Errors = append(end.Errors, deferred...)

	signal := v.StopSignal()
	failed := len(deferred) > 0 || v.taskFailed.Load() || (main != nil && !exited)
	switch {
	case signal != 0 && !v.stopHandled.Load():
		end.Signal = signal
	case failed:
		end.Code = 1
	case exited:
		end.Code = code
	case signal != 0:
		end.Signal = signal
	}
	return end
}

// splitProgramErrors Run'ın hatasını programın yakalanmayan hatasına (durdurma ve TR_exit
// istekleri hata sayılmaz) ve ertelenen işlerin hatalarına ayırır.
func splitProgramErrors(err error) (deferred []error, main error) {
	if err == nil {
		return nil, nil
	}
	var d *deferError
	if errors.As(err, &d) {
		if !silentEnd(d.mainSrc) {
			main = d.mainSrc
		}
		return d.extras, main
	}
	if silentEnd(err) {
		return nil, nil
	}
	return nil, err
}

// silentEnd hatanın bir hata olmadan bitiş isteği olduğunu söyler: durdurma ya da TR_exit. Bir
// testin içinden çağrılan TR_exit bir hatadır (1.12).
func silentEnd(err error) bool {
	if c, ok := IsExitRequest(err); ok && c.inTest {
		return false
	}
	return uncatchable(err)
}

// taskError bir eşzamanlı görevin bitişinde bildirilecek hatayı verir: durdurma ya da TR_exit
// yüzünden bitmek bir hata değildir; yalnız ertelenen işlerin hataları kalır.
func taskError(err error) error {
	deferred, main := splitProgramErrors(err)
	switch {
	case main != nil && len(deferred) == 0:
		return main
	case main != nil:
		return &deferError{mainSrc: main, extras: deferred}
	case len(deferred) == 0:
		return nil
	}
	return &deferError{mainSrc: deferred[0], extras: deferred[1:]}
}
