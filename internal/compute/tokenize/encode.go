package tokenize

import (
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// ============================================================================
// Kodlama (metin -> kimlik) ve çözme (kimlik -> metin)
// ============================================================================

// Options kodlama seçenekleridir. Sıfır değer, başvuru uygulamasının
// varsayılan davranışıdır: metindeki özel tokenlar tanınır ve son işleyici
// (varsa) özel tokenları (ör. BOS/EOS) ekler.
type Options struct {
	// DontParseSpecial doğruysa metindeki "<|end|>" gibi özel tokenlar
	// tanınmaz, sıradan metin gibi parçalanır (özel olmayan ek tokenlar yine
	// tanınır). Kullanıcıdan gelen metni güvenle kodlamak için uygundur.
	DontParseSpecial bool
	// DontAddSpecial doğruysa son işleyici özel token eklemez.
	DontAddSpecial bool
}

// Encode metni token kimliklerine çevirir (varsayılan seçeneklerle).
// Parçalayıcının bir düzenli ifadesi adım sınırını aşarsa *AdimSiniriHatasi
// ile panikler; kullanıcı girdisi işleyen sunucular GuvenliKodla kullanmalıdır.
func (p *Tokenizer) Encode(text string) []int {
	return p.EncodeWithOptions(text, Options{})
}

// EncodeWithOptions metni verilen seçeneklerle kodlar.
func (p *Tokenizer) EncodeWithOptions(text string, s Options) []int {
	return ToInt(nil, p.Encode32(nil, text, s))
}

// EncodeAppend metnin kimliklerini dst'nin sonuna ekler ve dst'yi döndürür.
// ozelleriTani yanlışsa özel tokenlar metin içinde tanınmaz.
func (p *Tokenizer) EncodeAppend(dst []int, text string, parseSpecial bool) []int {
	return ToInt(dst, p.Encode32(nil, text, Options{DontParseSpecial: !parseSpecial}))
}

// ToInt 32 bitlik kimlikleri int olarak dst'ye ekler (dst nil ise yeni dilim ayrılır).
func ToInt(dst []int, ids []int32) []int {
	if dst == nil {
		dst = make([]int, 0, len(ids))
	}
	for _, id := range ids {
		dst = append(dst, int(id))
	}
	return dst
}

// Encode32 metni 32 bitlik kimliklere çevirip dst'ye ekler (büyük derlemler
// için bellek dostu biçim).
func (p *Tokenizer) Encode32(dst []int32, text string, s Options) []int32 {
	if p.postProcessor == nil {
		return p.arr(dst, text, s.DontParseSpecial)
	}
	k := p.postProcessor.Process(&Encoding{IDs: p.arr(nil, text, s.DontParseSpecial)}, nil, !s.DontAddSpecial)
	return append(dst, k.IDs...)
}

// EncodePair bir metin çiftini (ör. soru ve bağlam) kodlar; ikinci nil ise
// tek metin kodlanır.
func (p *Tokenizer) EncodePair(firstArg string, second *string, s Options) *Encoding {
	a := &Encoding{IDs: p.arr(nil, firstArg, s.DontParseSpecial)}
	var b *Encoding
	if second != nil {
		b = &Encoding{IDs: p.arr(nil, *second, s.DontParseSpecial)}
	}
	if p.postProcessor == nil {
		k := defaultProcess(a, b)
		if b == nil {
			k = &Encoding{}
			k.add(a, 0, false)
		}
		return k
	}
	return p.postProcessor.Process(a, b, !s.DontAddSpecial)
}

// arr tek bir metnin (son işleme öncesi) kimliklerini dst'ye ekler.
func (p *Tokenizer) arr(dst []int32, text string, skipSpecial bool) []int32 {
	text = validText(text)
	modelEncode := func(x Piece) { dst = p.model.Encode(dst, x) }
	// Bütün düzenli ifade aramaları tek bir bütçeyi paylaşır: her biri kendi
	// sınırında kalan çok sayıda arama da toplamda metin uzunluğuyla orantılı
	// işte durdurulur.
	budget := p.encodeBudget(len(text))
	p.rawMatch.splitFn(text, skipSpecial, func(a addedPiece) {
		if a.id >= 0 {
			dst = append(dst, a.id)
			return
		}
		section := text[a.begin:a.last]
		first := a.begin == 0
		if p.normalizer != nil {
			section = normalizeBudgeted(p.normalizer, section, budget)
			if budget != nil {
				budget.add(p.encodeRegex * len(section))
			}
		}
		p.nrmMatch.splitFn(section, skipSpecial, func(b addedPiece) {
			if b.id >= 0 {
				dst = append(dst, b.id)
				return
			}
			if b.last <= b.begin {
				return
			}
			piece := Piece{Text: section[b.begin:b.last], Start: first && b.begin == 0, budget: budget}
			if p.preTokenizer == nil {
				dst = p.model.Encode(dst, piece)
				return
			}
			preTokenizeStream(p.preTokenizer, piece, modelEncode)
		})
	})
	return dst
}

// Tokens metnin token metinlerini (modelin gösterimiyle) verir.
func (p *Tokenizer) Tokens(text string) []string {
	ids := p.Encode(text)
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i], _ = p.IDToToken(id)
	}
	return out
}

// Decode kimlik dizisini metne çevirir. ozelleriAtla doğruysa özel tokenlar
// çıktıya yazılmaz. Sözlükte olmayan kimlikler sessizce atlanır. Bayt düzeyi
// çözücüde tokenların birleşimi geçerli UTF-8 değilse (ör. çok baytlı bir
// karakterin yarısı) bozuk diziler U+FFFD ile değiştirilir.
func (p *Tokenizer) Decode(idList []int, skipSpecial bool) string {
	if p.byteDecoder {
		buf := make([]byte, 0, len(idList)*4)
		for _, id := range idList {
			buf = p.addByte(buf, id, skipSpecial)
		}
		return lossyUTF8(buf)
	}
	tokenList := make([]string, 0, len(idList))
	for _, id := range idList {
		k := p.tokID(id)
		if k == nil || (skipSpecial && k.special) {
			continue
		}
		tokenList = append(tokenList, k.token)
	}
	if p.decoder == nil {
		return strings.Join(tokenList, " ")
	}
	n := 0
	for _, t := range tokenList {
		n += len(t)
	}
	budget := newBudget((p.parseRegex + 1) * n) // çözücülerin çıktı büyütmesi de bu bütçededir
	return strings.Join(decodeBudgeted(p.decoder, tokenList, budget), "")
}

// Decode32 32 bitlik kimlik dizisini metne çevirir (bkz. Coz).
func (p *Tokenizer) Decode32(idList []int32, skipSpecial bool) string {
	return p.Decode(ToInt(nil, idList), skipSpecial)
}

// DecodeBytes kimlik dizisinin ham baytlarını verir. Bayt düzeyi çözücüde UTF-8
// onarımı yapılmaz; akış hâlinde üretimde yarım karakterleri biriktirmek
// için kullanışlıdır. Diğer çözücülerde Coz'un sonucunun baytlarıdır.
func (p *Tokenizer) DecodeBytes(idList []int, skipSpecial bool) []byte {
	if !p.byteDecoder {
		return []byte(p.Decode(idList, skipSpecial))
	}
	buf := make([]byte, 0, len(idList)*4)
	for _, id := range idList {
		buf = p.addByte(buf, id, skipSpecial)
	}
	return buf
}

func (p *Tokenizer) addByte(buf []byte, id int, skipSpecial bool) []byte {
	k := p.tokID(id)
	if k == nil || (skipSpecial && k.special) {
		return buf
	}
	return append(buf, k.byteVal...)
}

// ============================================================================
// Toplu ve paralel işlem
// ============================================================================

// workerCount 0 ya da negatifse kullanılabilir işlemci sayısını verir.
func workerCount(n, is int) int {
	if n <= 0 {
		n = runtime.GOMAXPROCS(0)
	}
	if n > is {
		n = is
	}
	if n < 1 {
		n = 1
	}
	return n
}

// parallel 0..is-1 işlerini isci goroutine'e dağıtır.
func parallel(is, worker int, f func(i int)) {
	worker = workerCount(worker, is)
	if worker == 1 {
		for i := 0; i < is; i++ {
			f(i)
		}
		return
	}
	var counter atomic.Int64
	var wg sync.WaitGroup
	// Bir işçideki panik (ör. adım sınırı) süreci çökertmesin; çağıran
	// goroutine'de yeniden panik olarak yükseltilir.
	var panicOne sync.Once
	var panicValue any
	wg.Add(worker)
	for w := 0; w < worker; w++ {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicOne.Do(func() { panicValue = r })
					counter.Store(int64(is)) // kalan işleri bırak
				}
			}()
			for {
				i := int(counter.Add(1)) - 1
				if i >= is {
					return
				}
				f(i)
			}
		}()
	}
	wg.Wait()
	if panicValue != nil {
		panic(panicValue)
	}
}

// EncodeBatch metinleri paralel olarak kodlar; sonuç sırası girdi sırasıyla
// aynıdır. isci <= 0 ise tüm işlemciler kullanılır.
func (p *Tokenizer) EncodeBatch(texts []string, s Options, worker int) [][]int {
	out := make([][]int, len(texts))
	parallel(len(texts), worker, func(i int) {
		out[i] = p.EncodeWithOptions(texts[i], s)
	})
	return out
}

// EncodeBatch32 TopluKodla'nın 32 bitlik kimlik üreten biçimidir.
func (p *Tokenizer) EncodeBatch32(texts []string, s Options, worker int) [][]int32 {
	out := make([][]int32, len(texts))
	parallel(len(texts), worker, func(i int) {
		out[i] = p.Encode32(nil, texts[i], s)
	})
	return out
}

// DecodeBatch kimlik dizilerini paralel olarak çözer.
func (p *Tokenizer) DecodeBatch(arrs [][]int, skipSpecial bool, worker int) []string {
	out := make([]string, len(arrs))
	parallel(len(arrs), worker, func(i int) {
		out[i] = p.Decode(arrs[i], skipSpecial)
	})
	return out
}

// JoinDocuments belgeleri paralel kodlar ve her belgenin sonuna ayirici
// kimliğini (ör. <|endoftext|>) koyarak tek bir eğitim dizisi hâlinde
// birleştirir. ayirici < 0 ise ayırıcı eklenmez.
func (p *Tokenizer) JoinDocuments(docs []string, separator int, worker int) []int32 {
	pieces := p.EncodeBatch32(docs, Options{}, worker)
	total := 0
	for _, d := range pieces {
		total += len(d) + 1
	}
	out := make([]int32, 0, total)
	for _, d := range pieces {
		out = append(out, d...)
		if separator >= 0 {
			out = append(out, int32(separator))
		}
	}
	return out
}
