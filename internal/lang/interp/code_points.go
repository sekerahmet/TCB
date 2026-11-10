package interp

import (
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TR_ord ve TR_chr (13.1): tek karakterlik metnin Unicode kod noktası ve kod noktasının tek
// karakterlik metni. Karakter kod noktasıdır (metnin uzunluğu gibi, 3.3): birleşik yazılmış "é" (e ve
// birleştirici işaret) iki karakterdir. Kod noktası bayt değildir: TR_ord("ş") 351, TR_Bytes("ş")
// iki bayt (3.3a).

// ordBuiltin TR_ord(k).
func ordBuiltin(_ *VM, args []Value, _ map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnOrd, 1, len(args))
	}
	s, ok := args[0].(*StringVal)
	if !ok {
		return nil, ArgTypeError(codes.FnOrd, 1, args[0], codes.TypeString)
	}
	if n := utf8.RuneCountInString(s.Val); n != 1 {
		return nil, messages.New(messages.H2559, s.Val, n)
	}
	r, _ := utf8.DecodeRuneInString(s.Val)
	return NewInt(int64(r)), nil
}

// chrBuiltin TR_chr(n): n bir Unicode skaler değeri olmalıdır (2.6'nın \u{...} kümesi: 0–0x10FFFF,
// 0xD800–0xDFFF vekil aralığı hariç).
func chrBuiltin(_ *VM, args []Value, _ map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnChr, 1, len(args))
	}
	n, ok := args[0].(*IntVal)
	if !ok {
		return nil, ArgTypeError(codes.FnChr, 1, args[0], codes.TypeInt)
	}
	if n.Val < 0 || n.Val > utf8.MaxRune || n.Val >= 0xD800 && n.Val <= 0xDFFF {
		return nil, messages.New(messages.H2560, n.Val)
	}
	return &StringVal{Val: string(rune(n.Val))}, nil
}
