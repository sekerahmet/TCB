package codeindex

import (
	"fmt"
	"path/filepath"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// SkippedMsg atlanan dosyanın tanısını dilden bağımsız olarak tutar: isteğe bağlı satır/sütun konumu, ileti kodu ve
// parametre değerleri. Metin önbelleğe yazılmaz; okuyanın etkin dilinde (TR_LANG) yeniden kurulur.
type SkippedMsg struct {
	Line int          `json:"line,omitempty"`
	Col  int          `json:"col,omitempty"`
	Code string       `json:"code"`
	Args []SkippedArg `json:"args,omitempty"`
}

// SkippedArg bir tanı iletisinin tek parametresidir: iç ileti (Msg ya da NilMsg), kod başvurusu (Ref), düz ya da
// olduğu gibi korunan metin (Str, Verbatim) ya da tam sayı (Int).
type SkippedArg struct {
	Msg      *SkippedMsg `json:"msg,omitempty"`
	NilMsg   bool        `json:"nil_msg,omitempty"`
	Ref      *SkippedRef `json:"ref,omitempty"`
	Str      *string     `json:"str,omitempty"`
	Verbatim *string     `json:"verbatim,omitempty"`
	Int      *int64      `json:"int,omitempty"`
}

// SkippedRef iletideki dil öğesi başvurusudur (codes.Ref).
type SkippedRef struct {
	Code     string `json:"code"`
	Form     uint8  `json:"form,omitempty"`
	Receiver string `json:"receiver,omitempty"`
}

// skippedVerbatim metni ileti şablonuna olduğu gibi geçirir (lexer.verbatim karşılığı).
type skippedVerbatim string

func (v skippedVerbatim) MessageText(string) string { return string(v) }
func (v skippedVerbatim) String() string            { return string(v) }

func newSkipped(rel string, err error) Skipped {
	line, col, msg := unwrapSkipped(err)
	s := Skipped{File: rel}
	if msg != nil {
		s.Message = encodeSkippedMsg(line, col, msg)
		s.Reason = s.Message.reason()
		return s
	}
	s.Reason = firstLine(err)
	return s
}

func newSkippedRead(root, rel string, readErr error) Skipped {
	fe := turkish.FileError(nil, filepath.Join(root, filepath.FromSlash(rel)), messages.H2100, readErr)
	if m, ok := messages.Root(fe); ok {
		msg := encodeSkippedMsg(0, 0, m)
		return Skipped{File: rel, Message: msg, Reason: msg.reason()}
	}
	return newSkipped(rel, fe)
}

func unwrapSkipped(err error) (line, col int, msg *messages.Error) {
	for err != nil {
		switch e := err.(type) {
		case *messages.PosError:
			if line == 0 && e.Line > 0 {
				line, col = e.Line, e.Col
			}
			err = e.Err
		case *messages.Error:
			return line, col, e
		case interface{ Unwrap() error }:
			err = e.Unwrap()
		default:
			return line, col, nil
		}
	}
	return 0, 0, nil
}

func encodeSkippedMsg(line, col int, e *messages.Error) *SkippedMsg {
	if e == nil {
		return nil
	}
	out := &SkippedMsg{Line: line, Col: col, Code: e.Code.String()}
	if len(e.Args) > 0 {
		out.Args = make([]SkippedArg, len(e.Args))
		for i, a := range e.Args {
			out.Args[i] = encodeSkippedArg(a)
		}
	}
	return out
}

func encodeSkippedArg(a any) SkippedArg {
	switch v := a.(type) {
	case *messages.Error:
		if v == nil {
			return SkippedArg{NilMsg: true}
		}
		return SkippedArg{Msg: encodeSkippedMsg(0, 0, v)}
	case *messages.PosError:
		if v == nil {
			return SkippedArg{NilMsg: true}
		}
		line, col, inner := unwrapSkipped(v)
		return SkippedArg{Msg: encodeSkippedMsg(line, col, inner)}
	case codes.Ref:
		ref := &SkippedRef{Code: v.Code.String(), Form: uint8(v.Form)}
		if v.Receiver != codes.None {
			ref.Receiver = v.Receiver.String()
		}
		return SkippedArg{Ref: ref}
	case string:
		s := v
		return SkippedArg{Str: &s}
	case int:
		n := int64(v)
		return SkippedArg{Int: &n}
	case int64:
		n := v
		return SkippedArg{Int: &n}
	case uintptr:
		n := int64(v)
		return SkippedArg{Int: &n}
	case messages.Localized:
		s := v.MessageText(codes.SourceLanguage)
		return SkippedArg{Verbatim: &s}
	}
	s := fmt.Sprint(a)
	return SkippedArg{Str: &s}
}

func (m *SkippedMsg) err() error {
	if m == nil {
		return nil
	}
	code, err := messages.Parse(m.Code)
	if err != nil {
		return nil
	}
	args := make([]any, len(m.Args))
	for i, a := range m.Args {
		args[i] = a.val()
	}
	var out error = messages.New(code, args...)
	if m.Line > 0 {
		out = messages.At(m.Line, m.Col, out)
	}
	return out
}

func (m *SkippedMsg) reason() string {
	if err := m.err(); err != nil {
		return firstLine(err)
	}
	return ""
}

func (a SkippedArg) val() any {
	switch {
	case a.NilMsg:
		return (*messages.Error)(nil)
	case a.Msg != nil:
		return a.Msg.err()
	case a.Ref != nil:
		k, _ := codes.Parse(a.Ref.Code)
		var recv codes.Code
		if a.Ref.Receiver != "" {
			recv, _ = codes.Parse(a.Ref.Receiver)
		}
		return codes.Ref{Code: k, Form: codes.RefForm(a.Ref.Form), Receiver: recv}
	case a.Verbatim != nil:
		return skippedVerbatim(*a.Verbatim)
	case a.Str != nil:
		return *a.Str
	case a.Int != nil:
		return *a.Int
	}
	return ""
}

func (idx *Index) fillSkippedReasons() {
	for i := range idx.Skipped {
		if idx.Skipped[i].Message != nil {
			idx.Skipped[i].Reason = idx.Skipped[i].Message.reason()
		}
	}
}
