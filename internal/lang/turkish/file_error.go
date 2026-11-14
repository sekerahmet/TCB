package turkish

import (
	"errors"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// fileError işletim sisteminin dosya hatasını kullanıcının dilinde anlatır. İleti kodludur
// (messages.Has ile bulunur); özgün hata da Unwrap ile korunur: errors.Is(err,
// os.ErrNotExist) gibi denetimler çalışmaya devam eder.
type fileError struct {
	message *messages.Error
	extras  []*messages.Error
	orig    error
}

func (h *fileError) Error() string { return h.text((*messages.Error).Error) }

// MessageText hatanın lang dilindeki metnidir (messages.Localized): ileti ve ekleri yakalayan
// dosyanın dilinde yazılır.
func (h *fileError) MessageText(lang string) string {
	return h.text(func(m *messages.Error) string { return messages.ErrorIn(m, lang) })
}

// text iletiyi ve eklerini write ile yazar ("; " ile ayrılmış).
func (h *fileError) text(write func(*messages.Error) string) string {
	s := write(h.message)
	for _, e := range h.extras {
		s += "; " + write(e)
	}
	return s
}

func (h *fileError) Unwrap() []error { return []error{h.message, h.orig} }

// Reason is a file error carrying its own reason as a coded message (cases that do not come
// from the operating system: "uzantısı .gz ama içerik gzip değil", "salt okunur dosya" ...).
// FileError writes it as the reason.
type Reason struct{ Message *messages.Error }

// NewReason returns the reason message c with its parameter values.
func NewReason(c messages.Code, args ...any) Reason { return Reason{messages.New(c, args...)} }

func (n Reason) Error() string { return n.Message.Error() }

// MessageText nedenin lang dilindeki metnidir (messages.Localized).
func (n Reason) MessageText(lang string) string { return messages.ErrorIn(n.Message, lang) }
func (n Reason) Unwrap() error                  { return n.Message }

// FileError turns the error of a file operation into a message for the user in the active
// language; the operating system's English text ("The system cannot find the file
// specified") never reaches the user. The message is a single line:
//
//	<fn>: "<yol>" <islem>: <neden>[; <ek>...]
//
// fn is the builtin that failed (e.g. "dosya_oku()"; no prefix when empty), yol is the
// path as the user wrote it, op is the outcome as a whole-sentence message code with the
// parameters path and reason (messages.H2100 "{path} okunamadı: {reason}", messages.H2101 ...).
// suffixes (message parts, nil skipped) are appended after the reason with "; " (e.g. "bakılan
// yerler: ..."); like the message they are written in the language of the message. Known cases:
// file or folder missing, permission denied, path is a folder, disk full, file in use,
// invalid name, path too long, write-protected disk, unexpected EOF (H2153). An unknown
// operating system error gives its number only (H2628), any other unknown error none (H2629):
// the text of the operating system or of Go never enters the value (8.5), it differs from
// machine to machine; the original error stays reachable with Unwrap.
//
// Returns nil when err is nil. Standard library packages (tensör_kaydet/yükle,
// parçalayıcı_yükle, veri_kümesi) must report file errors through it as well.
func FileError(fn any, path string, op messages.Code, err error, suffixes ...*messages.Error) error {
	if err == nil {
		return nil
	}
	// İşlem kodu kendi tam cümlesidir ({path}, {reason}); fn varsa ortak "{fn}: {error}" iletisi
	// (H2000) onu sarar. İki kod da zincirde bulunur (messages.Has).
	var m *messages.Error = messages.New(op, path, fileErrorReason(path, err))
	if fn != nil && fn != "" {
		m = messages.New(messages.H2000, fn, m)
	}
	var extras []*messages.Error
	for _, suffix := range suffixes {
		if suffix != nil {
			extras = append(extras, suffix)
		}
	}
	return &fileError{message: m, extras: extras, orig: err}
}

// FileErrorReason returns only the reason sentence of the error in the active language (e.g.
// "böyle bir dosya ya da klasör yok"). For callers that build their own message.
func FileErrorReason(path string, err error) string {
	if err == nil {
		return ""
	}
	return fileErrorReason(path, err).Error()
}

// fileErrorReason nedenin kodlu iletisidir; FileError onu iletinin içinde taşır (messages.Has
// nedenin kodunu da bulur).
func fileErrorReason(path string, err error) *messages.Error {
	var n Reason
	if errors.As(err, &n) {
		return n.Message
	}
	return messages.SystemReason(path, err)
}
