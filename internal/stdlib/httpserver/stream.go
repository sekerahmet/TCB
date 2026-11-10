package httpserver

import (
	"net/http"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// --- İSTEK AKIŞI (28.7) ---
//
// TR_stream_response(blok) gövdesi blokla yazılan bir cevaptır. İşleyici onu döndürünce blok aynı
// görevde çalışır (ServeHTTP, host.RunUntil): başlıklar gider, her .TR_send_event bir SSE olayını
// (yalnız data: satırı) tamponsuz yazar. İstemci koparsa (isteğin bağlamı) ya da bir yazma takılır ya
// da hata verirse görev iptal edilir (10.9). Blok bitince akış kapanır; bloğun hatası standart hataya
// kodlu satırdır ve bağlantı düzgün bitirilmeden kesilir. Nabız kilit dışında, aynı yazıcıyla yazılır.

var nameHeartbeat = bind.NamedArg(codes.ArgHeartbeat)

// heartbeatInterval açık akışa yazılan nabzın aralığıdır (28.7). Testler değiştirir (t.Parallel'siz,
// t.Cleanup ile geri koyar); akış kurulurken bir kez okunur.
var heartbeatInterval = 15 * time.Second

// slowWrite tek bir yazmanın en uzun süresidir: aşan dinleyicinin yalnız bağlantısı kapanır (28.7).
var slowWrite = 10 * time.Second

// commentBeat istek akışının varsayılan nabzıdır: SSE yorum satırı, veri olayı değil (28.7).
var commentBeat = []byte(":\n\n")

// streamSpec akışlı cevabın bloğu (istek akışı) ya da aboneliği (yayın akışı) ve nabzıdır.
type streamSpec struct {
	block *interp.FuncVal
	sub   *subscription // yayın akışının dinleyicisi (broadcast.go); block nil
	beat  []byte        // yazılacak nabız çerçevesi
	every time.Duration
}

// streamResponse TR_stream_response(blok, TR_heartbeat: v)'dir (28.7): blok tek parametreli bir
// fonksiyondur, akışı alır; dönüş değeri kullanılmaz.
func streamResponse(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.FnStreamResponse)
	if err := bind.CheckNamed(fn, named, nameHeartbeat); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnStreamResponse, 1, len(args))
	}
	block, ok := args[0].(*interp.FuncVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.FnStreamResponse, 1, args[0], codes.TypeFunction)
	}
	if len(block.Parameters) != 1 {
		return nil, messages.New(messages.H7635, fn, len(block.Parameters))
	}
	spec := &streamSpec{block: block, beat: commentBeat, every: heartbeatInterval}
	if v, ok := bind.GetNamed(named, nameHeartbeat); ok {
		data, err := eventData(m, v)
		if err != nil {
			return nil, err
		}
		spec.beat = dataFrame(data)
	}
	h := newHeaders()
	h.add("Content-Type", "text/event-stream; charset=utf-8")
	h.add("Cache-Control", "no-store") // 28.7: akış saklanmaz
	return &Response{status: http.StatusOK, headers: h, stream: spec}, nil
}

// eventData olayın verisidir: 13.8'in JSON'u, metinler (sözlük anahtarları dahil) 19.4'ün maskesinden
// geçerek, kaçıştan önce. Programın elindeki değer değişmez.
func eventData(m *interp.VM, v interp.Value) ([]byte, error) {
	return jsonwrite.WriteOptions(v, jsonwrite.Options{TextFilter: m.Mask})
}

// dataFrame yalnız data: satırı olan bir SSE olayıdır.
func dataFrame(data []byte) []byte {
	f := make([]byte, 0, len(data)+8)
	f = append(f, "data: "...)
	f = append(f, data...)
	return append(f, "\n\n"...)
}

// EventStream TR_EventStream'dir: bloğun aldığı akış.
type EventStream struct {
	w      http.ResponseWriter
	rc     *http.ResponseController
	fail   func() // görevi iptal eder (yazma hatası: istemci gitti)
	mu     sync.Mutex
	closed bool // blok bitti; sonra yazılmaz
	broken bool // bir yazma hata verdi
}

// EventStreamType TR_EventStream'in tip adıdır.
var EventStreamType = interp.TypeName(codes.TypeEventStream)

func (e *EventStream) Type() interp.ValueType { return EventStreamType }
func (e *EventStream) IsTruthy() bool         { return true }

// Inspect değerin metnidir (3.14): tipin adı.
func (e *EventStream) Inspect() string { return codes.ValueNames().Name(codes.TypeEventStream) }

var eventStreamMembers = []codes.Code{codes.MemSendEvent}

// GetProperty a.TR_send_event(veri)'yi verir.
func (e *EventStream) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	if kd == codes.MemSendEvent {
		return bind.CodeMethod(kd, true, e.sendEvent), nil
	}
	return nil, bind.MemberError(EventStreamType, name, eventStreamMembers)
}

// sendEvent a.TR_send_event(veri)'dir: olay kilit bırakılarak yazılır; değer döndürmez. Blok bittikten
// sonra çağrı hatasıdır (H7636). Kopan istemciye yazma hata vermez: görev iptal edilir.
func (e *EventStream) sendEvent(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.MemSendEvent, 1, len(args))
	}
	e.mu.Lock()
	closed := e.closed
	e.mu.Unlock()
	if closed {
		return nil, messages.New(messages.H7636, interp.CallPrefix(codes.MemSendEvent))
	}
	data, err := eventData(m, args[0])
	if err != nil {
		return nil, err
	}
	m.Blocking(func() { e.write(dataFrame(data)) })
	return interp.Null, nil
}

// write bir çerçeveyi süre sınırıyla yazar ve ağa iter; hata verirse akış bozulur ve görev iptal
// edilir. Kapanmış ya da bozulmuş akışa yazmaz.
func (e *EventStream) write(frame []byte) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.broken {
		return false
	}
	_ = e.rc.SetWriteDeadline(time.Now().Add(slowWrite))
	_, err := e.w.Write(frame)
	if err == nil {
		err = e.rc.Flush()
	}
	if err != nil {
		e.broken = true
		e.fail()
	}
	return err == nil
}

// beat nabzı aralıkla yazar, done kapanana ya da yazma hata verene kadar; kilit dışında.
func (e *EventStream) beat(frame []byte, every time.Duration, done <-chan struct{}) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			if !e.write(frame) {
				return
			}
		}
	}
}

// stream akışlı cevabı yazar: başlıklar gider, blok aynı görevde çalışır (HEAD'de çalışmaz). cancel
// görevi iptal eder; istemci kopunca ya da yazma hata verince çağrılır.
func (s *HTTPServer) stream(task *interp.VM, w http.ResponseWriter, r *http.Request, res *Response, cancel func()) error {
	rc := http.NewResponseController(w)
	e := &EventStream{w: w, rc: rc, fail: cancel}
	res.writeHeaders(w)
	var err error
	task.Blocking(func() {
		w.WriteHeader(http.StatusOK)
		err = rc.Flush()
	})
	if r.Method == http.MethodHead || err != nil {
		return nil
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-r.Context().Done():
			cancel()
		case <-done:
		}
	}()
	go e.beat(res.stream.beat, res.stream.every, done)
	_, err = task.ApplyFunction(res.stream.block, []interp.Value{e}, nil, s.line)
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	return err
}

var _ interp.PropertyGetter = (*EventStream)(nil)
