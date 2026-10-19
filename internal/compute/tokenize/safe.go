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

// SafeEncode32 Kodla32'nin hata döndüren biçimidir; hata olursa dst değişmeden döner.
func (p *Tokenizer) SafeEncode32(dst []int32, text string, s Options) ([]int32, error) {
	n := len(dst)
	err := catchStepError(func() { dst = p.Encode32(dst, text, s) })
	if err != nil {
		return dst[:n], err
	}
	return dst, nil
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
		if x.withPattern && byteLevelRegex != nil {
			f(byteLevelRegex)
		}
	}
}
