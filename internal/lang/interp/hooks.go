package interp

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Bu dosyadaki arayüzler, VM dışında tanımlanan değerlerin (ör. internal/stdlib/ml'daki
// tensör) dilin işleçlerine, indekslemesine ve dış biçimlere dönüşümüne katılabilmesi için
// kancalardır. Bir değer arayüzü uygulamıyorsa VM'nin genel davranışı geçerlidir.

// Equaler gives a value its own equality rule for '==' and '!='. Values that do not implement
// it are compared by type and displayed text.
type Equaler interface {
	Equal(other Value) bool
}

// OperatorHook provides binary operators ('+', '-', '*', '/', '@', '**', '%', '<', '>', '<=',
// '>=' ...). op is the operator's text ("-", "**", "<" ...). With sagda=false the value is the
// left operand (value op diger), with sagda=true the right one (diger op value); the left
// operand's hook is tried first. If the operator is not supported it returns ok=false and the
// VM produces the generic error; a value that wants a clearer error returns ok=true with an
// error. '==' and '!=' do not go through this hook (see Equaler).
type OperatorHook interface {
	Operator(op string, other Value, onRight bool) (outcome Value, ok bool, err error)
}

// TaskOperatorHook is the form of OperatorHook that also receives the task doing the operation
// (the VM view). Operators whose result depends on task-specific state (e.g. tensor '@' inside
// a karışık_hassasiyet block) implement it; for such values it is called instead of Operator.
type TaskOperatorHook interface {
	TaskOperator(v *VM, op string, other Value, onRight bool) (outcome Value, ok bool, err error)
}

// Negatable provides unary minus ('-x').
type Negatable interface {
	Negate() (Value, error)
}

// Indexable provides reading 'x[i]' (not assignment). t[i][j] is two separate reads; the first
// read may itself return an Indexable value (e.g. a view).
type Indexable interface {
	Index(idx Value) (Value, error)
}

// IndexAssignable provides the assignment 'x[i] = value'. t[i][j] = value assigns with [j] to
// the value returned by reading t[i] (e.g. a view sharing the data). For compound assignment
// ('x[i] += 1') the VM first reads the current value with Index, applies the operation, and
// writes the result with SetIndex.
type IndexAssignable interface {
	SetIndex(idx Value, value Value) error
}

// MemberAssignable provides the assignment 'object.member = value' (e.g. an optimizer's
// learning rate). kd is the member's dictionary code (codes.None when the name is not a member
// name; resolved at compile time), ad the spelling the user wrote. For compound assignment
// ('op.öğrenme_oranı *= 0.5') the VM first reads the current value with GetProperty, applies the
// operation, and writes the result with SetMember. An unassignable member or an unsuitable
// value is an error (the VM adds the line).
type MemberAssignable interface {
	SetMember(kd codes.Code, name string, value Value) error
}

// Iterable provides the items to visit for 'döngü x içinde value', 'döngü i, x içinde value'
// and list comprehensions ('[expr döngü x içinde value]'). IterLen returns the number of items
// (or an error when the value cannot be looped over); for each round the VM asks for item i
// with IterItem(i), 0 ≤ i < length. The length is read once when the loop starts.
type Iterable interface {
	IterLen() (int, error)
	IterItem(i int) (Value, error)
}

// Stream is a lazy (streaming) source for 'döngü x içinde value', 'döngü i, x içinde value'
// and list comprehensions: the number of items is not known in advance and items are produced
// on demand (e.g. a dataset's sequential reader, later file lines). The VM calls Next for each
// round; devam=false means the source is exhausted. A loop does not restart the source: it
// consumes from where it stopped (a second loop over the same stream visits the remaining
// items). If Next will wait long (disk, network) it releases the interpreter lock with
// v.Blocking. A value that is both Iterable and Stream is used as Iterable.
type Stream interface {
	Next(v *VM) (value Value, cont bool, err error)
}

// PlainValuer returns the plain TR value that stands in for a value when it is written to
// external formats (JSON, Go values), e.g. tensor → nested list of Ondalık.
type PlainValuer interface {
	PlainValue() Value
}

// LineError is a runtime error with line information added. Errors from builtins and external
// values (e.g. tensors) are wrapped with it; an error that already carries a line (e.g. from a
// function a builtin called back) is not wrapped twice, so the message and code frame show
// the line of the original error.
type LineError struct {
	Line int
	Err  error
}

func (h *LineError) Error() string { return fmt.Sprintf("Satır %d: %v", h.Line, h.Err) }
func (h *LineError) Unwrap() error { return h.Err }

// hasLine hata zaten satır bilgisi taşıyor mu? Tipli hata önceliklidir; VM'nin metinle
// ürettiği "Satır N: ..." hataları için metin öneki yedek olarak denetlenir.
func hasLine(err error) bool {
	var sh *LineError
	var mh *ModuleError
	return errors.As(err, &sh) || errors.As(err, &mh) || strings.HasPrefix(err.Error(), "Satır ")
}

// lineSuffix hatayı (satır bilgisi yoksa) "Satır N: " önekli bir SatirliHata'ya çevirir.
func lineSuffix(line int, err error) error {
	if err == nil || line <= 0 || hasLine(err) {
		return err
	}
	return &LineError{Line: line, Err: err}
}

// hookOperator ikili işlemi (varsa) değerlerin işleç kancalarıyla yapar.
func (v *VM) hookOperator(op string, left, right Value, line int) (Value, bool, error) {
	if k, ok := left.(TaskOperatorHook); ok {
		if val, ok, err := k.TaskOperator(v, op, right, false); ok {
			return val, true, lineSuffix(line, err)
		}
	} else if k, ok := left.(OperatorHook); ok {
		if val, ok, err := k.Operator(op, right, false); ok {
			return val, true, lineSuffix(line, err)
		}
	}
	if k, ok := right.(TaskOperatorHook); ok {
		if val, ok, err := k.TaskOperator(v, op, left, true); ok {
			return val, true, lineSuffix(line, err)
		}
	} else if k, ok := right.(OperatorHook); ok {
		if val, ok, err := k.Operator(op, left, true); ok {
			return val, true, lineSuffix(line, err)
		}
	}
	return nil, false, nil
}
