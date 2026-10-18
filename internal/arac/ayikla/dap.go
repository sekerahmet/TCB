package ayikla

// ============================================================================
// VS CODE HATA AYIKLAMA SUNUCUSU (Debug Adapter Protocol) — tr dap
//
// VS Code, TR programlarında hata ayıklamak için `tr dap` sürecini başlatır ve
// standart giriş/çıkış üzerinden DAP iletileri gönderir. Böylece TR hata ayıklayıcısının
// yetenekleri VS Code'un kendi arayüzünde kullanılır: kesme noktaları (koşullu ve iz
// noktası), F5/F10/F11 adımlama, değişken ağacı, değer değiştirme, izleme noktası
// (değer değişince dur), satıra zıplama, çağrı yığını, Debug Console ve tablo görünümü.
//
// Eşzamanlılık: program kendi goroutine'inde çalışır. Program duraklatılmışken,
// durumu okuyan/değiştiren tüm istekler (değişkenler, değerlendirme, adımlar) VM
// goroutine'ine kanal üzerinden iletilir ve orada işlenir; böylece yorumlayıcı
// durumuna yalnızca tek bir goroutine dokunur.
// ============================================================================

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/yurut"
)

type dapMessage struct {
	Seq       int             `json:"seq"`
	Type      string          `json:"type"`
	Command   string          `json:"command,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type dapBreakpoint struct {
	Line       int
	Condition  string
	LogMessage string
}

type dapRequest struct {
	msg  *dapMessage
	done chan struct{}
}

// dapHandle, VS Code'un "variablesReference" ile açtığı bir kapsamı ya da değeri tanımlar.
type dapHandle struct {
	scope    *yurut.Environment // Yerel / Genel kapsam (değer yoksa)
	isGlobal bool
	value    yurut.Value
	expr     string // değerin TR ifadesi (evaluateName)
}

type dapServer struct {
	in    *bufio.Reader
	out   io.Writer
	outMu sync.Mutex
	seq   int

	mu sync.Mutex
	// kesmeNoktalari dosya başına (dosyaAnahtari) satır → kesme noktasıdır. VS Code
	// launch'tan önce birden çok dosya için istek gönderebilir; istekler birbirini silmez.
	// Yalnızca hata ayıklanan dosyanınkiler uygulanır.
	kesmeNoktalari map[string]map[int]dapBreakpoint
	paused         bool
	started        bool
	cmds           chan *dapRequest

	// Yalnızca VM goroutine'i duraklatılmışken kullanılır.
	curVM      *yurut.VM
	curEnv     *yurut.Environment
	curLine    int
	curReason  string
	handles    map[int]*dapHandle
	nextHandle int

	machine     *yurut.VM
	dbg         *Debugger
	prog        *agac.Program
	programPath string
	finished    chan struct{}
	exit        func(int)
}

// ServeDAP bir VS Code hata ayıklama oturumunu in/out üzerinden yürütür (tr dap).
func ServeDAP(in io.Reader, out io.Writer) error {
	return newDAPServer(in, out).serve()
}

func newDAPServer(in io.Reader, out io.Writer) *dapServer {
	return &dapServer{
		in:             bufio.NewReader(in),
		out:            out,
		kesmeNoktalari: make(map[string]map[int]dapBreakpoint),
		cmds:           make(chan *dapRequest),
		finished:       make(chan struct{}),
		exit:           os.Exit,
	}
}

// ---------------------------------------------------------------------------
// İleti çerçeveleme (Content-Length başlığı + JSON gövde)
// ---------------------------------------------------------------------------

// dapEnBuyukIleti bir DAP iletisinin gövdesinin en fazla bayt sayısıdır: bozuk ya da
// kötü niyetli bir Content-Length bellek tüketmesin.
const dapEnBuyukIleti = 64 << 20

func (s *dapServer) readMessage() (*dapMessage, error) {
	length := -1
	for {
		line, err := s.in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if length >= 0 {
				break
			}
			continue
		}
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("geçersiz Content-Length: %q", v)
			}
			if n > dapEnBuyukIleti {
				return nil, fmt.Errorf("ileti çok büyük (Content-Length %d, en fazla %d bayt)", n, dapEnBuyukIleti)
			}
			length = n
		}
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(s.in, buf); err != nil {
		return nil, err
	}
	var msg dapMessage
	if err := json.Unmarshal(buf, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *dapServer) send(m map[string]interface{}) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	s.seq++
	m["seq"] = s.seq
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n%s", len(data), data)
}

func (s *dapServer) respond(req *dapMessage, body interface{}) {
	m := map[string]interface{}{"type": "response", "request_seq": req.Seq, "success": true, "command": req.Command}
	if body != nil {
		m["body"] = body
	}
	s.send(m)
}

func (s *dapServer) respondErr(req *dapMessage, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	s.send(map[string]interface{}{
		"type": "response", "request_seq": req.Seq, "success": false, "command": req.Command,
		"message": msg,
		"body":    map[string]interface{}{"error": map[string]interface{}{"id": 1, "format": msg, "showUser": false}},
	})
}

func (s *dapServer) event(name string, body interface{}) {
	m := map[string]interface{}{"type": "event", "event": name}
	if body != nil {
		m["body"] = body
	}
	s.send(m)
}

func (s *dapServer) output(category, text string) {
	s.event("output", map[string]interface{}{"category": category, "output": text})
}

// dapOutput programın yazdır() çıktısını ve görev hatalarını VS Code'un Debug Console'una
// verilen kategoriyle ("stdout", "stderr") yönlendirir.
type dapOutput struct {
	s        *dapServer
	kategori string
}

func (o dapOutput) Write(p []byte) (int, error) {
	o.s.output(o.kategori, string(p))
	return len(p), nil
}

// dapNoStdin: hata ayıklama oturumunda standart giriş DAP iletilerine ayrılmıştır.
type dapNoStdin struct{}

func (dapNoStdin) Read([]byte) (int, error) {
	return 0, errors.New("VS Code hata ayıklama oturumunda girdi() kullanılamaz; programı terminalde çalıştırın (▶ veya Ctrl+F5)")
}

// ---------------------------------------------------------------------------
// İstek döngüsü
// ---------------------------------------------------------------------------

func (s *dapServer) serve() error {
	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if msg.Type == "request" {
			s.dispatch(msg)
		}
	}
}

func (s *dapServer) isPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

// forward isteği duraklatılmış VM goroutine'ine iletir ve işlenmesini bekler.
func (s *dapServer) forward(msg *dapMessage) {
	req := &dapRequest{msg: msg, done: make(chan struct{})}
	s.cmds <- req
	<-req.done
}

func (s *dapServer) dispatch(msg *dapMessage) {
	switch msg.Command {
	case "initialize":
		s.respond(msg, map[string]interface{}{
			"supportsConfigurationDoneRequest": true,
			"supportsConditionalBreakpoints":   true,
			"supportsLogPoints":                true,
			"supportsSetVariable":              true,
			"supportsEvaluateForHovers":        true,
			"supportsGotoTargetsRequest":       true,
			"supportsDataBreakpoints":          true,
			"supportsTerminateRequest":         true,
		})
		s.event("initialized", nil)
	case "launch":
		s.launch(msg)
	case "setBreakpoints":
		s.setBreakpoints(msg)
	case "setExceptionBreakpoints", "setFunctionBreakpoints":
		s.respond(msg, map[string]interface{}{"breakpoints": []interface{}{}})
	case "configurationDone":
		s.respond(msg, nil)
		s.start()
	case "threads":
		s.respond(msg, map[string]interface{}{"threads": []interface{}{map[string]interface{}{"id": 1, "name": "TR programı"}}})
	case "pause":
		if s.dbg != nil {
			s.dbg.mu.Lock()
			s.dbg.Mode = StepInto
			s.dbg.PauseReason = "Duraklatıldı"
			s.dbg.pauseKind = "pause"
			s.dbg.mu.Unlock()
		}
		s.respond(msg, nil)
	case "gotoTargets":
		// Hedefler duraklanan bloğa göredir: VM'in goroutine'inde yanıtlanır.
		if s.isPaused() {
			s.forward(msg)
			return
		}
		s.respond(msg, map[string]interface{}{"targets": []interface{}{}})
	case "setDataBreakpoints":
		switch {
		case s.isPaused():
			s.forward(msg)
		case !s.started:
			s.setDataBreakpoints(msg)
		default:
			s.respondErr(msg, "İzleme noktalarını değiştirmek için önce programı duraklatın")
		}
	case "disconnect", "terminate":
		s.respond(msg, nil)
		s.event("terminated", nil)
		s.exit(0)
	case "stackTrace", "scopes", "variables", "setVariable", "evaluate", "continue", "next",
		"stepIn", "stepOut", "goto", "dataBreakpointInfo", "trTablo", "trDegiskenler":
		if s.isPaused() {
			s.forward(msg)
			return
		}
		switch msg.Command {
		case "continue", "next", "stepIn", "stepOut":
			s.respond(msg, map[string]interface{}{"allThreadsContinued": true})
		case "stackTrace":
			s.respond(msg, map[string]interface{}{"stackFrames": []interface{}{}, "totalFrames": 0})
		default:
			s.respondErr(msg, "Program şu an çalışıyor; değerleri görmek için duraklatın")
		}
	default:
		s.respondErr(msg, "Desteklenmeyen istek: %s", msg.Command)
	}
}
