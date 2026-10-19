// Package tokenizer, genel tokenizer.json parçalayıcısını (github.com/sekerahmet/TCB/internal/compute/tokenize)
// TR diline bağlar: parçalayıcı_yükle(yol) ve parçalayıcı nesnesinin .kodla, .çöz,
// .dağarcık_boyutu ve .parça_kimliği üyeleri.
//
// Kodlama/çözme hata döndüren Safe... işlevleriyle yapılır: düzenli ifade adım
// sınırı aşılırsa yarım sonuç yerine yol gösterici bir TR hatası oluşur. Uzun işler
// (dosya yükleme, liste kodlama/çözme) yorumlayıcı kilidi bırakılarak yapılır.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package tokenizer

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/compute/tokenize"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// TypeName parçalayıcı nesnesinin TR'deki tip adıdır (kod listesinde A234).
var TypeName = string(interp.TypeName(codes.A234))

// Object TR'de yüklenmiş bir parçalayıcıdır. Salt okunurdur; dosya tutamağı ya da
// başka bir kaynak tutmaz (dosya yüklemede tümüyle okunup kapatılır).
type Object struct {
	p    *tokenize.Tokenizer
	path string
}

// Tokenizer sarılı Go parçalayıcısını verir.
func (n *Object) Tokenizer() *tokenize.Tokenizer { return n.p }

func (n *Object) Type() interp.ValueType { return interp.ValueType(TypeName) }
func (n *Object) IsTruthy() bool         { return true }

func (n *Object) Inspect() string {
	return fmt.Sprintf("%s(dağarcık %d, %s)", TypeName, n.p.VocabSize(), filepath.Base(n.path))
}

// WatchKind İzleme Masası'nda ikon seçimi için türdür.
func (n *Object) WatchKind() string { return "diger" }

// WatchLabel tip sütununda gösterilen özettir.
func (n *Object) WatchLabel() string {
	return fmt.Sprintf("%s (dağarcık %d)", TypeName, n.p.VocabSize())
}

// TableData dağarcığa eklenmiş parçaları (özel parçalar dahil) tablo olarak verir.
func (n *Object) TableData() ([]string, [][]string, error) {
	extras := n.p.AddedTokens()
	if len(extras) == 0 {
		return nil, nil, fmt.Errorf("bu parçalayıcıda eklenmiş (özel) parça yok; dağarcık boyutu %d", n.p.VocabSize())
	}
	var satirlar [][]string
	for _, e := range extras {
		special := "hayır"
		if e.Special {
			special = "evet"
		}
		satirlar = append(satirlar, []string{fmt.Sprint(e.ID), e.Content, special})
	}
	return []string{"Kimlik", "Parça", "Özel"}, satirlar, nil
}

// Parçalayıcı üyeleri (adları kod listesinde, internal/lang/codes Y61x).
const (
	memberEncode    = codes.Y610
	memberDecode    = codes.Y611
	memberVocabSize = codes.Y612
	memberTokenID   = codes.Y613
)

var members = codes.Names(memberEncode, memberDecode, memberVocabSize, memberTokenID)

func (n *Object) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case memberEncode:
		return n.method(codes.Name(memberEncode), n.encode), nil
	case memberDecode:
		return n.method(codes.Name(memberDecode), n.decode), nil
	case memberVocabSize:
		return interp.NewInt(int64(n.p.VocabSize())), nil
	case memberTokenID:
		return n.method(codes.Name(memberTokenID), n.tokenID), nil
	}
	return nil, bind.MemberError(TypeName, prop, members)
}

func (n *Object) method(name string, fn interp.BuiltinFunc) *interp.BuiltinVal {
	return &interp.BuiltinVal{Name: name, Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Safe(name, func() (interp.Value, error) { return fn(m, args, named) })
	}}
}

// ----------------------------------------------------------------------------
// Kayıt ve yükleme
// ----------------------------------------------------------------------------

// Register parçalayıcı yerleşiklerini VM'ye kaydeder.
func Register(v *interp.VM) {
	v.RegisterBuiltinCode(codes.V200, func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Safe(codes.Name(codes.V200), func() (interp.Value, error) { return load(m, args, named) })
	})
}

// warned aynı dosyanın yükleme uyarılarının süreçte bir kez yazılmasını sağlar.
var warned sync.Map

func load(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.V200)
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(yol) 1 parametre bekler, %d verildi", fn, len(args))
	}
	raw, ok, pathErr := interp.PathArg(fn, args[0])
	if !ok {
		return nil, fmt.Errorf("%s: yol bir metin olmalıdır (ör. \"tokenizer.json\"), %s verildi", fn, args[0].Type())
	}
	if pathErr != nil {
		return nil, pathErr
	}
	path, tried := bind.ResolvePathTried(m, raw)
	var p *tokenize.Tokenizer
	var err error
	var info os.FileInfo
	m.Blocking(func() {
		info, err = os.Stat(path)
		if err == nil && !info.IsDir() {
			p, err = tokenize.Load(path)
		}
	})
	if err != nil {
		return nil, loadError(raw, err, tried)
	}
	if info.IsDir() {
		candidate := filepath.Join(path, "tokenizer.json")
		extra := ""
		if _, e := os.Stat(candidate); e == nil {
			extra = fmt.Sprintf("; bu klasörde tokenizer.json var: %s(%q) deneyin", fn, filepath.ToSlash(filepath.Join(raw, "tokenizer.json")))
		}
		return nil, fmt.Errorf("%s: %q bir klasör; tokenizer.json biçimindeki parçalayıcı dosyasının yolunu verin%s", fn, raw, extra)
	}
	writeWarnings(path, p)
	return &Object{p: p, path: path}, nil
}

func loadError(path string, err error, tried []string) error {
	fn := codes.Name(codes.V200)
	if bind.IsNotExist(err) {
		return turkish.FileError(fn, path, "yüklenemedi", err,
			"tokenizer.json biçimindeki parçalayıcı dosyasının yolunu verin", interp.SearchedPlaces(tried))
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "desteklenmeyen"):
		msg = strings.Replace(msg, "desteklenmeyen", "parçalayıcı türü desteklenmiyor:", 1)
		return fmt.Errorf("%s: %q yüklenemedi, %s", fn, path, msg)
	case strings.Contains(msg, "reddedildi"):
		msg = strings.Replace(msg, " (izin için DuzIfadeAyarlari.RiskliDesenIzni)", "", 1)
		return fmt.Errorf("%s: %q yüklenemedi: %s; bu desen kötü niyetli bir metinde üstel süre harcayabileceği için güvenlik gereği kabul edilmiyor (desenin iç içe sınırsız niceleyici içermeyen bir biçimini kullanın)", fn, path, msg)
	case strings.Contains(msg, "JSON"):
		detail := "dosya JSON olarak okunamadı"
		var se *json.SyntaxError
		var te *json.UnmarshalTypeError
		if errors.As(err, &se) {
			detail = fmt.Sprintf("%d. baytta JSON sözdizimi hatası", se.Offset)
		} else if errors.As(err, &te) {
			detail = fmt.Sprintf("JSON yapısı beklenen biçimde değil (%q alanı)", te.Field)
		}
		return fmt.Errorf("%s: %q geçerli bir tokenizer.json dosyası değil: %s", fn, path, detail)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		// İzin, kilitli dosya, disk hatası...: işletim sisteminin metni yerine Türkçe neden
		return turkish.FileError(fn, path, "yüklenemedi", err)
	}
	return fmt.Errorf("%s: %q yüklenemedi: %s", fn, path, msg)
}

func writeWarnings(path string, p *tokenize.Tokenizer) {
	u := p.Warnings()
	if len(u) == 0 {
		return
	}
	key := path
	if a, err := filepath.Abs(path); err == nil {
		key = a
	}
	if _, written := warned.LoadOrStore(key, true); written {
		return
	}
	fmt.Fprintf(os.Stderr, "Uyarı: parçalayıcı %q düzenli ifadelerinde riskli desen var (bazı metinlerde yavaşlayabilir; adım sınırı aşılırsa kodlama hata verir):\n", filepath.Base(path))
	for _, x := range u {
		fmt.Fprintf(os.Stderr, "  - %s\n", x)
	}
}

// ----------------------------------------------------------------------------
// Kodlama
// ----------------------------------------------------------------------------

var (
	nameSpecial = bind.NamedArg(codes.V910)
	nameSecond  = bind.NamedArg(codes.V911)
)

// longText bu bayttan uzun tek metinler yorumlayıcı kilidi bırakılarak kodlanır.
const longText = 1 << 16

func stepError(fn string, err error) error {
	var ah *tokenize.StepLimitError
	if errors.As(err, &ah) && ah.Memory {
		return fmt.Errorf("%s: parçalayıcının düzenli ifadesi %q bu metinde geri izleme belleği sınırını aştı (aramanın başladığı bayt: %d); desen çok uzun bir yinelemeyi geri izlenebilir tutuyor ve işlem güvenlik için durduruldu. Metni daha küçük parçalara bölmeyi ya da olağan dışı uzun tekrarları temizlemeyi deneyin",
			fn, ah.Pattern, ah.Pos)
	}
	if ah != nil {
		return fmt.Errorf("%s: parçalayıcının düzenli ifadesi %q bu metinde adım sınırını (%d adım) aştı (aramanın başladığı bayt: %d); desen bu girdide aşırı geri izleme yapıyor ve işlem güvenlik için durduruldu. Metni daha küçük parçalara bölmeyi ya da olağan dışı uzun tekrarları (ör. binlerce aynı karakter) temizlemeyi deneyin",
			fn, ah.Pattern, ah.Budget, ah.Pos)
	}
	return fmt.Errorf("%s: %v", fn, err)
}

func (n *Object) encode(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(memberEncode)
	if err := bind.CheckNamed(fn+"()", named, nameSpecial, nameSecond); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(metin) 1 parametre bekler (metin ya da metin listesi), %d verildi", fn, len(args))
	}
	var pick tokenize.Options
	if v, ok := bind.GetNamed(named, nameSpecial); ok {
		b, err := bind.Bool(fn, codes.Name(codes.V910), v)
		if err != nil {
			return nil, err
		}
		pick.DontAddSpecial = !b
	}
	secondV, hasSecond := bind.GetNamed(named, nameSecond)

	switch x := args[0].(type) {
	case *interp.StringVal:
		if hasSecond {
			ik, ok := secondV.(*interp.StringVal)
			if !ok {
				return nil, fmt.Errorf("%s: tek metinle %s: de bir metin olmalıdır, %s verildi", fn, nameSecond.TR, secondV.Type())
			}
			var k *tokenize.Encoding
			var err error
			n.maybeRelease(m, len(x.Val)+len(ik.Val), func() { k, err = n.p.SafeEncodePair(x.Val, &ik.Val, pick) })
			if err != nil {
				return nil, stepError(fn, err)
			}
			return idList32(k.IDs), nil
		}
		var ids []int
		var err error
		n.maybeRelease(m, len(x.Val), func() { ids, err = n.p.SafeEncode(x.Val, pick) })
		if err != nil {
			return nil, stepError(fn, err)
		}
		return idList(ids), nil
	case *interp.ListVal:
		texts, err := stringList(fn, "metin listesi", x)
		if err != nil {
			return nil, err
		}
		var seconds []string
		if hasSecond {
			il, ok := secondV.(*interp.ListVal)
			if !ok {
				return nil, fmt.Errorf("%s: metin listesiyle %s: de aynı uzunlukta bir metin listesi olmalıdır, %s verildi", fn, nameSecond.TR, secondV.Type())
			}
			if seconds, err = stringList(fn, nameSecond.TR, il); err != nil {
				return nil, err
			}
			if len(seconds) != len(texts) {
				return nil, fmt.Errorf("%s: %s: listesi %d metin içeriyor, birinci liste %d; çiftler eşit uzunlukta olmalı", fn, nameSecond.TR, len(seconds), len(texts))
			}
		}
		var result [][]int
		var errs []error
		m.Blocking(func() {
			if seconds == nil {
				result, errs = n.p.SafeEncodeBatch(texts, pick, 0)
				return
			}
			result = make([][]int, len(texts))
			errs = make([]error, len(texts))
			parallel(len(texts), func(i int) {
				k, err := n.p.SafeEncodePair(texts[i], &seconds[i], pick)
				errs[i] = err
				if err == nil {
					result[i] = int32ToInt(k.IDs)
				}
			})
		})
		for i, h := range errs {
			if h != nil {
				return nil, stepError(fmt.Sprintf("kodla (listenin %d. metni)", i+1), h)
			}
		}
		elems := make([]interp.Value, len(result))
		for i, ids := range result {
			elems[i] = idList(ids)
		}
		return &interp.ListVal{Elements: elems}, nil
	}
	return nil, fmt.Errorf("kodla: bir metin ya da metin listesi bekler, %s verildi", args[0].Type())
}

// maybeRelease uzun metinlerde işi yorumlayıcı kilidini bırakarak yapar.
func (n *Object) maybeRelease(m *interp.VM, size int, f func()) {
	if size >= longText {
		m.Blocking(f)
		return
	}
	f()
}

func stringList(fn, what string, l *interp.ListVal) ([]string, error) {
	out := make([]string, len(l.Elements))
	for i, e := range l.Elements {
		s, ok := e.(*interp.StringVal)
		if !ok {
			return nil, fmt.Errorf("%s: %s yalnızca metin içermeli; %d. eleman %s", fn, what, i+1, e.Type())
		}
		out[i] = s.Val
	}
	return out, nil
}

// parallel f'yi 0..n-1 için işlemci sayısı kadar gorutinde çalıştırır. Bir gorutindeki
// panik süreci çökertmez: iş bitince çağıranın gorutininde yeniden fırlatılır (ve
// oradaki recover ile TR hatasına çevrilir).
func parallel(n int, f func(i int)) {
	worker := min(runtime.GOMAXPROCS(0), n)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var recovered any
	order := 0
	defer func() {
		if recovered != nil {
			panic(recovered)
		}
	}()
	for w := 0; w < worker; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					if recovered == nil {
						recovered = r
					}
					order = n
					mu.Unlock()
				}
			}()
			for {
				mu.Lock()
				i := order
				order++
				mu.Unlock()
				if i >= n {
					return
				}
				f(i)
			}
		}()
	}
	wg.Wait()
}

func int32ToInt(ids []int32) []int {
	out := make([]int, len(ids))
	for i, x := range ids {
		out[i] = int(x)
	}
	return out
}

func idList(ids []int) *interp.ListVal {
	elems := make([]interp.Value, len(ids))
	for i, id := range ids {
		elems[i] = interp.NewInt(int64(id))
	}
	return &interp.ListVal{Elements: elems}
}

func idList32(ids []int32) *interp.ListVal {
	elems := make([]interp.Value, len(ids))
	for i, id := range ids {
		elems[i] = interp.NewInt(int64(id))
	}
	return &interp.ListVal{Elements: elems}
}

// ----------------------------------------------------------------------------
// Çözme
// ----------------------------------------------------------------------------

func (n *Object) decode(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(memberDecode)
	if err := bind.CheckNamed(fn+"()", named, nameSpecial); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(kimlikler) 1 parametre bekler (kimlik listesi, listelerin listesi ya da tensör), %d verildi", fn, len(args))
	}
	// Varsayılan, başvuru kütüphanesininki gibi özel parçaları ATLAMAKTIR;
	// özel_parçalar: doğru onları da çıktıya yazar.
	skip := true
	if v, ok := bind.GetNamed(named, nameSpecial); ok {
		b, err := bind.Bool(fn, codes.Name(codes.V910), v)
		if err != nil {
			return nil, err
		}
		skip = !b
	}
	seqs, single, err := idSeqs(args[0])
	if err != nil {
		return nil, err
	}
	result := make([]string, len(seqs))
	errs := make([]error, len(seqs))
	job := func() {
		if len(seqs) == 1 {
			result[0], errs[0] = n.p.SafeDecode(seqs[0], skip)
			return
		}
		parallel(len(seqs), func(i int) { result[i], errs[i] = n.p.SafeDecode(seqs[i], skip) })
	}
	total := 0
	for _, d := range seqs {
		total += len(d)
	}
	n.maybeRelease(m, total*4, job)
	for i, h := range errs {
		if h != nil {
			if single {
				return nil, stepError(fn, h)
			}
			return nil, stepError(fmt.Sprintf("çöz (%d. dizi)", i+1), h)
		}
	}
	if single {
		return &interp.StringVal{Val: result[0]}, nil
	}
	elems := make([]interp.Value, len(result))
	for i, s := range result {
		elems[i] = &interp.StringVal{Val: s}
	}
	return &interp.ListVal{Elements: elems}, nil
}

// idSeqs çöz'ün argümanını kimlik dizilerine çevirir. tek doğruysa argüman tek
// bir dizidir (sonuç metin), değilse dizilerin listesidir (sonuç metin listesi).
func idSeqs(v interp.Value) (seqs [][]int, single bool, err error) {
	fn := codes.Name(memberDecode)
	switch x := v.(type) {
	case *interp.ListVal:
		if len(x.Elements) == 0 {
			return [][]int{nil}, true, nil
		}
		if _, inner := x.Elements[0].(*interp.ListVal); inner {
			seqs = make([][]int, len(x.Elements))
			for i, e := range x.Elements {
				l, ok := e.(*interp.ListVal)
				if !ok {
					return nil, false, fmt.Errorf("%s: listelerin listesinde %d. eleman liste değil (%s)", fn, i+1, e.Type())
				}
				if seqs[i], err = intList(fmt.Sprintf("%d. listenin", i+1), l.Elements); err != nil {
					return nil, false, err
				}
			}
			return seqs, false, nil
		}
		d, err := intList("listenin", x.Elements)
		if err != nil {
			return nil, false, err
		}
		return [][]int{d}, true, nil
	case *ml.TensorValue:
		t := x.Tensor()
		sh := t.Shape()
		data := t.Data()
		if len(sh) > 2 {
			return nil, false, fmt.Errorf("%s: tensör 1 boyutlu (tek dizi) ya da 2 boyutlu (dizi listesi) olmalıdır, %d boyutlu verildi", fn, len(sh))
		}
		ids := make([]int, len(data))
		for i, f := range data {
			if float32(int(f)) != f {
				return nil, false, fmt.Errorf("%s: tensördeki kimlikler tam sayı olmalıdır, %v bulundu", fn, f)
			}
			ids[i] = int(f)
		}
		if len(sh) < 2 {
			return [][]int{ids}, true, nil
		}
		seqs = make([][]int, sh[0])
		for i := range seqs {
			seqs[i] = ids[i*sh[1] : (i+1)*sh[1]]
		}
		return seqs, false, nil
	}
	return nil, false, fmt.Errorf("%s: kimlik listesi, listelerin listesi ya da tensör bekler, %s verildi", fn, v.Type())
}

func intList(what string, elems []interp.Value) ([]int, error) {
	out := make([]int, len(elems))
	for i, e := range elems {
		n, ok := bind.Int(e)
		if !ok {
			return nil, fmt.Errorf("çöz: kimlikler tam sayı olmalıdır; %s %d. elemanı %s", what, i+1, e.Inspect())
		}
		out[i] = int(n)
	}
	return out, nil
}

// ----------------------------------------------------------------------------
// Sözlük sorguları
// ----------------------------------------------------------------------------

func (n *Object) tokenID(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if err := bind.CheckNamed(codes.Name(codes.Y613)+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parça) 1 parametre bekler, %d verildi", codes.Name(codes.Y613), len(args))
	}
	s, ok := args[0].(*interp.StringVal)
	if !ok {
		return nil, fmt.Errorf("%s: parça bir metin olmalıdır (ör. \"<|endoftext|>\"), %s verildi", codes.Name(codes.Y613), args[0].Type())
	}
	id, ok := n.p.TokenToID(s.Val)
	if !ok {
		return interp.NULL, nil
	}
	return interp.NewInt(int64(id)), nil
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*Object)(nil)
)
