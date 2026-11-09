package httpserver

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// RequestType TR_Request'in tip adıdır.
var RequestType = interp.TypeName(codes.TypeRequest)

// Request işleyiciye verilen istektir (28.3). Sunucu kurar ya da test TR_Request(...) ile kurar.
type Request struct {
	method, path string
	query        *interp.DictVal // TR_Dict[TR_String, TR_List[TR_String]]
	headers      *Headers
	body         []byte
}

func (r *Request) Type() interp.ValueType { return RequestType }
func (r *Request) IsTruthy() bool         { return true }

// Inspect değerin metnidir (3.14): tipin adı.
func (r *Request) Inspect() string { return codes.ValueNames().Name(codes.TypeRequest) }

var requestMembers = []codes.Code{codes.MemMethod, codes.MemPath, codes.MemQuery, codes.MemHeaders, codes.MemBody}

// GetProperty isteğin okunur üyelerini verir.
func (r *Request) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	switch kd {
	case codes.MemMethod:
		return &interp.StringVal{Val: r.method}, nil
	case codes.MemPath:
		return &interp.StringVal{Val: r.path}, nil
	case codes.MemQuery:
		return r.query, nil
	case codes.MemHeaders:
		return r.headers, nil
	case codes.MemBody:
		return interp.NewBytes(r.body), nil
	}
	return nil, bind.MemberError(RequestType, name, requestMembers)
}

// Adlı argümanlar; adları kod listesindedir.
var (
	nameMethod  = bind.NamedArg(codes.ArgMethod)
	namePath    = bind.NamedArg(codes.ArgPath)
	nameQuery   = bind.NamedArg(codes.ArgQuery)
	nameHeaders = bind.NamedArg(codes.ArgHeaders)
	nameBody    = bind.NamedArg(codes.ArgBody)
)

// newRequest TR_Request(TR_method: "GET", TR_path: "/", TR_query: {}, TR_headers: {}, TR_body: ...)'dır:
// işleyici ağsız sınanır (28.3). Argümanların hepsi adlı ve varsayılanlıdır.
func newRequest(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.TypeRequest)
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.TypeRequest, 0, len(args))
	}
	if err := bind.CheckNamed(fn, named, nameMethod, namePath, nameQuery, nameHeaders, nameBody); err != nil {
		return nil, err
	}
	r := &Request{method: "GET", path: "/", query: interp.NewDictVal(), headers: newHeaders()}
	var err error
	if v, ok := bind.GetNamed(named, nameMethod); ok {
		if r.method, err = textArg(fn, nameMethod, v); err != nil {
			return nil, err
		}
		if err := methodError(fn, r.method); err != nil {
			return nil, err
		}
	}
	if v, ok := bind.GetNamed(named, namePath); ok {
		if r.path, err = textArg(fn, namePath, v); err != nil {
			return nil, err
		}
	}
	if v, ok := bind.GetNamed(named, nameQuery); ok {
		if r.query, err = queryArg(fn, v); err != nil {
			return nil, err
		}
	}
	if v, ok := bind.GetNamed(named, nameHeaders); ok {
		if r.headers, err = headersArg(fn, v); err != nil {
			return nil, err
		}
	}
	if v, ok := bind.GetNamed(named, nameBody); ok {
		b, ok := v.(*interp.BytesVal)
		if !ok {
			return nil, namedTypeError(fn, nameBody, v, codes.TypeBytes)
		}
		r.body = b.Data()
	}
	return r, nil
}

// methodError yöntem RFC 9110'un token'ı değilse ya da HEAD ise hatadır (28.3).
func methodError(fn any, method string) error {
	if !isToken(method) {
		return messages.New(messages.H7606, fn, method)
	}
	if method == "HEAD" {
		return messages.New(messages.H7605, fn)
	}
	return nil
}

// isToken s RFC 9110'un token'ı mı: boş olmayan, ASCII harf, rakam ve !#$%&'*+-.^_`|~.
func isToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c < 0x80 && isTokenSymbol(c)) {
			return false
		}
	}
	return true
}

func isTokenSymbol(c byte) bool {
	for i := 0; i < len(tokenSymbols); i++ {
		if tokenSymbols[i] == c {
			return true
		}
	}
	return false
}

const tokenSymbols = "!#$%&'*+-.^_`|~"

// queryArg TR_query: argümanıdır: metin anahtarlı, metin listesi değerli sözlük; kopyası alınır.
func queryArg(fn any, v interp.Value) (*interp.DictVal, error) {
	d, ok := v.(*interp.DictVal)
	if !ok {
		return nil, namedTypeError(fn, nameQuery, v, codes.TypeDict)
	}
	out := interp.NewDictVal()
	for _, k := range d.Order {
		if _, keyed := d.KeyVals[k]; keyed {
			return nil, namedTypeError(fn, nameQuery, d.KeyValue(k), codes.TypeString)
		}
		l, ok := d.Pairs[k].(*interp.ListVal)
		if !ok {
			return nil, namedTypeError(fn, nameQuery, d.Pairs[k], codes.TypeList)
		}
		vals := make([]interp.Value, len(l.Elements))
		for i, e := range l.Elements {
			s, ok := e.(*interp.StringVal)
			if !ok {
				return nil, namedTypeError(fn, nameQuery, e, codes.TypeString)
			}
			vals[i] = &interp.StringVal{Val: s.Val}
		}
		out.Set(k, &interp.ListVal{Elements: vals})
	}
	return out, nil
}

// textArg adlı argümanın metin değeridir.
func textArg(fn any, a bind.Named, v interp.Value) (string, error) {
	s, ok := v.(*interp.StringVal)
	if !ok {
		return "", namedTypeError(fn, a, v, codes.TypeString)
	}
	return s.Val, nil
}

// namedTypeError adlı argümanın tipi yanlıştır.
func namedTypeError(fn any, a bind.Named, given interp.Value, want ...codes.Code) error {
	return messages.New(messages.H7604, fn, a, interp.TypeNames(want...), given.Type())
}

var _ interp.PropertyGetter = (*Request)(nil)
