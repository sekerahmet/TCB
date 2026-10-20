package codes

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// internalf reports a malformed embedded list (code list, definitions file) as an internal
// error: only a bug in TR itself can make an embedded file invalid, so the user sees the
// shared "please report" message and the English detail is for developers.
func internalf(form string, a ...any) error {
	return messages.Internalf(form, a...)
}
