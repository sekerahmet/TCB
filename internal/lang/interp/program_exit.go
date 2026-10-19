package interp

import (
	"errors"
	"fmt"
)

// Program çıkışı: bir yerleşik programın hemen, belirli bir çıkış koduyla bitmesini
// isteyebilir (ör. komut satırı seçeneklerinde --yardım yazılınca 0, hatalı bir argümanda
// 2). İstek bir hata gibi yukarı yayılır: yığın çözülürken 'ertele' blokları çalışır, ama
// 'dene/yakala' onu yakalamaz (kesilme gibi, bkz. interrupt.go). Run isteği döndürür;
// komut satırı Mesaj'ı yazar ve Kod ile çıkar.

// ExitRequest is a request to end the program. Message is text for a person (help or error,
// without a trailing newline); it goes to standard output when Code is 0, otherwise to
// standard error.
type ExitRequest struct {
	Code    int
	Message string
}

func (c *ExitRequest) Error() string {
	if c.Message == "" {
		return fmt.Sprintf("program %d koduyla bitirildi", c.Code)
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

// uncatchable dene/yakala'nın yakalamadığı hatalardır: kesilme ve program çıkışı isteği.
func uncatchable(err error) bool {
	if errors.Is(err, ErrInterrupted) {
		return true
	}
	_, ok := IsExitRequest(err)
	return ok
}
