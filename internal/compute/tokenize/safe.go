package tokenize

// ============================================================================
// Hata döndüren (sunucu kullanımına uygun) kodlama/çözme işlevleri
// ============================================================================
//
// Düzenli ifade kullanan bileşenler (Split, Replace, bayt düzeyi desen) adım
// bütçesini aşarsa (bkz. duzifade_guvenlik.go) Kodla/Coz gibi hata
// döndürmeyen işlevler *AdimSiniriHatasi ile panikler. Kullanıcı girdisi
// işleyen sunucular aşağıdaki Guvenli... işlevlerini kullanmalıdır: bu
// durumda yarım ya da yanlış bir sonuç yerine açık bir hata döner.

// SafeEncode Kodla'nın hata döndüren biçimidir.
func (p *Tokenizer) SafeEncode(text string, s Options) (ids []int, err error) {
	err = catchStepError(func() { ids = p.EncodeWithOptions(text, s) })
	if err != nil {
		ids = nil
	}
	return
}

// SafeEncodePair KodlaCift'in hata döndüren biçimidir.
func (p *Tokenizer) SafeEncodePair(firstArg string, second *string, s Options) (k *Encoding, err error) {
	err = catchStepError(func() { k = p.EncodePair(firstArg, second, s) })
	if err != nil {
		k = nil
	}
	return
}

// SafeDecode Coz'un hata döndüren biçimidir (çözücüde düzenli ifade varsa).
func (p *Tokenizer) SafeDecode(idList []int, skipSpecial bool) (m string, err error) {
	err = catchStepError(func() { m = p.Decode(idList, skipSpecial) })
	return
}

// SafeEncodeBatch TopluKodla'nın hata döndüren biçimidir. Hatalı metinlerin
// sonucu nil'dir; hatalar[i] i. metnin hatasıdır (hiç hata yoksa dilim nil).
func (p *Tokenizer) SafeEncodeBatch(texts []string, s Options, worker int) ([][]int, []error) {
	out := make([][]int, len(texts))
	errs := make([]error, len(texts))
	parallel(len(texts), worker, func(i int) {
		out[i], errs[i] = p.SafeEncode(texts[i], s)
	})
	for _, h := range errs {
		if h != nil {
			return out, errs
		}
	}
	return out, nil
}

// SafeEncodePairBatch her texts[i]'yi seconds[i] ile çift olarak kodlar (len(seconds) == len(texts));
// hatalı çiftlerin sonucu nil'dir, errs[i] i. çiftin hatasıdır. Kimlikler int olarak verilir.
func (p *Tokenizer) SafeEncodePairBatch(texts, seconds []string, s Options, worker int) ([][]int, []error) {
	out := make([][]int, len(texts))
	errs := make([]error, len(texts))
	parallel(len(texts), worker, func(i int) {
		k, err := p.SafeEncodePair(texts[i], &seconds[i], s)
		errs[i] = err
		if err == nil {
			out[i] = ToInt(nil, k.IDs)
		}
	})
	return out, errs
}

// SafeDecodeBatch her kimlik dizisini metne çevirir (SafeDecode); errs[i] i. dizinin hatasıdır.
func (p *Tokenizer) SafeDecodeBatch(seqs [][]int, skipSpecial bool, worker int) ([]string, []error) {
	out := make([]string, len(seqs))
	errs := make([]error, len(seqs))
	parallel(len(seqs), worker, func(i int) {
		out[i], errs[i] = p.SafeDecode(seqs[i], skipSpecial)
	})
	return out, errs
}

// Warnings parçalayıcının düzenli ifadelerinde derleme sırasında bulunan
// risk uyarılarını verir (ör. sınırsız yineleme içinde çakışan seçenekler).
func (p *Tokenizer) Warnings() []string {
	var out []string
	sum := func(d *Regex) {
		for _, u := range d.Warnings() {
			out = append(out, d.source+": "+u)
		}
	}
	walkExprs(p.normalizer, sum)
	walkExprs(p.preTokenizer, sum)
	walkExprs(p.decoder, sum)
	return out
}

// walkExprs bir bileşen (zinciri) içindeki düzenli ifadeleri gezer.
func walkExprs(b any, f func(*Regex)) {
	switch x := b.(type) {
	case NormalizerSequence:
		for _, o := range x {
			walkExprs(o, f)
		}
	case PreTokenizerSequence:
		for _, o := range x {
			walkExprs(o, f)
		}
	case DecoderSequence:
		for _, o := range x {
			walkExprs(o, f)
		}
	case replaceNorm:
		if x.d.expr != nil {
			f(x.d.expr)
		}
	case replaceDecoder:
		if x.d.expr != nil {
			f(x.d.expr)
		}
	case *splitWithPattern:
		if x.expr != nil {
			f(x.expr)
		}
	case *bytePreTok:
		if re, err := byteLevelExpr(); x.withPattern && err == nil {
			f(re)
		}
	}
}
