package httpserver

import (
	"net/http"
	"net/textproto"
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// ResponseType TR_Response'un tip adıdır.
var ResponseType = interp.TypeName(codes.TypeResponse)

// Response işleyicinin cevabıdır (28.3): bir değerdir, yazıcı nesnesi yoktur. Okunur üyeleri durum
// kodu, ağa gidecek gövde baytları ve varsayılan Content-Type'ı da taşıyan başlıklardır.
type Response struct {
	status  int
	body    []byte
	headers *Headers
}

func (r *Response) Type() interp.ValueType { return ResponseType }
func (r *Response) IsTruthy() bool         { return true }

// Inspect değerin metnidir (3.14): tipin adı.
func (r *Response) Inspect() string { return codes.ValueNames().Name(codes.TypeResponse) }

var responseMembers = []codes.Code{codes.MemStatus, codes.MemBody, codes.MemHeaders}

// GetProperty cevabın okunur üyelerini verir: ağsız TR testi cevabı böyle sınar.
func (r *Response) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	switch kd {
	case codes.MemStatus:
		return interp.NewInt(int64(r.status)), nil
	case codes.MemBody:
		return interp.NewBytes(r.body), nil
	case codes.MemHeaders:
		return r.headers, nil
	}
	return nil, bind.MemberError(ResponseType, name, responseMembers)
}

var (
	nameStatus = bind.NamedArg(codes.ArgStatus)
	nameJSON   = bind.NamedArg(codes.ArgJson)
)

// newResponse TR_Response(TR_status: 200, TR_body: ..., TR_json: ..., TR_headers: {...})'dır (28.3).
func newResponse(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.TypeResponse)
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.TypeResponse, 0, len(args))
	}
	if err := bind.CheckNamed(fn, named, nameStatus, nameBody, nameJSON, nameHeaders); err != nil {
		return nil, err
	}
	r := &Response{status: http.StatusOK, headers: newHeaders()}
	if v, ok := bind.GetNamed(named, nameStatus); ok {
		n, ok := v.(*interp.IntVal)
		if !ok {
			return nil, namedTypeError(fn, nameStatus, v, codes.TypeInt)
		}
		if n.Val < 200 || n.Val > 599 {
			return nil, messages.New(messages.H7607, fn, n.Val)
		}
		r.status = int(n.Val)
	}
	body, hasBody := bind.GetNamed(named, nameBody)
	value, hasJSON := bind.GetNamed(named, nameJSON)
	contentType := ""
	switch {
	case hasBody && hasJSON:
		return nil, messages.New(messages.H7608, fn)
	case hasJSON:
		data, err := jsonwrite.Write(value)
		if err != nil {
			return nil, err
		}
		r.body, contentType = data, "application/json"
	case hasBody:
		switch b := body.(type) {
		case *interp.StringVal:
			r.body, contentType = []byte(b.Val), "text/plain; charset=utf-8"
		case *interp.BytesVal:
			r.body, contentType = b.Data(), "application/octet-stream"
		default:
			return nil, namedTypeError(fn, nameBody, body, codes.TypeString, codes.TypeBytes)
		}
	}
	if v, ok := bind.GetNamed(named, nameHeaders); ok {
		var err error
		if r.headers, err = headersArg(fn, v); err != nil {
			return nil, err
		}
	}
	if _, given := r.headers.get("Content-Type"); !given && contentType != "" {
		r.headers.add("Content-Type", contentType)
	}
	return r, nil
}

// headersArg TR_headers: argümanıdır: metin anahtarlı, metin değerli sözlük. Go'nun sessizce sildiği
// ya da değiştirdiği her başlık burada hatadır (28.3): geçersiz ad, harf büyüklüğü farklı aynı ad,
// satır sonu ya da denetim karakteri taşıyan değer.
func headersArg(fn any, v interp.Value) (*Headers, error) {
	d, ok := v.(*interp.DictVal)
	if !ok {
		return nil, namedTypeError(fn, nameHeaders, v, codes.TypeDict)
	}
	h := newHeaders()
	for _, k := range d.Order {
		if _, keyed := d.KeyVals[k]; keyed {
			return nil, namedTypeError(fn, nameHeaders, d.KeyValue(k), codes.TypeString)
		}
		s, ok := d.Pairs[k].(*interp.StringVal)
		if !ok {
			return nil, namedTypeError(fn, nameHeaders, d.Pairs[k], codes.TypeString)
		}
		switch first, dup := h.sameName(k); {
		case !isToken(k):
			return nil, messages.New(messages.H7609, fn, k)
		case dup:
			return nil, messages.New(messages.H7610, fn, first, k)
		case hasControl(s.Val):
			return nil, messages.New(messages.H7611, fn, k)
		}
		h.add(k, s.Val)
	}
	return h, nil
}

// sameName harf büyüklüğüne bakmadan aynı adlı başlık varsa onun yazımını verir.
func (h *Headers) sameName(name string) (string, bool) {
	if _, ok := h.get(name); !ok {
		return "", false
	}
	for _, n := range h.names {
		if textproto.CanonicalMIMEHeaderKey(n) == textproto.CanonicalMIMEHeaderKey(name) {
			return n, true
		}
	}
	return name, true
}

// hasControl değer satır sonu ya da başka bir denetim karakteri (C0, DEL) taşıyor mu?
func hasControl(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

// write cevabı yazar: başlıklar, işleyici vermedikçe nosniff, gövdeden hesaplanan Content-Length ve
// gövde; HEAD'de gövde gönderilmez (RFC 9110, 9.3.2). Adlar Go'nun anahtar biçimine çevrilir: Go
// Content-Type'ı o biçimle arar, bulamazsa gövdeyi koklayıp ikinci bir Content-Type ekler.
func (r *Response) write(w http.ResponseWriter, head bool) {
	out := w.Header()
	for _, n := range r.headers.names {
		v, _ := r.headers.get(n)
		out[textproto.CanonicalMIMEHeaderKey(n)] = []string{v}
	}
	if _, given := r.headers.get("X-Content-Type-Options"); !given {
		out["X-Content-Type-Options"] = []string{"nosniff"}
	}
	allowed := r.status != http.StatusNoContent && r.status != http.StatusNotModified
	if allowed {
		out["Content-Length"] = []string{strconv.Itoa(len(r.body))}
	}
	w.WriteHeader(r.status)
	if allowed && !head {
		_, _ = w.Write(r.body) // kopan istemci işleyicinin hatası değildir (28.9)
	}
}

var _ interp.PropertyGetter = (*Response)(nil)
