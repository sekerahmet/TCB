package httpserver

import (
	"context"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// HTTPServerType TR_HTTPServer'ın tip adıdır.
var HTTPServerType = interp.TypeName(codes.TypeHTTPServer)

func (s *HTTPServer) Type() interp.ValueType { return HTTPServerType }
func (s *HTTPServer) IsTruthy() bool         { return true }

// Inspect değerin metnidir (3.14): tipin adı.
func (s *HTTPServer) Inspect() string { return codes.ValueNames().Name(codes.TypeHTTPServer) }

var serverMembers = []codes.Code{codes.MemPort, codes.MemWait, codes.MemClose}

// GetProperty s.TR_port, s.TR_wait() ve s.TR_close()'u verir.
func (s *HTTPServer) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	switch kd {
	case codes.MemPort:
		return interp.NewInt(int64(s.port)), nil
	case codes.MemWait:
		return bind.CodeMethod(kd, true, s.wait), nil
	case codes.MemClose:
		return bind.CodeMethod(kd, true, func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, interp.ArgCountError(codes.MemClose, 0, len(args))
			}
			return interp.BoolToVal(s.shutdown(m)), nil
		}), nil
	}
	return nil, bind.MemberError(HTTPServerType, name, serverMembers)
}

// wait s.TR_wait()'tir: sunucu kapanana kadar kilidi bırakarak bekler (28.1). Durdurmayı üstlenen
// programda istekten sonra "durdurma istendi" hatasıyla döner, sunucu sürer (28.8, 1.6).
func (s *HTTPServer) wait(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.MemWait, 0, len(args))
	}
	if err := m.AwaitNew(interp.CallPrefix(codes.MemWait), func() { <-s.done }); err != nil {
		return nil, err
	}
	return interp.Null, nil
}

// closeWait kapanışın en uzun beklemesidir (28.8; 21.5'in süresi gibi).
const closeWait = 2 * time.Second

// shutdown sunucuyu kapatır (28.8): yeni bağlantı kabul edilmez, süren işleyici görevleri iptal edilir,
// yazılmakta olan cevap tamamlanır; açık akışlar kapanır: yayın dinleyicisinin cevabı düzgün biter, istek
// akışının bloğu iptal edilir ve akışı düzgün bitirilmeden kesilir (cevap yarımdır). En çok 2 saniye
// beklenir, sonra bağlantılar kesilir. y kilidi tutar. Bu çağrı kapattıysa true verir; zaten kapalıysa
// false.
func (s *HTTPServer) shutdown(y *interp.VM) bool {
	if !s.closing.CompareAndSwap(false, true) {
		return false
	}
	close(s.stop) // açık yayın dinleyicileri cevabı bitirir; istek akışlarının blokları iptal edilir (host.Close)
	s.host.Close(y)
	y.Blocking(func() {
		ctx, cancel := context.WithTimeout(context.Background(), closeWait)
		defer cancel()
		if s.srv.Shutdown(ctx) != nil {
			_ = s.srv.Close()
		}
		_ = s.ln.Close() // Serve henüz başlamadıysa dinleyici açık kalırdı: kapanıştan sonra bağlantı kabul edilmez
	})
	close(s.done)
	return true
}

// cut süreç hemen biterken (1.7) ya da program kapanışı yolsuz biterken bağlantıları keser.
func (s *HTTPServer) cut() {
	if s.closing.CompareAndSwap(false, true) {
		close(s.stop)
		_ = s.srv.Close()
		_ = s.ln.Close()
		close(s.done)
	}
}

var _ interp.PropertyGetter = (*HTTPServer)(nil)
