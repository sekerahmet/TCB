package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// --- YAYIN AKIŞI (28.7) ---
//
// TR_EventBroadcast(TR_capacity: n, TR_snapshot: g, TR_reduce: f) aynı olayları birden çok dinleyiciye
// gönderir. .TR_send_event(veri) yayıncının dışlamasında (sem) maskeli JSON'u kurar, olayı görüntüye
// katlar (f), kimliği verir ve olayı halkaya koyar; dinleyicilere yalnız haber verir, ağa yazmaz.
// .TR_subscribe(istek) aynı dışlamada görüntüyü ya da geri oynatılacak olayları ve dinleyicinin imlecini
// alır: abonelik gönderimin arasına giremez, olay kaybolmaz ve iki kez gelmez. Dinleyici işleyicinin
// görevi bittikten sonra Go'nun goroutine'inde, kilitsiz yazar (listen): halkadan imlecinden sonrakileri
// okur. Kaçırdığı olay halkadan düşerse ya da bir yazma slowWrite'ı aşarsa yalnız onun bağlantısı
// kapanır (olay atlanmaz; istemci Last-Event-ID ile yeniden bağlanır). Gönderim ağı beklemez.
// Olay kimliği yayıncıya özgüdür: "<ön ek>-<sıra>"; ön eki tutmayan Last-Event-ID (başka yayıncının ya da
// yeniden başlamadan önceki sunucunun kimliği) geri oynatmaz, görüntü verir.

var (
	nameCapacity = bind.NamedArg(codes.ArgCapacity)
	nameSnapshot = bind.NamedArg(codes.ArgSnapshot)
	nameReduce   = bind.NamedArg(codes.ArgReduce)
)

// broadcastBeat yayın akışının varsayılan nabzının verisidir (28.7).
var broadcastBeat = []byte(`{"type":"heartbeat"}`)

// event saklanan bir olaydır: kimliği ve yazılacak çerçevesi (id ve data satırları).
type event struct {
	id    int64
	frame []byte
}

// EventBroadcast TR_EventBroadcast'tır.
type EventBroadcast struct {
	capacity int
	reduce   *interp.FuncVal
	beat     []byte
	every    time.Duration
	prefix   string // kimliklerin ön eki (newPrefix)

	sem  chan struct{} // yayıncının dışlaması: gönderim ve abonelik
	snap *interp.JSONVal

	mu        sync.Mutex // aşağıdakiler: dinleyici goroutine'leri kilitsiz okur
	holder    *interp.VM // dışlamayı tutan görev (katlamanın kendine göndermesi)
	last      int64
	ring      []event
	listeners map[chan struct{}]struct{}
}

// EventBroadcastType TR_EventBroadcast'ın tip adıdır.
var EventBroadcastType = interp.TypeName(codes.TypeEventBroadcast)

func (b *EventBroadcast) Type() interp.ValueType { return EventBroadcastType }
func (b *EventBroadcast) IsTruthy() bool         { return true }

// Inspect değerin metnidir (3.14): tipin adı.
func (b *EventBroadcast) Inspect() string { return codes.ValueNames().Name(codes.TypeEventBroadcast) }

var eventBroadcastMembers = []codes.Code{codes.MemSendEvent, codes.MemSubscribe}

// GetProperty y.TR_send_event(veri) ve y.TR_subscribe(istek)'i verir.
func (b *EventBroadcast) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	switch kd {
	case codes.MemSendEvent:
		return bind.CodeMethod(kd, true, b.sendEvent), nil
	case codes.MemSubscribe:
		return bind.CodeMethod(kd, true, b.subscribe), nil
	}
	return nil, bind.MemberError(EventBroadcastType, name, eventBroadcastMembers)
}

// newBroadcast TR_EventBroadcast(TR_capacity: n, TR_snapshot: g, TR_reduce: f, TR_heartbeat: v)'dir: üçü
// zorunludur; n pozitif bir TR_Int, g bir TR_JSON, f iki parametreli bir fonksiyon.
func newBroadcast(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.TypeEventBroadcast)
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.TypeEventBroadcast, 0, len(args))
	}
	if err := bind.CheckNamed(fn, named, nameCapacity, nameSnapshot, nameReduce, nameHeartbeat); err != nil {
		return nil, err
	}
	for _, a := range []bind.Named{nameCapacity, nameSnapshot, nameReduce} {
		if _, ok := bind.GetNamed(named, a); !ok {
			return nil, messages.New(messages.H7637, fn, a)
		}
	}
	n, err := positiveArg(fn, named, nameCapacity, 0)
	if err != nil {
		return nil, err
	}
	v, _ := bind.GetNamed(named, nameSnapshot)
	snap, ok := v.(*interp.JSONVal)
	if !ok {
		return nil, namedTypeError(fn, nameSnapshot, v, codes.TypeJSON)
	}
	v, _ = bind.GetNamed(named, nameReduce)
	reduce, ok := v.(*interp.FuncVal)
	if !ok {
		return nil, namedTypeError(fn, nameReduce, v, codes.TypeFunction)
	}
	if len(reduce.Parameters) != 2 {
		return nil, messages.New(messages.H7638, fn, len(reduce.Parameters))
	}
	b := &EventBroadcast{capacity: int(n), reduce: reduce, beat: dataFrame(broadcastBeat), every: heartbeatInterval,
		prefix: newPrefix(), sem: make(chan struct{}, 1), snap: snap, listeners: map[chan struct{}]struct{}{}}
	if v, ok := bind.GetNamed(named, nameHeartbeat); ok {
		data, err := eventData(m, v)
		if err != nil {
			return nil, err
		}
		b.beat = dataFrame(data)
	}
	return b, nil
}

// acquire yayıncının dışlamasını alır, kilidi bırakarak bekler (28.7 (d)). Dışlamayı tutan görevin
// kendisi (katlama aynı yayıncıya gönderiyor ya da abone ediyor) çağrı hatası alır: kendini beklerdi.
func (b *EventBroadcast) acquire(m *interp.VM) (err error) {
	b.mu.Lock()
	self := b.holder == m
	b.mu.Unlock()
	if self {
		return messages.New(messages.H7641, interp.CallPrefix(codes.TypeEventBroadcast))
	}
	select {
	case b.sem <- struct{}{}:
	default:
		acquired := false
		defer func() {
			// Bekleme sonrası kesme (görev iptal edildi) dışlamayı tutarken çıkmasın.
			if r := recover(); r != nil {
				if acquired {
					<-b.sem
				}
				panic(r)
			}
		}()
		m.Blocking(func() {
			b.sem <- struct{}{}
			acquired = true
		})
	}
	b.mu.Lock()
	b.holder = m
	b.mu.Unlock()
	return nil
}

func (b *EventBroadcast) release() {
	b.mu.Lock()
	b.holder = nil
	b.mu.Unlock()
	<-b.sem
}

// sendEvent y.TR_send_event(veri)'dir (28.7 (c), (d)): maskeli JSON, katlama, kimlik, saklama ve
// dinleyicilere haber dışlamada birlikte. Katlama hatası ya da JSON olmayan sonucu çağrı hatasıdır:
// olay gitmez, görüntü önceki değerinde kalır.
func (b *EventBroadcast) sendEvent(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.MemSendEvent, 1, len(args))
	}
	if err := b.acquire(m); err != nil {
		return nil, err
	}
	defer b.release()
	data, err := eventData(m, args[0])
	if err != nil {
		return nil, err
	}
	ev, err := interp.JSONToTR(data)
	if err != nil {
		return nil, err
	}
	fn := interp.CallPrefix(codes.MemSendEvent)
	out, err := m.ApplyFunction(b.reduce, []interp.Value{b.snap, interp.NewJSON(ev)}, nil, m.BuiltinLine())
	if err != nil {
		if errors.Is(err, interp.ErrInterrupted) || errors.Is(err, interp.ErrHostClosed) {
			return nil, err
		}
		return nil, messages.New(messages.H7639, fn, err)
	}
	snap, ok := out.(*interp.JSONVal)
	if !ok {
		return nil, messages.New(messages.H7640, fn, out.Type())
	}
	b.snap = snap
	b.mu.Lock()
	b.last++
	b.ring = append(b.ring, event{id: b.last, frame: idFrame(b.prefix, b.last, data)})
	if len(b.ring) > b.capacity {
		b.ring = append(b.ring[:0:0], b.ring[len(b.ring)-b.capacity:]...)
	}
	for l := range b.listeners {
		select {
		case l <- struct{}{}:
		default:
		}
	}
	b.mu.Unlock()
	return interp.Null, nil
}

// newPrefix yayıncının kimlik ön ekidir: kurulurken rastgele 8 bayt, 16 onaltılık karakter. Yeniden
// üretilemez; yeniden başlayan sunucunun yayıncısı eski kimlikleri kendi aralığında saymaz (sayaç her
// yayıncıda 1'den başlar, ön eksiz kimlik yeni olayları eski durumun üstüne geri oynatırdı).
func newPrefix() string {
	var p [8]byte
	_, _ = rand.Read(p[:]) // crypto/rand.Read hata dönmez (Go 1.24)
	return hex.EncodeToString(p[:])
}

// idFrame kimlikli bir SSE olayıdır: id ("<ön ek>-<sıra>") ve data satırları.
func idFrame(prefix string, id int64, data []byte) []byte {
	f := make([]byte, 0, len(data)+len(prefix)+32)
	f = append(f, "id: "...)
	f = append(f, prefix...)
	f = append(f, '-')
	f = strconv.AppendInt(f, id, 10)
	f = append(f, "\ndata: "...)
	f = append(f, data...)
	return append(f, "\n\n"...)
}

// subscription dinleyicinin başlangıcıdır: önce yazılacak çerçeveler ve imleci (son yazdığı kimlik).
type subscription struct {
	b      *EventBroadcast
	frames [][]byte
	cursor int64
}

// subscribe y.TR_subscribe(istek)'tir (28.7): Last-Event-ID saklananlardan birini gösteriyorsa ondan
// sonrakiler, değilse görüntü (son kimlikle, 19.4'ün toplu maskesinden bir kez daha geçerek). Dışlamada:
// gönderim araya giremez.
func (b *EventBroadcast) subscribe(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.MemSubscribe, 1, len(args))
	}
	req, ok := args[0].(*Request)
	if !ok {
		return nil, interp.ArgTypeError(codes.MemSubscribe, 1, args[0], codes.TypeRequest)
	}
	if err := b.acquire(m); err != nil {
		return nil, err
	}
	defer b.release()
	sub := &subscription{b: b}
	b.mu.Lock()
	sub.cursor = b.last
	from, replay := b.replayFrom(req)
	if replay {
		for _, e := range b.ring {
			if e.id > from {
				sub.frames = append(sub.frames, e.frame)
			}
		}
	}
	b.mu.Unlock()
	if !replay {
		data, err := eventData(m, b.snap)
		if err != nil {
			return nil, err
		}
		sub.frames = [][]byte{idFrame(b.prefix, sub.cursor, data)}
	}
	h := newHeaders()
	h.add("Content-Type", "text/event-stream; charset=utf-8")
	h.add("Cache-Control", "no-store") // 28.7: akış saklanmaz
	return &Response{status: http.StatusOK, headers: h, stream: &streamSpec{sub: sub, beat: b.beat, every: b.every}}, nil
}

// replayFrom Last-Event-ID'yi okur (b.mu tutulurken): bu yayıncının ön ekini ve pozitif bir sırayı
// taşıyorsa ve sıra saklananların bir öncesinden son kimliğe kadarsa geri oynatılır. Yoksa, ön ek
// başkasınınsa, sıra sayı değilse ya da dışındaysa görüntü.
func (b *EventBroadcast) replayFrom(req *Request) (int64, bool) {
	s, ok := req.headers.get("Last-Event-ID")
	n, found := strings.CutPrefix(s, b.prefix+"-")
	if !ok || !found || n == "" || len(n) > 18 {
		return 0, false
	}
	for _, c := range n {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	id, _ := strconv.ParseInt(n, 10, 64)
	oldest := b.last + 1
	if len(b.ring) > 0 {
		oldest = b.ring[0].id
	}
	return id, id > 0 && id >= oldest-1 && id <= b.last
}

// since imleçten sonraki olayların çerçeveleridir ve yeni imleç; kaçırılan olay halkadan düştüyse ok
// yanlıştır (dinleyici çok geride: bağlantısı kapanır, olay atlanmaz).
func (b *EventBroadcast) since(cursor int64) (frames [][]byte, next int64, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if cursor == b.last {
		return nil, cursor, true
	}
	if len(b.ring) == 0 || b.ring[0].id > cursor+1 {
		return nil, cursor, false
	}
	for _, e := range b.ring {
		if e.id > cursor {
			frames = append(frames, e.frame)
		}
	}
	return frames, b.last, true
}

// listen dinleyicinin cevabını yazar, işleyicinin görevi bittikten sonra, kilitsiz (28.7): başlıklar,
// başlangıç çerçeveleri, sonra yeni olaylar ve nabız; istemci gidene, kapanış başlayana, dinleyici çok
// geride kalana ya da bir yazma takılana kadar. HEAD'de dinleyici kaydedilmez.
func (s *HTTPServer) listen(w http.ResponseWriter, r *http.Request, res *Response) {
	sub := res.stream.sub
	rc := http.NewResponseController(w)
	write := func(frame []byte) bool {
		_ = rc.SetWriteDeadline(time.Now().Add(slowWrite))
		_, err := w.Write(frame)
		if err == nil {
			err = rc.Flush()
		}
		return err == nil
	}
	res.writeHeaders(w)
	w.WriteHeader(http.StatusOK)
	if rc.Flush() != nil || r.Method == http.MethodHead {
		return
	}
	notify := make(chan struct{}, 1)
	b := sub.b
	b.mu.Lock()
	b.listeners[notify] = struct{}{}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.listeners, notify)
		b.mu.Unlock()
	}()
	// Abonelikle kayıt arasında gönderilenler de halkada: imleçten sonrakiler hemen yazılır.
	more, cursor, ok := b.since(sub.cursor)
	if !ok {
		return
	}
	frames := append(sub.frames, more...)
	t := time.NewTicker(res.stream.every)
	defer t.Stop()
	for {
		for _, f := range frames {
			if !write(f) {
				return
			}
		}
		select {
		case <-notify:
		case <-t.C:
			if !write(res.stream.beat) {
				return
			}
		case <-r.Context().Done():
			return
		case <-s.stop:
			return // kapanış: cevap düzgün biter, istemci Last-Event-ID ile döner (28.8)
		}
		if frames, cursor, ok = b.since(cursor); !ok {
			return
		}
	}
}

var _ interp.PropertyGetter = (*EventBroadcast)(nil)
