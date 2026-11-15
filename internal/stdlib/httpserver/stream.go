package httpserver

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
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
	beat  *sseFrame     // yazılacak nabız; her yazılışta maskelenir (28.7)
	every time.Duration
}

// sseFrame bir SSE çerçevesidir: veriden önceki satırlar (head: "id: ...\n" ve "data: ", ya da yorum
// nabzının tamamı), maskeli JSON verisi ve onu maskeleyen maskenin kimliği (interp.VM.MaskKey). Maske
// çıktının yazıldığı anda uygulanır (28.7, 19.4): kimlik değiştiyse wire veriyi yeniden maskeler.
// Bir çerçeveyi tek goroutine kullanır: halkadakileri b.mu korur, dinleyicinin kopyaları kendinindir.
type sseFrame struct {
	head []byte
	data []byte // nil: verisiz çerçeve (yorum nabzı)
	key  any
}

// dataEvent v'yi olayın verisi olarak maskeler; kimlik maskeden önce okunur: arada eklenen gizli değer
// bir sonraki yazılışta yeniden maskelenir.
func dataEvent(m *interp.VM, head []byte, v interp.Value) (*sseFrame, error) {
	key := m.MaskKey()
	data, err := eventData(m, v)
	if err != nil {
		return nil, err
	}
	return &sseFrame{head: head, data: data, key: key}, nil
}

// wire çerçevenin yazılacak baytlarıdır; maskenin kimliği değiştiyse veri önce yeniden maskelenir.
func (f *sseFrame) wire(m *interp.VM) []byte {
	if f.data == nil {
		return f.head
	}
	if key := m.MaskKey(); key != f.key {
		f.data, f.key = remask(m, f.data), key
	}
	b := make([]byte, 0, len(f.head)+len(f.data)+2)
	b = append(append(b, f.head...), f.data...)
	return append(b, "\n\n"...)
}

// remask maskeli JSON'un metin sözcüklerini (sözlük anahtarları dahil) şimdiki maskeden geçirir; öbür
// baytlar olduğu gibi kalır (DENETİM 775 Ş2: çözüp yeniden yazmak sayıların yazımını değiştirebilirdi).
// Değişen metin eventData'nın kodlamasıyla yazılır (json.Marshal). Hiçbir metin değişmezse data döner.
// data jsonwrite'ın çıktısıdır: tırnak yalnız metin sözcüğünü açar ve kapar.
func remask(m *interp.VM, data []byte) []byte {
	var out []byte
	last := 0
	for i := 0; i < len(data); i++ {
		if data[i] != '"' {
			continue
		}
		j := i + 1
		for j < len(data) && data[j] != '"' {
			if data[j] == '\\' {
				j++
			}
			j++
		}
		if j >= len(data) {
			break
		}
		var s string
		if json.Unmarshal(data[i:j+1], &s) == nil {
			if masked := m.Mask(s); masked != s {
				b, _ := json.Marshal(masked)
				out = append(append(out, data[last:i]...), b...)
				last = j + 1
			}
		}
		i = j
	}
	if out == nil {
		return data
	}
	return append(out, data[last:]...)
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
	spec := &streamSpec{block: block, beat: &sseFrame{head: commentBeat}, every: heartbeatInterval}
	if v, ok := bind.GetNamed(named, nameHeartbeat); ok {
		beat, err := dataEvent(m, dataHead, v)
		if err != nil {
			return nil, err
		}
		spec.beat = beat
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

// holdPad unutulan maskenin tutulanına en uzun gizli değerin üstüne eklenen sınırdır (28.7): adresin
// kimlik kısmındaki belirtecin ya da JWT'nin çoğu sığar (tasarım Açık-6). Yol başına tarama en çok
// en uzun gizli değer + holdPad + yaprağın boyudur, olay sayısından bağımsız.
const holdPad = 1024

// forgotten olay akışının unutulan maskesidir (28.7): son olayın her metin yaprağından 19.4a'nın akışının
// tutacağı, yaprağın JSON göstergesiyle. Sözlük anahtarları ve nabız tutulmaz. İstek akışında akış
// başına, yayında yayıncı başınadır.
type forgotten struct {
	mu     sync.Mutex
	held   map[string]string // yalnız son olayın yolları
	warned bool              // H7644 akış başına bir kez
	// Bir data çağrısının durumu (mu altında). leafFn ve iki harita akış başına bir kez kurulur: olay
	// başına ayırma yoktur, yol metni yalnız tutulan varsa ayrılır (profil 2026-11-15 15:52).
	m      *interp.VM
	next   map[string]string
	caught bool
	at     string
	leafFn func(path []byte, text string) string
}

// leaf bir metin yaprağını aynı yolun tutulanıyla maskeler (data'nın içinden, mu altında).
func (f *forgotten) leaf(path []byte, text string) string {
	out, held, c := f.m.MaskJoined(f.held[string(path)], text, holdPad)
	if held != "" {
		f.next[string(path)] = held
	}
	if c && !f.caught {
		f.caught, f.at = true, string(path)
	}
	return out
}

// data olayın verisidir (eventData gibi): her metin yaprağı aynı yolun önceki olaydan tutulanıyla
// birlikte maskelenir (interp.VM.MaskJoined). Birleşim bir şey bulursa caught true'dur ve standart hataya
// H7644 yazılır, akış başına bir kez; bulmazsa veri eventData'nınkiyle bayt bayt aynıdır. Hata verirse olay
// gitmez, tutulan değişmez.
func (f *forgotten) data(m *interp.VM, v interp.Value) (data []byte, caught bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.leafFn == nil {
		f.leafFn, f.held, f.next = f.leaf, map[string]string{}, map[string]string{}
	}
	clear(f.next)
	f.m, f.caught, f.at = m, false, ""
	data, err = jsonwrite.WriteOptions(v, jsonwrite.Options{TextFilter: m.Mask, LeafFilter: f.leafFn})
	f.m = nil
	if err != nil {
		return nil, false, err
	}
	f.held, f.next = f.next, f.held // yalnız son olayın yolları; eski harita bir sonraki çağrıda boşalır
	if f.caught && !f.warned {
		f.warned = true
		m.WriteOutput(m.Err, turkish.Printable(messages.Text(messages.H7644, f.at))+"\n")
	}
	return data, f.caught, nil
}

// dataHead yalnız data: satırı olan SSE olayının başıdır (istek akışının olayı ve nabzı).
var dataHead = []byte("data: ")

// dataFrame yalnız data: satırı olan bir SSE olayıdır.
func dataFrame(data []byte) []byte {
	f := make([]byte, 0, len(data)+8)
	f = append(f, dataHead...)
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
	held   forgotten
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
	data, _, err := e.held.data(m, args[0])
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

// beat nabzı aralıkla yazar, done kapanana ya da yazma hata verene kadar; kilit dışında. Nabız her
// yazılışta maskelenir (28.7): kurulduktan sonra eklenen gizli değer de *** olur.
func (e *EventStream) beat(m *interp.VM, f *sseFrame, every time.Duration, done <-chan struct{}) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			if !e.write(f.wire(m)) {
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
	// Blok bitince ya da kesilince (iptal, kapanış: panik) akış kapanır. Kilit süren bir nabız yazmasının
	// bitmesini bekler; ondan sonra yazılmaz: işleyici döndükten sonra cevaba yazmak Go'nun sunucusunda
	// yarıştır (TestHeartbeatIsMaskedAtEachWrite -race ile buldu; önce closed yalnız olağan dönüşte kuruluyordu).
	defer func() {
		e.mu.Lock()
		e.closed = true
		e.mu.Unlock()
		close(done)
	}()
	go func() {
		select {
		case <-r.Context().Done():
			cancel()
		case <-done:
		}
	}()
	go e.beat(task, res.stream.beat, res.stream.every, done)
	_, err = task.ApplyFunction(res.stream.block, []interp.Value{e}, nil, s.line)
	return err
}

var _ interp.PropertyGetter = (*EventStream)(nil)
