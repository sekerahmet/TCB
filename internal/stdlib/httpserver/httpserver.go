// Package httpserver, TR programının bir modeli hizmet olarak açtığı HTTP sunucusudur (dil tanımı
// 28): TR_serve_http, istek ve cevap değerleri (TR_Request, TR_Response, TR_Headers), dosya sunmak
// (TR_file_response) ve belirteç (TR_secret_token). HTTP'yi Go'nun net/http'si ayrıştırır; geldiği
// yerin, belirtecin, yolun, eşzamanlı istek sınırının ve gövde sınırının denetimleri kilit alınmadan,
// görev açılmadan yapılır (28.4); geçen istek interp.TaskHost'tan açılan bir TR görevinde işlenir.
//
// Program bitince (hatayla ya da durdurmayla da) açık sunucular 28.8'in yoluyla kapanır
// (interp.VM.AddMainEnd); süreç hemen biterken bağlantılar kesilir (AddHardStop).
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package httpserver

import (
	"slices"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// state bir programın (VM'nin) açık sunucularıdır.
type state struct {
	mu      sync.Mutex
	servers []*HTTPServer
}

func (st *state) add(s *HTTPServer) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.servers = append(st.servers, s)
	all.mu.Lock()
	defer all.mu.Unlock()
	all.servers = append(all.servers, s)
}

func (st *state) list() []*HTTPServer {
	st.mu.Lock()
	defer st.mu.Unlock()
	return slices.Clone(st.servers)
}

// all süreçteki bütün sunuculardır: CloseAll program yolu işlemeden biterken onları keser.
var all state

// Register HTTP yerleşiklerini VM'ye kaydeder.
func Register(v *interp.VM) {
	st := &state{}
	v.RegisterBuiltinCode(codes.FnServeHttp, bind.SafeBuiltin(interp.CallPrefix(codes.FnServeHttp), st.serve))
	v.RegisterNoNamed(codes.FnSecretToken, secretToken)
	v.RegisterBuiltinCode(codes.FnFileResponse, bind.SafeBuiltin(interp.CallPrefix(codes.FnFileResponse), fileResponse))
	v.RegisterTypeCall(codes.TypeRequest, &interp.BuiltinVal{Fn: bind.SafeBuiltin(interp.CallPrefix(codes.TypeRequest), newRequest)})
	v.RegisterTypeCall(codes.TypeResponse, &interp.BuiltinVal{Fn: bind.SafeBuiltin(interp.CallPrefix(codes.TypeResponse), newResponse)})
	v.AddMainEnd(func(y *interp.VM) {
		for _, s := range st.list() {
			s.shutdown(y)
		}
	})
	v.AddHardStop(func() {
		for _, s := range st.list() {
			s.cut()
		}
	})
}

// CloseAll kapanmamış sunucuların bağlantılarını keser: program bitince çağrılır, birden çok kez
// çağrılabilir.
func CloseAll() {
	for _, s := range all.list() {
		s.cut()
	}
	all.mu.Lock()
	all.servers = nil
	all.mu.Unlock()
}
