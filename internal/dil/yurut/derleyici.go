package yurut

import (
	"tr_dili/internal/dil/agac"
)

// --- KAPANIŞ DERLEYİCİSİ ---
//
// AST, çalıştırılmadan önce bir kez Go kapanışlarına (closure) çevrilir; böylece ağaç her
// turda yeniden dolaşılmaz, işleç metinleri ve sabitler derleme anında çözülür. Değişkenler
// mümkün olduğunda derleme anında (derinlik, yuva) çiftine çözülür ve çalışma anında ad
// araması yapılmaz.
//
// Derleme anındaki her kapsam (derleyiciKapsam) çalışma anındaki tam olarak bir ortama
// karşılık gelir: fonksiyon çağrısı, ad tanımlayan blok, döngü turu, yakala bloğu,
// eşzamanlı/ertele gövdesi, liste üreteci öğesi, modül ve genel kapsam. Ad tanımlamayan
// bloklar ortam açmaz ve kapsam da oluşturmaz. Bu birebirlik sayesinde derinlik çalışma
// anında da geçerlidir.
//
// Bir yuva nil ise (ad henüz tanımlanmamış: tanım ilerideyse ya da hata ayıklayıcı
// 'zıpla' ile atlandıysa) arama adla, ağaç yorumlayıcısındaki gibi sürdürülür; bu yüzden
// anlam değişmez.

type ifadeKodu func(v *VM, env *Environment) (Value, error)
type deyimKodu func(v *VM, env *Environment) (Value, flowSignal, error)

// derleyiciKapsam derleme anındaki bir kapsamdır.
type derleyiciKapsam struct {
	// bilgi kapsamın ad tablosudur. nil ise kapsam, düzeni derleme anında bilinmeyen ve
	// çalışma anında verilen bir ortamdır (hata ayıklayıcının değerlendirdiği ifadeler):
	// buradan geçen adlar adla aranır, buradaki tanımlar adla yapılır.
	bilgi *kapsamBilgisi
	ust   *derleyiciKapsam
	// genel, bu kapsam genel kapsamsa onun ortamıdır; genel adlara derinlik yürümeden
	// doğrudan erişilir.
	genel *Environment
	// dinamik: kapsamda takma adsız bir 'aktar' var; adları çalışma anında eklenir ve dış
	// kapsamlardaki aynı adları gölgeleyebilir. Bu kapsamdan geçen adlar adla aranır.
	dinamik bool
	// golge bu kapsamda yerleşik adlarını gölgeleyen kullanıcı tanımlarıdır (golge.go);
	// yalnızca hata mesajı için tutulur, çoğu kapsamda nil'dir.
	golge map[string]golgeTanimi
	// henuzYok bir parametrenin varsayılan değeri derlenirken o parametre ve sonrakilerin
	// adlarıdır: varsayılan yalnızca önceki parametreleri görür; bu adlar dış kapsamdaki
	// aynı adlı değişkene çözülmez, okunursa hata verir.
	henuzYok map[string]bool
}

// tanimla bir adı kapsamın tablosuna ekler (derleme anı).
func (k *derleyiciKapsam) tanimla(ad string) {
	if ad == "" {
		k.dinamik = true
		return
	}
	if k.bilgi != nil {
		k.bilgi.yuva(ad)
	}
}

// onTanimla deyimlerin bu kapsama tanımlayacağı tüm adlara, deyimler derlenmeden önce yuva
// ayırır. Böylece tanımdan önce gelen bir kullanım da (ör. hata ayıklayıcı geri atladığında)
// doğru yuvaya çözülür.
func (k *derleyiciKapsam) onTanimla(stmts []agac.Statement) {
	for _, st := range stmts {
		deyimAdlari(st, k.tanimla)
		k.deyimGolgeleri(st)
	}
}

const (
	cozDinamik = iota
	cozYerel
	cozGenel
	cozHenuzYok // varsayılan değerde henüz bağlanmamış bir parametre (bkz. henuzYok)
)

type cozum struct {
	tur      int
	derinlik int
	yuva     int
	genel    *Environment
}

// coz bir adı derleme anında çözer.
func (k *derleyiciKapsam) coz(ad string) cozum {
	d := 0
	for c := k; c != nil; c = c.ust {
		if c.henuzYok[ad] {
			return cozum{tur: cozHenuzYok}
		}
		if c.bilgi == nil {
			return cozum{}
		}
		if i, ok := c.bilgi.indeks[ad]; ok {
			if c.genel != nil {
				return cozum{tur: cozGenel, yuva: i, genel: c.genel}
			}
			return cozum{tur: cozYerel, derinlik: d, yuva: i}
		}
		if c.dinamik {
			return cozum{}
		}
		d++
	}
	return cozum{}
}

// derleyici bir derleme işinin durumudur.
type derleyici struct {
	// kok genel kapsamdır (yapı alanlarının varsayılanları burada değerlendirilir).
	kok *derleyiciKapsam
	// kaynak derlenen kodun dosyasıdır (nil: ana program); fonksiyonlar onu taşır.
	kaynak *Kaynak
}

// yeniDerleyici görevin o an çalıştığı dosya için bir derleyici açar.
func (v *VM) yeniDerleyici() *derleyici {
	return &derleyici{kok: &derleyiciKapsam{bilgi: v.GlobalEnv.kapsam, genel: v.GlobalEnv}, kaynak: v.kaynak}
}

// altKapsam ust içinde yeni bir ortama karşılık gelen kapsam açar.
func altKapsam(ust *derleyiciKapsam) *derleyiciKapsam {
	return &derleyiciKapsam{bilgi: &kapsamBilgisi{}, ust: ust}
}

// sinirKapsami çalışma anında verilecek, düzeni bilinmeyen bir ortamı temsil eder.
func sinirKapsami() *derleyiciKapsam {
	return &derleyiciKapsam{}
}

// tanimlayici k kapsamına ad tanımlayan bir işlev döndürür (yuvalı ya da adla).
func tanimlayici(k *derleyiciKapsam, ad string) func(env *Environment, val Value, sabit bool) {
	if k.bilgi == nil {
		return func(env *Environment, val Value, sabit bool) { env.Define(ad, val, sabit) }
	}
	i := k.bilgi.yuva(ad)
	return func(env *Environment, val Value, sabit bool) { env.tanimla(i, val, sabit) }
}

// --- BLOKLAR ---

// blokKodu derlenmiş bir deyim listesidir. Deyimlerin kendisi hata ayıklayıcı (her
// deyimden önce danışılır) ve 'zıpla' (satıra atlama) için saklanır.
type blokKodu struct {
	deyimler []agac.Statement
	kodlar   []deyimKodu
}

func (c *derleyici) deyimler(k *derleyiciKapsam, stmts []agac.Statement) *blokKodu {
	b := &blokKodu{deyimler: stmts, kodlar: make([]deyimKodu, len(stmts))}
	for i, st := range stmts {
		b.kodlar[i] = c.deyim(k, st)
	}
	return b
}

func (b *blokKodu) calistir(v *VM, env *Environment) (Value, flowSignal, error) {
	var lastVal Value = NULL
	kodlar := b.kodlar
	for i := 0; i < len(kodlar); i++ {
		if v.Debugger != nil {
			if j, zipla := b.ayiklayiciyaDanis(v, i, env); zipla {
				i = j - 1 // döngü sonunda i++ olacağı için j-1
				continue
			}
		}
		val, sig, err := kodlar[i](v, env)
		if err != nil {
			return nil, signalNone, err
		}
		if sig != signalNone {
			return val, sig, nil
		}
		lastVal = val
	}
	return lastVal, signalNone, nil
}

// ayiklayiciyaDanis i. deyimden önce hata ayıklayıcıya danışır; ayıklayıcı bloğun içinde
// bir satıra zıplamak isterse o deyimin indeksini ve doğru döner. Sıcak döngünün dışında
// tutulur (calistir'ın gövdesi küçük kalsın).
//
//go:noinline
func (b *blokKodu) ayiklayiciyaDanis(v *VM, i int, env *Environment) (int, bool) {
	stmt := b.deyimler[i]
	if stmt == nil {
		return 0, false
	}
	// kesme_noktası satırının kendisinde değil, ya da kesme_noktası tetiklendikten sonra dur
	if _, isBp := stmt.(*agac.BreakpointStatement); isBp {
		return 0, false
	}
	v.etkinBlok = b // hata ayıklayıcı zıplama hedeflerini bu bloğa göre sorar
	jumpLine := v.Debugger.BeforeStatement(v, stmt, env)
	if jumpLine <= 0 {
		return 0, false
	}
	hedef, ok := b.ziplamaHedefi(jumpLine)
	if !ok {
		return 0, false
	}
	for j, targetStmt := range b.deyimler {
		if targetStmt != nil && targetStmt.Line() == hedef {
			return j, true
		}
	}
	return 0, false
}

// zıplamaHedefi blokta satir'a zıplanabiliyorsa zıplanacak deyimin satırını verir: satırdaki
// ya da ondan sonraki ilk deyim; satır bloğun ilk deyiminden önceyse ya da son deyiminin
// başından sonraysa (blok dışı) zıplanamaz.
func (b *blokKodu) ziplamaHedefi(satir int) (int, bool) {
	ilk := true
	for _, st := range b.deyimler {
		if st == nil {
			continue
		}
		if ilk && satir < st.Line() {
			return 0, false
		}
		ilk = false
		if st.Line() >= satir {
			return st.Line(), true
		}
	}
	return 0, false
}

// ZiplamaHedefi hata ayıklayıcı duraklatıldığında, duraklanan bloğun içinde satir'a
// zıplanabiliyorsa zıplanacak deyimin satırını verir (düzenleyicinin "goto" isteği). Zıplama yalnızca
// şu an çalışan bloğun deyimleri arasında yapılabilir; blok dışındaki satırda ok yanlıştır.
// Duraklatılmış görevin kendi goroutine'inde çağrılmalıdır.
func (v *VM) ZiplamaHedefi(satir int) (int, bool) {
	if v.etkinBlok == nil {
		return 0, false
	}
	return v.etkinBlok.ziplamaHedefi(satir)
}

// blokDeyimi bir bloğu derler: blok ad tanımlıyorsa her çalışmada yeni bir ortam açar,
// tanımlamıyorsa çevredeki ortamda çalışır.
func (c *derleyici) blokDeyimi(k *derleyiciKapsam, b *agac.BlockStatement) deyimKodu {
	if b == nil {
		return bosDeyim
	}
	if blokBayraklari(b)&blokTanimlar == 0 {
		return c.deyimler(k, b.Statements).calistir
	}
	alt := altKapsam(k)
	alt.onTanimla(b.Statements)
	bk := c.deyimler(alt, b.Statements)
	bilgi := alt.bilgi
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		return bk.calistir(v, yeniOrtam(env, bilgi))
	}
}

func bosDeyim(v *VM, env *Environment) (Value, flowSignal, error) {
	return NULL, signalNone, nil
}

// --- GİRİŞ NOKTALARI ---

// programCalistir deyimleri genel kapsamda derleyip genel ortamda çalıştırır.
func (v *VM) programCalistir(stmts []agac.Statement) (Value, flowSignal, error) {
	c := v.yeniDerleyici()
	c.kok.onTanimla(stmts)
	return c.deyimler(c.kok, stmts).calistir(v, v.GlobalEnv)
}

// genelAltindaCalistir deyimleri genel kapsamın altında yeni bir ortamda çalıştırır
// (testler); ortam çağırana verilir ki 'ertele' blokları çalıştırılabilsin.
func (v *VM) genelAltindaDerle(stmts []agac.Statement) (*blokKodu, *kapsamBilgisi) {
	c := v.yeniDerleyici()
	alt := altKapsam(c.kok)
	alt.onTanimla(stmts)
	return c.deyimler(alt, stmts), alt.bilgi
}

// dinamikIfade ifadeyi, düzeni bilinmeyen bir ortamda (adla çözümlemeyle) değerlendirir.
func (v *VM) dinamikIfade(expr agac.Expression, env *Environment) (Value, error) {
	return v.yeniDerleyici().ifade(sinirKapsami(), expr)(v, env)
}

// dinamikDeyimler deyimleri, düzeni bilinmeyen bir ortamda (adla çözümlemeyle) çalıştırır.
func (v *VM) dinamikDeyimler(stmts []agac.Statement, env *Environment) (Value, flowSignal, error) {
	return v.yeniDerleyici().deyimler(sinirKapsami(), stmts).calistir(v, env)
}
