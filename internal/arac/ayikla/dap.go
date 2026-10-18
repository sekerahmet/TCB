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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/denetci"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/turkce"
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

// ---------------------------------------------------------------------------
// Başlatma
// ---------------------------------------------------------------------------

func (s *dapServer) launch(msg *dapMessage) {
	var a struct {
		Program     string `json:"program"`
		StopOnEntry bool   `json:"stopOnEntry"`
		NoDebug     bool   `json:"noDebug"`
		Cwd         string `json:"cwd"`
	}
	if err := json.Unmarshal(msg.Arguments, &a); err != nil || a.Program == "" {
		s.respondErr(msg, "Hata ayıklanacak .tr dosyası belirtilmedi ('program')")
		return
	}
	path, _ := filepath.Abs(a.Program)
	data, err := os.ReadFile(path)
	if err != nil {
		s.respondErr(msg, "Hata ayıklanacak program okunamadı (%s): %s", path, turkce.DosyaHatasiNedeni(path, err))
		return
	}
	code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")

	p := ayristir.New(sozcuk.New(code))
	prog := p.ParseProgram()
	errs := p.Errors()
	if len(errs) == 0 {
		chk := denetci.New()
		errs = chk.CheckProgram(prog)
		// Uyarılar (ör. yerleşik ad gölgeleme) programı durdurmaz; Debug Console'a yazılır.
		if u := chk.Warnings(); len(u) > 0 {
			s.output("stderr", "⚠️ Uyarılar:\n")
			for _, e := range u {
				s.output("stderr", "  • "+e+"\n"+indentLines(turkce.KodCercevesi(code, e), "    "))
			}
		}
	}
	if len(errs) > 0 {
		s.output("stderr", "❌ Program başlatılamadı:\n")
		for _, e := range errs {
			s.output("stderr", "  • "+e+"\n"+indentLines(turkce.KodCercevesi(code, e), "    "))
		}
		s.respondErr(msg, "%s içinde hata var; ayrıntılar Debug Console'da", filepath.Base(path))
		s.event("terminated", nil)
		return
	}

	if a.Cwd != "" {
		_ = os.Chdir(a.Cwd)
	}
	machine := yurut.New()
	machine.SourceFile = path
	machine.SourceCode = code
	machine.Out = dapOutput{s, "stdout"}
	machine.Err = dapOutput{s, "stderr"} // eşzamanlı görev hataları da Debug Console'a
	machine.Stdin = dapNoStdin{}

	if !a.NoDebug {
		dbg := NewDebugger(path, code, a.StopOnEntry)
		dbg.Reader = nil
		dbg.dap = s
		dbg.PauseReason = ""
		if a.StopOnEntry {
			dbg.PauseReason = "Programın başında durdu"
			dbg.pauseKind = "entry"
		}
		machine.Debugger = dbg
		s.dbg = dbg
	}
	s.machine = machine
	s.prog = prog
	s.programPath = path
	s.respond(msg, nil)
}

func (s *dapServer) start() {
	s.mu.Lock()
	if s.started || s.machine == nil {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go func() {
		defer close(s.finished)
		_, err := s.machine.Run(s.prog)
		s.machine.WaitAsync()
		exitCode := 0
		if err != nil {
			exitCode = 1
			metin := "❌ Çalışma Zamanı Hatası:\n"
			for _, m := range yurut.HataMaddeleri(err, s.machine.SourceCode) {
				metin += "  • " + m.Mesaj + "\n" + indentLines(m.Cerceve, "    ")
			}
			s.output("stderr", metin)
		}
		s.event("exited", map[string]interface{}{"exitCode": exitCode})
		s.event("terminated", nil)
	}()
}

func indentLines(text, prefix string) string {
	if text == "" {
		return ""
	}
	var sb strings.Builder
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		sb.WriteString(prefix + l + "\n")
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Kesme noktaları
// ---------------------------------------------------------------------------

func (s *dapServer) setBreakpoints(msg *dapMessage) {
	var a struct {
		Source struct {
			Path string `json:"path"`
		} `json:"source"`
		Breakpoints []struct {
			Line         int    `json:"line"`
			Condition    string `json:"condition"`
			LogMessage   string `json:"logMessage"`
			HitCondition string `json:"hitCondition"`
		} `json:"breakpoints"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)

	// Kesme noktaları dosya başına tutulur: her istek yalnızca kendi dosyasınınkileri
	// değiştirir. Şimdilik yalnızca hata ayıklanan dosyanınkiler uygulanır ('aktar' ile
	// yüklenen modüllerin satırları ayrı dosyalardadır); launch'tan önce dosya bilinmediği
	// için her istek doğrulanmış sayılır.
	result := []interface{}{}
	noktalar := make(map[int]dapBreakpoint, len(a.Breakpoints))
	s.mu.Lock()
	s.kesmeNoktalari[dosyaAnahtari(a.Source.Path)] = noktalar
	sameFile := s.programPath == "" || samePath(a.Source.Path, s.programPath)
	for _, bp := range a.Breakpoints {
		noktalar[bp.Line] = dapBreakpoint{Line: bp.Line, Condition: bp.Condition, LogMessage: bp.LogMessage}
		entry := map[string]interface{}{"line": bp.Line, "verified": sameFile}
		if !sameFile {
			entry["message"] = "Yalnızca hata ayıklanan dosyada kesme noktası konabilir"
		}
		result = append(result, entry)
	}
	s.mu.Unlock()
	s.respond(msg, map[string]interface{}{"breakpoints": result})
}

// dosyaAnahtari bir yolu kesme noktası haritasının anahtarına çevirir (mutlak, temiz,
// büyük/küçük harfe duyarsız: samePath ile aynı eşitlik).
func dosyaAnahtari(yol string) string {
	if yol == "" {
		return ""
	}
	m, err := filepath.Abs(yol)
	if err != nil {
		m = yol
	}
	return strings.ToLower(filepath.Clean(m))
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	pa, _ := filepath.Abs(a)
	pb, _ := filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(pa), filepath.Clean(pb))
}

// breakpointHit satırda durulup durulmayacağına karar verir: koşul sağlanmazsa
// durmaz; iz noktası (logMessage) ise mesajı yazar ve durmaz.
func (s *dapServer) breakpointHit(v *yurut.VM, line int, env *yurut.Environment) bool {
	s.mu.Lock()
	bp, ok := s.kesmeNoktalari[dosyaAnahtari(s.programPath)][line]
	s.mu.Unlock()
	if !ok {
		return false
	}
	if bp.Condition != "" {
		val, err := s.evalQuiet(v, env, bp.Condition)
		if err != nil {
			s.output("console", fmt.Sprintf("Kesme noktası koşulu hatalı (satır %d): %v\n", line, err))
			return true
		}
		if !val.IsTruthy() {
			return false
		}
	}
	if bp.LogMessage != "" {
		s.output("console", s.interpolate(v, env, bp.LogMessage)+"\n")
		return false
	}
	return true
}

// interpolate iz noktası mesajındaki {ifade} parçalarını değerleriyle doldurur.
func (s *dapServer) interpolate(v *yurut.VM, env *yurut.Environment, text string) string {
	var sb strings.Builder
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '{' {
			sb.WriteRune(runes[i])
			continue
		}
		j := i + 1
		for j < len(runes) && runes[j] != '}' {
			j++
		}
		if j >= len(runes) {
			sb.WriteString(string(runes[i:]))
			break
		}
		expr := string(runes[i+1 : j])
		if val, err := s.evalQuiet(v, env, expr); err == nil {
			sb.WriteString(plainText(val))
		} else {
			sb.WriteString("<" + err.Error() + ">")
		}
		i = j
	}
	return sb.String()
}

// evalQuiet bir ifadeyi debugger'ı tetiklemeden hesaplar.
func (s *dapServer) evalQuiet(v *yurut.VM, env *yurut.Environment, expr string) (yurut.Value, error) {
	p := ayristir.New(sozcuk.New(expr))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, errors.New(p.Errors()[0])
	}
	if len(prog.Statements) != 1 {
		return nil, fmt.Errorf("tek bir ifade bekleniyordu: %s", expr)
	}
	es, ok := prog.Statements[0].(*agac.ExpressionStatement)
	if !ok {
		return nil, fmt.Errorf("tek bir ifade bekleniyordu: %s", expr)
	}
	return s.quietly(func() (yurut.Value, error) { return v.EvalExpression(es.Expression, env) })
}

func (s *dapServer) quietly(fn func() (yurut.Value, error)) (yurut.Value, error) {
	if s.dbg == nil {
		return fn()
	}
	was := s.dbg.Active
	s.dbg.Active = false
	defer func() { s.dbg.Active = was }()
	return fn()
}

// ---------------------------------------------------------------------------
// Duraklama: VM goroutine'i burada bekler ve gelen istekleri işler
// ---------------------------------------------------------------------------

func (s *dapServer) waitForCommand(v *yurut.VM, line int, env *yurut.Environment) int {
	d := s.dbg
	d.mu.Lock()
	reason, kind := d.PauseReason, d.pauseKind
	d.mu.Unlock()
	if kind == "" {
		kind = "step"
	}
	// Durum metinleri terminal ve VS Code'da aynıdır (tuşlar da aynı anlamdadır).
	if reason == "" && kind == "step" {
		reason = durumAdim
	}

	s.mu.Lock()
	s.paused = true
	s.curVM, s.curEnv, s.curLine, s.curReason = v, env, line, reason
	s.handles = make(map[int]*dapHandle)
	s.nextHandle = 0
	s.mu.Unlock()

	s.event("stopped", map[string]interface{}{
		"reason": kind, "description": reason, "text": reason, "threadId": 1, "allThreadsStopped": true,
	})

	for req := range s.cmds {
		jump, resume := s.handlePaused(req.msg)
		if resume {
			s.mu.Lock()
			s.paused = false
			s.mu.Unlock()
			close(req.done)
			return jump
		}
		close(req.done)
	}
	return 0
}

// resume adım komutunu uygular; duraklama nedeni bir sonraki duruş için temizlenir.
func (s *dapServer) resume(mode StepMode, target int, reason, kind string) {
	d := s.dbg
	d.mu.Lock()
	d.Mode = mode
	d.TargetDepth = target
	d.PauseReason = reason
	d.pauseKind = kind
	d.mu.Unlock()
}

func (s *dapServer) handlePaused(msg *dapMessage) (jump int, resume bool) {
	switch msg.Command {
	case "continue":
		s.resume(StepContinue, 0, "", "")
		s.respond(msg, map[string]interface{}{"allThreadsContinued": true})
		return 0, true
	case "next":
		s.resume(StepOver, s.curVM.StackDepth(), "", "")
		s.respond(msg, nil)
		return 0, true
	case "stepIn":
		s.resume(StepInto, 0, "", "")
		s.respond(msg, nil)
		return 0, true
	case "stepOut":
		s.resume(StepOut, s.curVM.StackDepth(), "", "")
		s.respond(msg, nil)
		return 0, true
	case "goto":
		var a struct {
			TargetID int `json:"targetId"`
		}
		_ = json.Unmarshal(msg.Arguments, &a)
		if a.TargetID < 1 {
			s.respondErr(msg, "Geçersiz hedef satır")
			return 0, false
		}
		hedef, ok := s.curVM.ZiplamaHedefi(a.TargetID)
		if !ok {
			s.respondErr(msg, "Satır %d şu an çalışan bloğun dışında; yalnızca bulunulan bloğun (fonksiyon, döngü ya da eğer gövdesinin) satırlarına zıplanabilir", a.TargetID)
			return 0, false
		}
		s.resume(StepInto, 0, "Satır "+turkce.SayiEk(int64(hedef), turkce.Yonelme)+" zıplandı", "goto")
		s.respond(msg, nil)
		return hedef, true
	case "gotoTargets":
		var a struct {
			Line int `json:"line"`
		}
		_ = json.Unmarshal(msg.Arguments, &a)
		hedefler := []interface{}{}
		if hedef, ok := s.curVM.ZiplamaHedefi(a.Line); ok {
			hedefler = append(hedefler, map[string]interface{}{"id": hedef, "label": "Satır " + strconv.Itoa(hedef), "line": hedef})
		}
		s.respond(msg, map[string]interface{}{"targets": hedefler})
	case "stackTrace":
		s.stackTrace(msg)
	case "scopes":
		s.scopes(msg)
	case "variables":
		s.variables(msg)
	case "setVariable":
		s.setVariable(msg)
	case "evaluate":
		s.evaluate(msg)
	case "dataBreakpointInfo":
		s.dataBreakpointInfo(msg)
	case "setDataBreakpoints":
		s.setDataBreakpoints(msg)
	case "trTablo":
		s.table(msg)
	case "trDegiskenler":
		s.variablesTable(msg)
	default:
		s.respondErr(msg, "Desteklenmeyen istek: %s", msg.Command)
	}
	return 0, false
}

func (s *dapServer) source() map[string]interface{} {
	return map[string]interface{}{"name": filepath.Base(s.programPath), "path": s.programPath}
}

func (s *dapServer) stackTrace(msg *dapMessage) {
	stack := s.curVM.StackSnapshot()
	frameName := func(i int) string {
		if i < 0 {
			return "ana program"
		}
		return stack[i].FuncName + "()"
	}
	frames := []interface{}{map[string]interface{}{
		"id": 1, "name": frameName(len(stack) - 1), "line": s.curLine, "column": 1, "source": s.source(),
	}}
	for i := len(stack) - 1; i >= 0; i-- {
		line := stack[i].Line
		if line < 1 {
			line = 1
		}
		frames = append(frames, map[string]interface{}{
			"id": len(stack) - i + 1, "name": frameName(i - 1), "line": line, "column": 1, "source": s.source(),
		})
	}
	s.respond(msg, map[string]interface{}{"stackFrames": frames, "totalFrames": len(frames)})
}

func (s *dapServer) newHandle(h *dapHandle) int {
	s.nextHandle++
	s.handles[s.nextHandle] = h
	return s.nextHandle
}

func (s *dapServer) scopes(msg *dapMessage) {
	var a struct {
		FrameID int `json:"frameId"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	var scopes []interface{}
	if a.FrameID <= 1 && s.curEnv != nil && s.curEnv != s.curVM.GlobalEnv {
		scopes = append(scopes, map[string]interface{}{
			"name": "Yerel", "presentationHint": "locals", "expensive": false,
			"variablesReference": s.newHandle(&dapHandle{scope: s.curEnv}),
		})
	}
	scopes = append(scopes, map[string]interface{}{
		"name": "Genel", "expensive": false,
		"variablesReference": s.newHandle(&dapHandle{scope: s.curVM.GlobalEnv, isGlobal: true}),
	})
	s.respond(msg, map[string]interface{}{"scopes": scopes})
}

// scopeVariables kapsamdaki değişkenleri (yerleşik fonksiyon ve tanımlar hariç) sıralı döndürür.
func (s *dapServer) scopeVariables(h *dapHandle) []varEntry {
	global := s.curVM.GlobalEnv
	seen := map[string]bool{}
	var list []varEntry
	for cur := h.scope; cur != nil; cur = cur.Outer() {
		if !h.isGlobal && cur == global {
			break
		}
		cur.EachLocal(func(k string, v yurut.Value) {
			if seen[k] {
				return
			}
			seen[k] = true
			if v.Type() == yurut.VAL_BUILTIN || v.Type() == yurut.VAL_STRUCT || v.Type() == yurut.VAL_FUNC {
				return
			}
			list = append(list, varEntry{Name: k, Val: v})
		})
		if h.isGlobal {
			break
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

// dapChild açılmış bir değerin alt öğesidir. val nil ise öğe yalnızca bilgi satırıdır
// (ör. kısaltılmış tensörün sonundaki "…"); değeri not'tur ve açılamaz.
type dapChild struct {
	name, expr string
	val        yurut.Value
	not        string
}

// yinelemeSiniri, gezilebilir dış değerlerde (ör. tensör) panelde gösterilen en fazla öğe
// sayısıdır; fazlası tek bir "…" satırıyla özetlenir.
const yinelemeSiniri = 1000

// children açılabilir bir değerin alt elemanlarını döndürür.
func children(val yurut.Value, expr string) []dapChild {
	const limit = 5000
	var out []dapChild
	switch v := val.(type) {
	case yurut.Yinelenebilir:
		// Dış değerler (ör. tensör): ilk eksen boyunca öğeler (tensörde 1 boyutluda sayılar,
		// N boyutluda (N-1) boyutlu görünümler); çok büyük değerlerde ilk öğeler ve "…".
		n, err := v.YinelemeUzunlugu()
		if err != nil {
			break
		}
		for i := 0; i < n && i < yinelemeSiniri; i++ {
			el, err := v.YinelemeOgesi(i)
			if err != nil {
				break
			}
			out = append(out, dapChild{name: fmt.Sprintf("[%d]", i), expr: fmt.Sprintf("%s[%d]", expr, i), val: el})
		}
		if n > yinelemeSiniri {
			out = append(out, dapChild{name: "…", not: fmt.Sprintf("ilk %d öğe gösteriliyor (toplam %d)", yinelemeSiniri, n)})
		}
	case *yurut.ListVal:
		for i, el := range v.Elements {
			if i >= limit {
				break
			}
			out = append(out, dapChild{name: fmt.Sprintf("[%d]", i), expr: fmt.Sprintf("%s[%d]", expr, i), val: el})
		}
	case *yurut.DictVal:
		for _, k := range v.Order {
			el, ok := v.Pairs[k]
			if !ok {
				continue
			}
			key := v.KeyValue(k)
			keyExpr := key.Inspect()
			if sv, isStr := key.(*yurut.StringVal); isStr {
				keyExpr = trStringLiteral(sv.Val)
			}
			out = append(out, dapChild{name: v.DisplayKey(k), expr: expr + "[" + keyExpr + "]", val: el})
			if len(out) >= limit {
				break
			}
		}
	case *yurut.StructInstanceVal:
		seen := map[string]bool{}
		for _, f := range v.Def.Fields {
			if fv, ok := v.Fields[f.Name]; ok {
				seen[f.Name] = true
				out = append(out, dapChild{name: f.Name, expr: expr + "." + f.Name, val: fv})
			}
		}
		var rest []string
		for k := range v.Fields {
			if !seen[k] {
				rest = append(rest, k)
			}
		}
		sort.Strings(rest)
		for _, k := range rest {
			out = append(out, dapChild{name: k, expr: expr + "." + k, val: v.Fields[k]})
		}
	}
	return out
}

func expandable(val yurut.Value) bool {
	switch v := val.(type) {
	case *yurut.ListVal:
		return len(v.Elements) > 0
	case *yurut.DictVal:
		return len(v.Pairs) > 0
	case *yurut.StructInstanceVal:
		return len(v.Fields) > 0
	case yurut.Yinelenebilir:
		n, err := v.YinelemeUzunlugu()
		return err == nil && n > 0
	}
	return false
}

func (s *dapServer) refFor(val yurut.Value, expr string) int {
	if val == nil || !expandable(val) {
		return 0
	}
	return s.newHandle(&dapHandle{value: val, expr: expr})
}

// display değişkenin yanında gösterilecek kısa metindir.
func display(val yurut.Value) string {
	if val == nil {
		return "yok"
	}
	if iz, ok := val.(IzlemeDegeri); ok {
		// Çok satırlı yazdırma biçimi yerine kısa özet gösterilir.
		return iz.IzlemeEtiketi()
	}
	var prefix string
	switch v := val.(type) {
	case *yurut.ListVal:
		prefix = fmt.Sprintf("(%d) ", len(v.Elements))
	case *yurut.DictVal:
		prefix = fmt.Sprintf("{%d} ", len(v.Pairs))
	}
	text := prefix + val.Inspect()
	if r := []rune(text); len(r) > 200 {
		text = string(r[:200]) + "…"
	}
	return text
}

func (s *dapServer) variableEntry(name, expr string, val yurut.Value) map[string]interface{} {
	return map[string]interface{}{
		"name": name, "value": display(val), "type": string(val.Type()),
		"evaluateName": expr, "variablesReference": s.refFor(val, expr),
	}
}

func (s *dapServer) variables(msg *dapMessage) {
	var a struct {
		VariablesReference int `json:"variablesReference"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	h := s.handles[a.VariablesReference]
	vars := []interface{}{}
	if h != nil {
		if h.value == nil {
			for _, e := range s.scopeVariables(h) {
				vars = append(vars, s.variableEntry(e.Name, e.Name, e.Val))
			}
		} else {
			for _, c := range children(h.value, h.expr) {
				if c.val == nil {
					vars = append(vars, map[string]interface{}{"name": c.name, "value": c.not, "variablesReference": 0})
					continue
				}
				vars = append(vars, s.variableEntry(c.name, c.expr, c.val))
			}
		}
	}
	s.respond(msg, map[string]interface{}{"variables": vars})
}

// setVariable ✏️ "değeri değiştir": yeni değer bir TR ifadesi olarak hesaplanır.
func (s *dapServer) setVariable(msg *dapMessage) {
	var a struct {
		VariablesReference int    `json:"variablesReference"`
		Name               string `json:"name"`
		Value              string `json:"value"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	h := s.handles[a.VariablesReference]
	if h == nil {
		s.respondErr(msg, "Değişken bulunamadı")
		return
	}
	val, err := s.evalQuiet(s.curVM, s.curEnv, a.Value)
	if err != nil {
		s.respondErr(msg, "Değer hesaplanamadı: %v", err)
		return
	}
	expr := a.Name
	switch {
	case h.value == nil:
		if err := h.scope.Assign(a.Name, val); err != nil {
			s.respondErr(msg, "%v", err)
			return
		}
	default:
		found := false
		for _, c := range children(h.value, h.expr) {
			if c.name != a.Name || c.val == nil {
				continue
			}
			found, expr = true, c.expr
			switch container := h.value.(type) {
			case *yurut.ListVal:
				idx, _ := strconv.Atoi(strings.Trim(a.Name, "[]"))
				container.Elements[idx] = val
			case *yurut.DictVal:
				for _, k := range container.Order {
					if container.DisplayKey(k) == a.Name {
						container.Pairs[k] = val
					}
				}
			case *yurut.StructInstanceVal:
				container.Fields[a.Name] = val
			case yurut.IndeksAtanabilir:
				// Dış değerler (ör. tensör satırı ya da elemanı): değer[i] = yeni değer
				idx, _ := strconv.Atoi(strings.Trim(a.Name, "[]"))
				if err := container.IndekseAta(yurut.NewInt(int64(idx)), val); err != nil {
					s.respondErr(msg, "%v", err)
					return
				}
			default:
				s.respondErr(msg, "Bu değerin elemanları değiştirilemez")
				return
			}
		}
		if !found {
			s.respondErr(msg, "Eleman bulunamadı: %s", a.Name)
			return
		}
	}
	s.respond(msg, map[string]interface{}{
		"value": display(val), "type": string(val.Type()), "variablesReference": s.refFor(val, expr),
	})
}

// evaluate: izleme (watch), üzerine gelme (hover) ve Debug Console. Konsolda deyimler
// de çalıştırılabilir (ör. 'bakiye = 0' ya da 'yazdır(liste)').
func (s *dapServer) evaluate(msg *dapMessage) {
	var a struct {
		Expression string `json:"expression"`
		Context    string `json:"context"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	p := ayristir.New(sozcuk.New(a.Expression))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		s.respondErr(msg, "%s", p.Errors()[0])
		return
	}
	var val yurut.Value
	var err error
	if es, ok := singleExpression(prog); ok {
		val, err = s.quietly(func() (yurut.Value, error) { return s.curVM.EvalExpression(es.Expression, s.curEnv) })
	} else if a.Context == "repl" {
		val, err = s.quietly(func() (yurut.Value, error) {
			return s.curVM.ExecStatements(prog.Statements, s.curEnv)
		})
	} else {
		err = fmt.Errorf("yalnızca ifade değerlendirilebilir")
	}
	if err != nil {
		s.respondErr(msg, "%v", err)
		return
	}
	if val == nil {
		val = yurut.NULL
	}
	s.respond(msg, map[string]interface{}{
		"result": display(val), "type": string(val.Type()), "variablesReference": s.refFor(val, a.Expression),
	})
}

func singleExpression(prog *agac.Program) (*agac.ExpressionStatement, bool) {
	if len(prog.Statements) != 1 {
		return nil, false
	}
	es, ok := prog.Statements[0].(*agac.ExpressionStatement)
	return es, ok
}

// dataBreakpointInfo: Variables panelinde "Break on Value Change" (izleme noktası).
func (s *dapServer) dataBreakpointInfo(msg *dapMessage) {
	var a struct {
		VariablesReference int    `json:"variablesReference"`
		Name               string `json:"name"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	h := s.handles[a.VariablesReference]
	if h == nil || h.value != nil {
		s.respond(msg, map[string]interface{}{"dataId": nil, "description": "İzleme noktası yalnızca değişkenlere konabilir"})
		return
	}
	s.respond(msg, map[string]interface{}{
		"dataId": a.Name, "description": "'" + a.Name + "' değiştiğinde dur",
		"accessTypes": []string{"write"}, "canPersist": true,
	})
}

func (s *dapServer) setDataBreakpoints(msg *dapMessage) {
	var a struct {
		Breakpoints []struct {
			DataID string `json:"dataId"`
		} `json:"breakpoints"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	if s.dbg == nil {
		s.respond(msg, map[string]interface{}{"breakpoints": []interface{}{}})
		return
	}
	var wps []*Watchpoint
	result := []interface{}{}
	for i, bp := range a.Breakpoints {
		wp := &Watchpoint{ID: i + 1, Expression: bp.DataID}
		if s.curEnv != nil {
			if v, ok := s.curEnv.Get(bp.DataID); ok {
				wp.LastVal = v.Inspect()
			}
		}
		wps = append(wps, wp)
		result = append(result, map[string]interface{}{"verified": true})
	}
	s.dbg.Watchpoints = wps
	s.respond(msg, map[string]interface{}{"breakpoints": result})
}

// ---------------------------------------------------------------------------
// İzleme Masası (özel istek: trDegiskenler)
// ---------------------------------------------------------------------------

// variablesTable tüm değişkenleri tek bir tabloda
// (Kapsam | Değişken | Tip | Değer) döndürür; tablo olarak açılabilen değerler işaretlenir.
func (s *dapServer) variablesTable(msg *dapMessage) {
	list := []interface{}{}
	add := func(kapsam string, h *dapHandle) {
		for _, e := range s.scopeVariables(h) {
			_, _, tblErr := TableData(e.Val)
			list = append(list, map[string]interface{}{
				"kapsam": kapsam, "ad": e.Name, "tip": typeLabel(e.Val), "tur": valueKind(e.Val),
				"deger": display(e.Val), "tablo": tblErr == nil,
			})
		}
	}
	if s.curEnv != nil && s.curEnv != s.curVM.GlobalEnv {
		add("Yerel", &dapHandle{scope: s.curEnv})
	}
	add("Genel", &dapHandle{scope: s.curVM.GlobalEnv, isGlobal: true})

	frame := "ana program"
	if st := s.curVM.StackSnapshot(); len(st) > 0 {
		frame = st[len(st)-1].FuncName + "()"
	}
	s.respond(msg, map[string]interface{}{
		"degiskenler": list, "satir": s.curLine, "neden": s.curReason,
		"cerceve": frame, "dosya": filepath.Base(s.programPath),
	})
}

// IzlemeDegeri, motor paketlerinin kendi değer türlerini (ör. tensör) İzleme Masası'nda
// göstermek için uyguladığı arayüzdür. ayikla bu paketleri içe aktarmaz; değer
// arayüzü uyguluyorsa tür, tip etiketi ve tablo verisi ondan alınır.
type IzlemeDegeri interface {
	// IzlemeTuru ikon seçimi için türdür (ör. "tensor").
	IzlemeTuru() string
	// IzlemeEtiketi tip sütununda gösterilir (ör. "Tensör [2×3]").
	IzlemeEtiketi() string
	// TabloVerisi değeri tabloya çevirir; tablo olarak açılamıyorsa hata döner.
	TabloVerisi() ([]string, [][]string, error)
}

// valueKind panelde ikon seçmek için değerin türünü verir.
func valueKind(val yurut.Value) string {
	if iz, ok := val.(IzlemeDegeri); ok {
		return iz.IzlemeTuru()
	}
	switch v := val.(type) {
	case *yurut.ListVal:
		if cols, _, err := TableData(v); err == nil && !(len(cols) == 1 && cols[0] == "Değer") {
			return "tablo"
		}
		return "liste"
	case *yurut.DictVal:
		return "sozluk"
	case *yurut.StructInstanceVal:
		return "yapi"
	case *yurut.IntVal, *yurut.FloatVal:
		return "sayi"
	case *yurut.StringVal:
		return "metin"
	case *yurut.BoolVal:
		return "mantik"
	case *yurut.NullVal:
		return "yok"
	}
	return "diger"
}

// typeLabel tip sütununda değerin yapısını özetler (ör. Tablo [3×3]).
func typeLabel(val yurut.Value) string {
	if iz, ok := val.(IzlemeDegeri); ok {
		return iz.IzlemeEtiketi()
	}
	switch v := val.(type) {
	case *yurut.ListVal:
		if cols, rows, err := TableData(v); err == nil && !(len(cols) == 1 && cols[0] == "Değer") {
			return fmt.Sprintf("Tablo [%d×%d]", len(rows), len(cols))
		}
		return fmt.Sprintf("Liste [%d]", len(v.Elements))
	case *yurut.DictVal:
		return fmt.Sprintf("Sözlük [%d]", len(v.Pairs))
	case *yurut.StructInstanceVal:
		return "Yapı " + v.Def.Name
	}
	return string(val.Type())
}

// ---------------------------------------------------------------------------
// Tablo görünümü (özel istek: trTablo)
// ---------------------------------------------------------------------------

func (s *dapServer) table(msg *dapMessage) {
	var a struct {
		Ifade string `json:"ifade"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	val, err := s.evalQuiet(s.curVM, s.curEnv, a.Ifade)
	if err != nil {
		s.respondErr(msg, "%v", err)
		return
	}
	cols, rows, err := TableData(val)
	if err != nil {
		s.respondErr(msg, "%v", err)
		return
	}
	s.respond(msg, map[string]interface{}{"baslik": a.Ifade, "sutunlar": cols, "satirlar": rows, "tip": string(val.Type())})
}

// TableData bir değeri tabloya (sütunlar + satırlar) dönüştürür:
// sözlük/yapı listeleri, düz listeler, sözlükler, yapılar ve (IzlemeDegeri ile) tensörler
// desteklenir.
func TableData(val yurut.Value) ([]string, [][]string, error) {
	const limit = 10000
	if iz, ok := val.(IzlemeDegeri); ok {
		return iz.TabloVerisi()
	}
	switch v := val.(type) {
	case *yurut.ListVal:
		records := true
		for _, el := range v.Elements {
			switch el.(type) {
			case *yurut.DictVal, *yurut.StructInstanceVal:
			default:
				records = false
			}
		}
		if records && len(v.Elements) > 0 {
			var cols []string
			seen := map[string]bool{}
			for _, el := range v.Elements {
				for _, c := range children(el, "") {
					if !seen[c.name] {
						seen[c.name] = true
						cols = append(cols, c.name)
					}
				}
			}
			var rows [][]string
			for i, el := range v.Elements {
				if i >= limit {
					break
				}
				cells := map[string]string{}
				for _, c := range children(el, "") {
					cells[c.name] = plainText(c.val)
				}
				row := make([]string, len(cols))
				for j, c := range cols {
					row[j] = cells[c]
				}
				rows = append(rows, row)
			}
			return cols, rows, nil
		}
		var rows [][]string
		for i, el := range v.Elements {
			if i >= limit {
				break
			}
			rows = append(rows, []string{plainText(el)})
		}
		return []string{"Değer"}, rows, nil
	case *yurut.DictVal:
		var rows [][]string
		for _, k := range v.Order {
			rows = append(rows, []string{v.DisplayKey(k), plainText(v.Pairs[k])})
		}
		return []string{"Anahtar", "Değer"}, rows, nil
	case *yurut.StructInstanceVal:
		var rows [][]string
		for _, c := range children(v, "") {
			rows = append(rows, []string{c.name, plainText(c.val)})
		}
		return []string{"Alan", "Değer"}, rows, nil
	}
	return nil, nil, fmt.Errorf("tablo görünümü yalnızca liste, sözlük, yapı ve tensörler için kullanılabilir (bu değer: %s)", val.Type())
}

// plainText hücrede gösterilecek metindir: metinler tırnaksız yazılır.
func plainText(val yurut.Value) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(*yurut.StringVal); ok {
		return s.Val
	}
	return val.Inspect()
}

// trStringLiteral bir metni TR kaynak kodunda güvenle kullanılabilecek değişmeze çevirir.
func trStringLiteral(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "{", `\{`, "}", `\}`, "\n", `\n`, "\t", `\t`)
	return `"` + r.Replace(s) + `"`
}
