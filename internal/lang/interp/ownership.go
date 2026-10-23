package interp

import (
	"os"
	"sync/atomic"
)

// --- SAHİPLİK: BELİRLENİMCİ BIRAKMA (dil tanımı 15.43) ---
//
// Bazı motor değerleri (tensör) Go'nun çöp toplayıcısının görmediği bir kaynak tutar (cihaz
// belleği). Yorumlayıcı böyle bir değere artık hiçbir yoldan ulaşılamayacağını bildiği anda
// kaynağı bırakır (Owned.Release); bilemediği her durumda değer çöp toplayıcıya kalır.
//
// Kurallar:
//   - Taze değer: bir işlemin (işleç, yerleşik, motor metodu, indeks) o çağrıda ürettiği
//     değer. Motor bunu işlemi biterken MarkFresh ile bildirir; yorumlayıcı derlenmiş kodun
//     çağrı yerinde değeri sahiplenir (adopt): değer o anda çalışan fonksiyon çağrısının
//     çerçevesine (ownerFrame) girer. Sahiplenilmeyen taze işareti bir sonraki çağrıda
//     geçersizdir (damga, ownGen).
//   - Bağ: değer yerel bir ortamın (kapanışın göremediği) yuvasına yazılınca, sahip bir
//     listenin elemanı olunca ya da bir iş sürerken korunması gerekince (pin) bağ sayısı
//     artar; yuva başka değer alınca, ortam kapanınca, liste bırakılınca azalır.
//   - Kaçış: değer bir nesne alanına, sözlüğe, sahipsiz bir listeye, genel/modül ya da bir
//     kapanışın gördüğü ortama yazılırsa, argümanlarını saklayabilecek bir yerleşiğe
//     verilirse ya da Go'dan çağrılan bir fonksiyonun sonucu olursa kaçar: bir daha
//     belirlenimci bırakılmaz (çöp toplayıcının olur). Kaçış kalıcıdır.
//   - Dönüş: fonksiyonun döndürdüğü, kendi çerçevesindeki değer çağıranın çerçevesine geçer.
//   - Bırakma: çerçevenin bekleyenleri (pending) deyim sınırlarında, döngü turu başında ve
//     çerçeve kapanırken denetlenir; bağı sıfır, kaçmamış ve hâlâ o çerçevede olan değer
//     bırakılır. Bir işlemin ara sonucu (işlenen ya da saf yerleşiğin argümanı) işlem
//     biter bitmez bırakılır (releaseTemp): bağı sıfır bir değere yalnızca onu taşıyan
//     ifade ulaşabilir.
//   - Bir ifadenin içinde, aynı çerçevede deyim çalıştıran bir yapı (iç içe 'eğer'/'eşle')
//     varsa deyim sınırı bırakması o yapı bitene kadar ertelenir (ownerFrame.nested): dıştaki
//     ifadenin henüz kullanılmamış ara sonuçları bekleyenler arasındadır.
//
// Karşılaştırma: TensorFlow.js'in tf.tidy'si ve TorchSharp'ın DisposeScope'u kapsamın ara
// tensörlerini siler ama dönüş değerini ve saklanacakları kullanıcının işaretlemesini ister
// (tf.keep, MoveToOuterDisposeScope); burada dönüş kendiliğinden çağırana geçer, kaçış
// kendiliğinden algılanır. Ara sonuçların işlemden hemen sonra bırakılması Mojo'nun "ASAP"
// yok etmesine benzer. TR_RELEASE=0 sahiplenmeyi kapatır (tanılama: yalnızca çöp toplayıcı).

// Owned yorumlayıcının belirlenimci bırakabileceği bir değerdir.
type Owned interface {
	Value
	// Ownership değerin sahiplik durumudur (değerin kendi alanı).
	Ownership() *Ownership
	// Release kaynağı bırakır; yorumlayıcı en çok bir kez çağırır, değer bundan sonra
	// kullanılmaz.
	Release()
}

type ownState uint8

const (
	ownNone     ownState = iota // sahiplik dışı (taze işaretlenmiş olabilir)
	ownHeld                     // bir çerçevenin sahipliğinde
	ownEscaped                  // kaçtı: çöp toplayıcının
	ownReleased                 // bırakıldı
)

// Ownership bir değerin sahiplik durumudur; Owned değerler bunu bir alan olarak taşır.
// Sıfır değeri sahiplik dışıdır.
type Ownership struct {
	frame *ownerFrame
	gen   uint64 // MarkFresh damgası (0: taze değil)
	binds int32
	state ownState
}

// ownerFrame bir fonksiyon çağrısının (ya da programın üst düzeyinin) sahip olduğu
// değerlerdir. Yalnızca bekleyenler tutulur: sahiplenilen ve bağı sıfıra inen değerler.
type ownerFrame struct {
	pending []Owned
	// nested sıfırdan büyükse çerçevede bir ifadenin içinde deyim çalışıyordur; deyim
	// sınırında bırakma yapılmaz (bkz. paket açıklaması).
	nested int
}

// plain en sık sahiplenilemeyen değerlerdir (yok, sayı, mantık, metin); satır içine alınır.
func plain(val Value) bool {
	switch val.(type) {
	case nil, *IntVal, *FloatVal, *BoolVal, *StringVal:
		return true
	}
	return false
}

// ownershipOf değerin sahiplik durumunu verir (sahiplenilemeyen değerde nil). Yuva yazımı
// sıcak yoldur: en sık değerler (yok, sayı, mantık, metin) somut türle hemen elenir, arayüz
// dönüşümüne yalnızca diğerleri gider.
func ownershipOf(val Value) (Owned, *Ownership) {
	if plain(val) {
		return nil, nil
	}
	if o, ok := val.(Owned); ok {
		return o, o.Ownership()
	}
	return nil, nil
}

// ownershipOff TR_RELEASE=0 ise doğrudur (sahiplenme yok, her şey çöp toplayıcının).
var ownershipOff = os.Getenv("TR_RELEASE") == "0"

// ownActive ilk değer sahiplenilince doğru olur (süreç genelidir, atomiktir; okuması düz
// bir yüklemedir). Ondan önce hiçbir değer bir çerçevenin sahipliğinde değildir: bağ, bağ
// kaldırma ve kaçış yapacak bir şey yoktur, yuva yazımı sahiplik denetimi yapmaz (sahipli
// değer üretmeyen programlar bedel ödemez).
var ownActive atomic.Bool

// ownGen derlenmiş kodun her işlem çağrısından önce artan damgadır. Motor bir değeri
// işlem sürerken taze işaretler (MarkFresh); çağrı yeri değeri ancak damga değişmeden
// sahiplenir, eski bir işaret geçersizdir. Sayaç süreç genelidir (ayrı kilitli birden çok VM
// aynı anda çalışabilir; atomiktir): başka bir işlemin artırması yalnızca bir sahiplenmeyi
// kaçırtır (değer çöp toplayıcıya kalır), yanlış bir değeri sahiplendirmez; bir işleme
// başkasının değeri dönmez.
var ownGen atomic.Uint64

// MarkFresh motorun çalışan işlemde ürettiği değeri taze olarak işaretler; derlenmiş kodun
// çağrı yeri onu sahiplenir. Sahiplik dışı olmayan (daha önce sahiplenilmiş ya da kaçmış)
// değerde etkisizdir. Bir listeyi işaretlemek onu elemanlarının sahibi yapar (ör. tensörün
// .böl'ü); elemanlar ayrıca işaretlenmelidir.
func MarkFresh(o Owned) {
	if own := o.Ownership(); own.state == ownNone {
		own.gen = ownGen.Load()
	}
}

// newOp derlenmiş kodun bir işlem çağrısından önce damgayı yeniler.
func newOp() { ownGen.Add(1) }

// frameAt çağrı derinliği d'nin çerçevesini verir (yoksa açar).
func (v *VM) frameAt(d int) *ownerFrame {
	for len(v.owners) <= d {
		v.owners = append(v.owners, nil)
	}
	f := v.owners[d]
	if f == nil {
		f = &ownerFrame{}
		v.owners[d] = f
	}
	return f
}

// topFrame çalışan çağrının çerçevesi (yoksa nil).
func (v *VM) topFrame() *ownerFrame {
	if d := len(v.CallStack); d < len(v.owners) {
		return v.owners[d]
	}
	return nil
}

// beginCall yeni bir çağrının (derinlik d) çerçevesini sıfırlar: yarıda kalmış eski bir
// çağrının değerleri sahipsiz kalır (çöp toplayıcının olur).
func (v *VM) beginCall(d int) {
	if ownActive.Load() && d < len(v.owners) {
		v.owners[d] = nil
	}
}

// endCall derinlik d'deki çağrıyı kapatır: dönüş değeri (çağrının çerçevesindeyse)
// çağıranın çerçevesine geçer, sonra çerçevenin bekleyenleri bırakılır.
func (v *VM) endCall(d int, ret Value) {
	if ownActive.Load() {
		v.closeFrame(d, ret)
	}
}

func (v *VM) closeFrame(d int, ret Value) {
	if d >= len(v.owners) || v.owners[d] == nil {
		return
	}
	f := v.owners[d]
	v.owners[d] = nil
	if ret != nil {
		v.moveTo(ret, f, d-1)
	}
	v.flush(f)
}

// moveTo f çerçevesindeki değeri (sahip listeyse elemanlarıyla) derinlik d'nin çerçevesine
// taşır.
func (v *VM) moveTo(val Value, f *ownerFrame, d int) {
	o, own := ownershipOf(val)
	if own == nil || own.state != ownHeld || own.frame != f {
		return
	}
	to := v.frameAt(d)
	own.frame = to
	if own.binds == 0 {
		to.pending = append(to.pending, o)
	}
	if l, ok := val.(*ListVal); ok {
		for _, e := range l.Elements {
			v.moveTo(e, f, d)
		}
	}
}

// adopt derlenmiş kodun çağrı yerinde taze değeri çalışan çağrının çerçevesine alır; sahip
// listenin elemanları önce alınır ve listeye bağlanır.
func (v *VM) adopt(val Value) {
	o, own := ownershipOf(val)
	if own == nil || ownershipOff {
		return
	}
	if own.state != ownNone || own.gen == 0 || own.gen != ownGen.Load() {
		return
	}
	ownActive.Store(true)
	f := v.frameAt(len(v.CallStack))
	if l, ok := val.(*ListVal); ok {
		for _, e := range l.Elements {
			v.adopt(e)
			bind(e)
		}
	}
	own.state, own.frame = ownHeld, f
	f.pending = append(f.pending, o)
}

// adoptList derleyicinin kurduğu listeyi (liste yazımı, liste üreteci) elemanlarının sahibi
// yapar: liste çalışan çağrının çerçevesine girer, sahiplenilmiş elemanlar listeye bağlanır.
func (v *VM) adoptList(l *ListVal) {
	if ownershipOff {
		return
	}
	for _, e := range l.Elements {
		bind(e)
	}
	f := v.frameAt(len(v.CallStack))
	l.own.state, l.own.frame = ownHeld, f
	f.pending = append(f.pending, l)
}

// bind değere bir bağ ekler (sahiplenilmiş değerlerde).
func bind(val Value) {
	if !ownActive.Load() {
		return
	}
	if _, own := ownershipOf(val); own != nil && own.state == ownHeld {
		own.binds++
	}
}

// unbind değerin bir bağını kaldırır; bağ sıfıra inerse değer çerçevesinin bekleyenlerine
// girer.
func unbind(val Value) {
	if !ownActive.Load() {
		return
	}
	if o, own := ownershipOf(val); own != nil && own.state == ownHeld {
		if own.binds--; own.binds == 0 {
			own.frame.pending = append(own.frame.pending, o)
		}
	}
}

// escape değeri (sahip listeyse elemanlarıyla) kalıcı olarak sahiplik dışına çıkarır.
func escape(val Value) {
	if !ownActive.Load() {
		return
	}
	_, own := ownershipOf(val)
	if own == nil {
		return
	}
	switch own.state {
	case ownEscaped, ownReleased:
		return
	}
	held := own.state == ownHeld
	own.state, own.frame = ownEscaped, nil
	if l, ok := val.(*ListVal); ok && held {
		for _, e := range l.Elements {
			escape(e)
		}
	}
}

// escapeAll argümanların hepsini kaçırır (argümanlarını saklayabilecek yerleşik).
func escapeAll(args []Value, named map[string]Value) {
	for _, a := range args {
		escape(a)
	}
	for _, a := range named {
		escape(a)
	}
}

// releaseTemp bir işlemin ara sonucunu (işleneni ya da saf yerleşiğin argümanı) işlem
// biter bitmez bırakır: bağı sıfır ve çalışan çağrının çerçevesindeyse ona yalnızca bu
// ifade ulaşıyordu. Sonucun kendisi bırakılmaz.
func (v *VM) releaseTemp(val, result Value) {
	o, own := ownershipOf(val)
	if own == nil || val == result {
		return
	}
	if own.state == ownHeld && own.binds == 0 && own.frame == v.topFrame() {
		release(o)
	}
}

// release değeri bırakır; sahip listenin elemanlarının bağı kalkar.
func release(o Owned) {
	own := o.Ownership()
	own.state, own.frame = ownReleased, nil
	o.Release()
}

// flush çerçevenin bekleyenlerinden bağı sıfır kalanları bırakır (bir listenin bırakılması
// elemanlarını da bekleyenlere ekleyebilir).
func (v *VM) flush(f *ownerFrame) {
	for len(f.pending) > 0 {
		n := len(f.pending) - 1
		o := f.pending[n]
		f.pending[n] = nil
		f.pending = f.pending[:n]
		if own := o.Ownership(); own.state == ownHeld && own.frame == f && own.binds == 0 {
			release(o)
		}
	}
}

// boundary deyim sınırında çalışan çağrının bekleyenlerini bırakır (çerçeve bir ifadenin
// içinde değilse).
func (v *VM) boundary() {
	if ownActive.Load() {
		v.flushTop()
	}
}

func (v *VM) flushTop() {
	if d := len(v.CallStack); d < len(v.owners) {
		if f := v.owners[d]; f != nil && f.nested == 0 && len(f.pending) > 0 {
			v.flush(f)
		}
	}
}

var _ Owned = (*ListVal)(nil)

// Ownership (Owned) listenin sahiplik durumu: sahip liste elemanlarının sahibidir.
func (l *ListVal) Ownership() *Ownership { return &l.own }

// Release (Owned) sahip listenin elemanlarındaki bağını kaldırır.
func (l *ListVal) Release() {
	for _, e := range l.Elements {
		unbind(e)
	}
}

// pin bir işin sürdüğü süre değeri korur (bağ ekler); unpin kaldırır.
func pin(val Value)   { bind(val) }
func unpin(val Value) { unbind(val) }

// envOwns ortamın yuvalarına yazılan değerlerin bağlanıp bağlanmadığı: yalnızca derleyicinin
// kapanışsız bildiği yerel ortamlar bağlar, diğerlerine yazılan değer kaçar.
func (e *Environment) envOwns() bool { return e.scope != nil && e.scope.local }

// store i. yuvaya değeri yazar ve sahipliğini günceller: yerel ortamda yeni değer bağlanır,
// eskisinin bağı kalkar; diğer ortamlarda yeni değer kaçar.
func (e *Environment) store(i int, val Value) {
	old := e.vals[i]
	e.vals[i] = val
	if ownActive.Load() && !(plain(val) && plain(old)) {
		e.storeOwned(old, val)
	}
}

func (e *Environment) storeOwned(old, val Value) {
	if e.envOwns() {
		bind(val)
		unbind(old)
		return
	}
	escape(val)
}

// closeEnv kapanan yerel ortamın yuvalarındaki bağları kaldırır.
func (e *Environment) closeEnv() {
	if ownActive.Load() && e.envOwns() {
		e.unbindSlots()
	}
}

func (e *Environment) unbindSlots() {
	for _, x := range e.vals {
		if !plain(x) {
			unbind(x)
		}
	}
}

// infix derlenmiş bir ikili işlemi hesaplar: taze sonuç sahiplenilir, işlenenlerden ara
// sonuç olanlar hemen bırakılır. İki listenin birleşimi işlenenlerin elemanlarını yeni bir
// (sahipsiz) listeye koyduğundan işlenen listeler kaçar.
func (v *VM) infix(codeVal uint8, op string, left, right Value, line int) (Value, error) {
	newOp()
	res, err := v.evalInfixKod(codeVal, op, left, right, line)
	if err != nil {
		return nil, err
	}
	if _, list := res.(*ListVal); list {
		escape(left)
		escape(right)
		return res, nil
	}
	v.adopt(res)
	v.releaseTemp(left, res)
	v.releaseTemp(right, res)
	return res, nil
}

// afterIndex indeks okumasının taze sonucunu (tensör dilimi) sahiplenir. Listenin dilimi
// elemanlarını yeni bir listeye koyduğundan kaynak liste kaçar; ara sonuç olan kaynak (ör.
// bir işlemin döndürdüğü liste) bırakılır; okunan eleman o listeye bağlı olduğundan
// deyim sınırına kadar korunur.
func (v *VM) afterIndex(res, left, idx Value) {
	if _, list := res.(*ListVal); list {
		if _, src := left.(*ListVal); src {
			escape(left)
			return
		}
	}
	v.adopt(res)
	v.releaseTemp(left, res)
	v.releaseTemp(idx, res)
}

// ownList derleyicinin kurduğu listeyi (liste yazımı, liste üreteci) sahiplenilmiş bir
// eleman taşıyorsa elemanlarının sahibi yapar.
func (v *VM) ownList(l *ListVal) {
	for _, e := range l.Elements {
		if _, own := ownershipOf(e); own != nil && own.state == ownHeld {
			v.adoptList(l)
			return
		}
	}
}

// nestedStatements bir ifadenin içinde deyim çalıştıran yapıyı ('eğer'/'eşle') sarar: yapı
// sürerken çalışan çağrının çerçevesinde deyim sınırı bırakması yapılmaz (dıştaki ifadenin
// ara sonuçları bekleyenler arasındadır).
func nestedStatements(d exprCode) exprCode {
	return func(v *VM, env *Environment) (Value, error) {
		f := v.frameAt(len(v.CallStack))
		f.nested++
		val, err := d(v, env)
		f.nested--
		return val, err
	}
}

// AssignCopier, x[i] = değer atamasında değeri saklamayıp kopyalayan dış değerlerin
// (tensör) uyguladığı arayüzdür: atanan değer kaçmaz, ara sonuçsa bırakılır.
type AssignCopier interface {
	CopiesAssigned()
}
