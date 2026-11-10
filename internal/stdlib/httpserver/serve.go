package httpserver

import (
	"errors"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Adlı argümanlar (28.1, 28.2, 28.4, 28.5).
var (
	nameHost          = bind.NamedArg(codes.ArgHost)
	namePort          = bind.NamedArg(codes.ArgPort)
	nameToken         = bind.NamedArg(codes.ArgToken)
	nameMaxConcurrent = bind.NamedArg(codes.ArgMaxConcurrent)
	nameMaxBodyBytes  = bind.NamedArg(codes.ArgMaxBodyBytes)
	namePublicPaths   = bind.NamedArg(codes.ArgPublicPaths)
)

// Süre sınırları (28.5): başlıklar 60 saniyede okunur, bağlantı bir sonraki istek için 5 saniye boşta
// bekler; cevap yazmanın ve işleyicinin süre sınırı yoktur. Testler kısaltır (t.Parallel'siz, t.Cleanup
// ile geri koyar); sunucu kurulurken okunur. Varsayılanları TestTimeoutDefaults sabitler.
var (
	headerTimeout = 60 * time.Second
	idleTimeout   = 5 * time.Second
)

const (
	defaultMaxBody = 1 << 20
	localHost      = "127.0.0.1"
	minTokenLength = 16
)

// options TR_serve_http'in denetlenmiş argümanlarıdır.
type options struct {
	host, token   string
	port          int
	maxConcurrent int64
	maxBody       int64
	public        map[string]bool // belirteç istemeyen tam yollar, yalnız GET ve HEAD (28.2)
}

// serve TR_serve_http(işleyici, ...)'dır (28.1): sunucuyu açar, beklemeden TR_HTTPServer'ı verir.
func (st *state) serve(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.FnServeHttp)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnServeHttp, 1, len(args))
	}
	switch args[0].(type) {
	case *interp.FuncVal, *interp.BuiltinVal:
	default:
		return nil, interp.ArgTypeError(codes.FnServeHttp, 1, args[0], codes.TypeFunction)
	}
	opt, err := parseOptions(fn, named)
	if err != nil {
		return nil, err
	}
	if opt.token != "" {
		m.AddSecret(opt.token) // belirtecin değeri gizlidir; yalnız sayı olsa da (19.4)
	}
	var ln net.Listener
	m.Blocking(func() { ln, err = net.Listen("tcp", net.JoinHostPort(opt.host, strconv.Itoa(opt.port))) })
	if err != nil {
		if isAddrInUse(err) {
			return nil, messages.New(messages.H7625, fn, opt.port)
		}
		return nil, messages.New(messages.H7626, fn, net.JoinHostPort(opt.host, strconv.Itoa(opt.port)))
	}
	s := &HTTPServer{
		handler: args[0], line: m.BuiltinLine(), vm: m, host: m.NewTaskHost(m.BuiltinLine()), ln: ln,
		local: opt.host == localHost, token: opt.token, port: ln.Addr().(*net.TCPAddr).Port,
		maxConcurrent: opt.maxConcurrent, maxBody: opt.maxBody, public: opt.public, done: make(chan struct{}), stop: make(chan struct{}),
	}
	s.srv = &http.Server{
		Handler: s, ReadHeaderTimeout: headerTimeout, IdleTimeout: idleTimeout,
		ErrorLog: log.New(serverLog{s}, "", 0),
	}
	st.add(s)
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// parseOptions adlı argümanları denetler; hatada sunucu açılmaz (28.1, 28.2).
func parseOptions(fn any, named map[string]interp.Value) (options, error) {
	opt := options{host: localHost, maxBody: defaultMaxBody}
	if err := bind.CheckNamed(fn, named, nameHost, namePort, nameToken, nameMaxConcurrent, nameMaxBodyBytes, namePublicPaths); err != nil {
		return opt, err
	}
	var err error
	if v, ok := bind.GetNamed(named, nameHost); ok {
		if opt.host, err = textArg(fn, nameHost, v); err != nil {
			return opt, err
		}
		if err := hostError(fn, opt.host); err != nil {
			return opt, err
		}
	}
	if v, ok := bind.GetNamed(named, namePort); ok {
		n, ok := v.(*interp.IntVal)
		if !ok {
			return opt, namedTypeError(fn, namePort, v, codes.TypeInt)
		}
		if n.Val < 0 || n.Val > 65535 {
			return opt, messages.New(messages.H7627, fn, n.Val)
		}
		opt.port = int(n.Val)
	}
	if v, ok := bind.GetNamed(named, nameToken); ok {
		if opt.token, err = textArg(fn, nameToken, v); err != nil {
			return opt, err
		}
		if !validToken(opt.token) {
			return opt, messages.New(messages.H7624, fn)
		}
	}
	if opt.host != localHost && opt.token == "" {
		return opt, messages.New(messages.H7623, fn, opt.host)
	}
	if opt.maxConcurrent, err = positiveArg(fn, named, nameMaxConcurrent, 0); err != nil {
		return opt, err
	}
	if opt.maxBody, err = positiveArg(fn, named, nameMaxBodyBytes, defaultMaxBody); err != nil {
		return opt, err
	}
	if opt.public, err = publicPathsArg(fn, named, opt.token); err != nil {
		return opt, err
	}
	return opt, nil
}

// publicPathsArg TR_public_paths:'tir (28.2): belirteçli sunucuda belirteç istemeyen tam yollar. Her öğe
// 28.3'e göre geçerli, `/` ile başlayan, sorgusuz bir yoldur (H7660); belirteçsiz sunucuda liste bir şey
// korumaz, ajan sayfayı korunuyor sanmasın diye çağrı hatasıdır (H7661). Boş liste hata değildir.
func publicPathsArg(fn any, named map[string]interp.Value, token string) (map[string]bool, error) {
	v, ok := bind.GetNamed(named, namePublicPaths)
	if !ok {
		return nil, nil
	}
	list, ok := v.(*interp.ListVal)
	if !ok {
		return nil, namedTypeError(fn, namePublicPaths, v, codes.TypeList)
	}
	paths := map[string]bool{}
	for _, e := range list.Elements {
		p, ok := e.(*interp.StringVal)
		if !ok {
			return nil, namedTypeError(fn, namePublicPaths, v, codes.TypeList)
		}
		if !strings.HasPrefix(p.Val, "/") || strings.ContainsAny(p.Val, "?#") || !validPath(p.Val) {
			return nil, messages.New(messages.H7660, fn, p.Val)
		}
		paths[p.Val] = true
	}
	if token == "" {
		return nil, messages.New(messages.H7661, fn)
	}
	return paths, nil
}

// hostError adresi ayrıştırılmış hâliyle denetler (28.2): yerel adresin tek yazımı 127.0.0.1'dir;
// localhost, IPv6 döngü adresi ve öbür döngü adresleri hatadır. Ad (IP olmayan) kabul edilmez: adın
// çözümü döngü adresine varabilir.
func hostError(fn any, host string) error {
	if host == "localhost" {
		return messages.New(messages.H7621, fn, host)
	}
	trimmed := host
	if len(trimmed) > 1 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	ip := net.ParseIP(trimmed)
	switch {
	case ip == nil:
		return messages.New(messages.H7622, fn, host)
	case ip.IsLoopback() && host != localHost:
		return messages.New(messages.H7621, fn, host)
	}
	return nil
}

// validToken belirteç en az 16 karakter ve yalnız [0-9A-Za-z_-] mi (28.2)?
func validToken(t string) bool {
	if len(t) < minTokenLength {
		return false
	}
	for i := 0; i < len(t); i++ {
		c := t[i]
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// positiveArg pozitif bir TR_Int adlı argümandır; verilmezse def.
func positiveArg(fn any, named map[string]interp.Value, a bind.Named, def int64) (int64, error) {
	v, ok := bind.GetNamed(named, a)
	if !ok {
		return def, nil
	}
	n, ok := v.(*interp.IntVal)
	if !ok || n.Val <= 0 {
		return 0, messages.New(messages.H7628, fn, a, interp.Shown(v))
	}
	return n.Val, nil
}

// HTTPServer TR_HTTPServer'dır (28.1).
type HTTPServer struct {
	handler       interp.Value
	line          int
	vm            *interp.VM // standart hataya yazmak için
	host          *interp.TaskHost
	srv           *http.Server
	ln            net.Listener // Shutdown, Serve dinleyiciyi kaydetmeden önce gelirse onu kapatmaz
	local         bool
	token         string
	port          int
	maxConcurrent int64
	maxBody       int64
	public        map[string]bool // TR_public_paths: (28.2)
	active        atomic.Int64
	closing       atomic.Bool
	stop          chan struct{} // kapanış başladı: yayın dinleyicileri cevabı bitirir (28.8)
	done          chan struct{}
}

// stderrLine standart hataya 8.4'ün kaçışı ve 19.4'ün maskesiyle bir satır yazar.
func (s *HTTPServer) stderrLine(text string) {
	s.vm.WriteOutput(s.vm.Err, turkish.Printable(text)+"\n")
}

// isAddrInUse kapı kullanılıyor mu? Windows'un kodu WSAEADDRINUSE'dur (10048).
func isAddrInUse(err error) bool {
	var e syscall.Errno
	return errors.As(err, &e) && (e == syscall.EADDRINUSE || runtime.GOOS == "windows" && e == wsaeAddrInUse)
}

const wsaeAddrInUse = 10048

// serverLog Go'nun sunucu günlüğünü kodlu iletiye çevirir (8.5, 28.9).
type serverLog struct{ s *HTTPServer }

func (l serverLog) Write(p []byte) (int, error) {
	l.s.stderrLine(messages.Text(messages.H7629, string(p)))
	return len(p), nil
}
