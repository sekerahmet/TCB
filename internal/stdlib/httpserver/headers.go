package httpserver

import (
	"net/http"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// HeadersType TR_Headers'ın tip adıdır.
var HeadersType = interp.TypeName(codes.TypeHeaders)

// Headers isteğin ya da cevabın başlıklarıdır (28.3): ad harf büyüklüğüne bakmadan bulunur, aynı
// başlığın değerleri gelişi sırasıyla virgülle birleşir.
type Headers struct {
	names []string          // ilk geldiği yazımıyla, geliş sırasıyla
	vals  map[string]string // küçük harfli ad -> değer
}

func newHeaders() *Headers { return &Headers{vals: map[string]string{}} }

func (h *Headers) Type() interp.ValueType { return HeadersType }
func (h *Headers) IsTruthy() bool         { return len(h.names) > 0 }

// Inspect değerin metnidir (3.14): tipin adı.
func (h *Headers) Inspect() string { return codes.ValueNames().Name(codes.TypeHeaders) }

// add başlığı ekler; aynı ad (harf büyüklüğüne bakmadan) varsa değeri virgülle eklenir.
func (h *Headers) add(name, value string) {
	k := strings.ToLower(name)
	if old, ok := h.vals[k]; ok {
		h.vals[k] = old + ", " + value
		return
	}
	h.names = append(h.names, name)
	h.vals[k] = value
}

func (h *Headers) get(name string) (string, bool) {
	v, ok := h.vals[strings.ToLower(name)]
	return v, ok
}

// headersOf Go'nun ayrıştırdığı isteğin başlıklarıdır; Host da başlıklardadır (28.3).
func headersOf(r *http.Request) *Headers {
	h := newHeaders()
	h.add("Host", r.Host)
	keys := make([]string, 0, len(r.Header))
	for k := range r.Header {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		for _, v := range r.Header[k] {
			h.add(k, v)
		}
	}
	return h
}

// Index h["ad"]'dır: olmayan başlık sözlükteki gibi hatadır, ileti h.TR_get(ad)'ı önerir.
func (h *Headers) Index(idx interp.Value) (interp.Value, error) {
	s, ok := idx.(*interp.StringVal)
	if !ok {
		return nil, messages.New(messages.H7602, interp.CallPrefix(codes.TypeHeaders), idx.Type())
	}
	v, ok := h.get(s.Val)
	if !ok {
		return nil, messages.New(messages.H7603, interp.CallPrefix(codes.TypeHeaders), s.Val)
	}
	return &interp.StringVal{Val: v}, nil
}

// GetProperty h.TR_get(ad)'ı verir: olmayan başlıkta TR_none (13.4).
func (h *Headers) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	if kd != codes.MemGet {
		return nil, bind.MemberError(HeadersType, name, []codes.Code{codes.MemGet})
	}
	return bind.CodeMethod(kd, true, func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.MemGet, 1, len(args))
		}
		s, ok := args[0].(*interp.StringVal)
		if !ok {
			return nil, interp.ArgTypeError(codes.MemGet, 1, args[0], codes.TypeString)
		}
		if v, ok := h.get(s.Val); ok {
			return &interp.StringVal{Val: v}, nil
		}
		return interp.Null, nil
	}), nil
}

var (
	_ interp.PropertyGetter = (*Headers)(nil)
	_ interp.Indexable      = (*Headers)(nil)
)
