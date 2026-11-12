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
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/compute/tokenize"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// TypeName parçalayıcı nesnesinin TR'deki tip adıdır (kod listesinde TR_type_Tokenizer).
var TypeName = string(interp.TypeName(codes.TypeTokenizer))

// Object TR'de yüklenmiş bir parçalayıcıdır. Salt okunurdur; dosya tutamağı ya da
// başka bir kaynak tutmaz (dosya yüklemede tümüyle okunup kapatılır).
type Object struct {
	p    *tokenize.Tokenizer
	path string
	// trFalse ailesiz parçalayıcının TR_special_tokens: TR_false'taki hatasıdır (16.2; H7178 ya da H7184); nil ise
	// TR_false bugünkü gibi bütün eklenmiş parçaları kapatır. Yüklemede bir kez kurulur.
	trFalse error
}

// Tokenizer sarılı Go parçalayıcısını verir.
func (n *Object) Tokenizer() *tokenize.Tokenizer { return n.p }

func (n *Object) Type() interp.ValueType { return interp.ValueType(TypeName) }
func (n *Object) IsTruthy() bool         { return true }

// Inspect is the value's text (3.14): language-independent, the same as its program text.
func (n *Object) Inspect() string { return n.Text(codes.ValueNames()) }

var _ interp.LanguageText = (*Object)(nil)

// Text is the value's text in lang: str() and yazdır give it in the language of the file they
// are written in (docs/spec/03_tipler.md).
func (n *Object) Text(lang *codes.Language) string {
	return messages.TextIn(lang.Code, messages.H5600, lang.Name(codes.TypeTokenizer), n.p.VocabSize(), filepath.Base(n.path))
}

// WatchKind İzleme Masası'nda ikon seçimi için türdür.
func (n *Object) WatchKind() string { return "other" }

// WatchLabel tip sütununda gösterilen özettir.
func (n *Object) WatchLabel() string {
	return messages.Text(messages.H5601, codes.NameRef(codes.TypeTokenizer), n.p.VocabSize())
}

// TableData dağarcığa eklenmiş parçaları (özel parçalar dahil) tablo olarak verir.
func (n *Object) TableData() ([]string, [][]string, error) {
	extras := n.p.AddedTokens()
	if len(extras) == 0 {
		return nil, nil, messages.New(messages.H5602, n.p.VocabSize())
	}
	var satirlar [][]string
	for _, e := range extras {
		special := messages.Text(messages.H5603)
		if e.Special {
			special = messages.Text(messages.H5604)
		}
		satirlar = append(satirlar, []string{fmt.Sprint(e.ID), e.Content, special})
	}
	return []string{messages.Text(messages.H5605), messages.Text(messages.H5606), messages.Text(messages.H5607)}, satirlar, nil
}

// Parçalayıcı üyeleri (adları kod listesinde, internal/lang/codes mem).
const (
	memberEncode      = codes.MemEncode
	memberDecode      = codes.MemDecode
	memberBatchDecode = codes.MemBatchDecode
	memberVocabSize   = codes.MemVocabSize
	memberTokenID     = codes.MemTokenId
	memberPreTag      = codes.MemPretokenizerTag
	memberDeclaredEOS = codes.MemDeclaredEosTokens
)

var members = []codes.Code{memberEncode, memberDecode, memberBatchDecode, memberVocabSize, memberTokenID, memberPreTag, memberDeclaredEOS}

func (n *Object) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case memberEncode:
		return bind.CodeMethod(memberEncode, false, n.encode), nil
	case memberDecode:
		return bind.CodeMethod(memberDecode, false, n.decode), nil
	case memberBatchDecode:
		return bind.CodeMethod(memberBatchDecode, false, n.batchDecode), nil
	case memberVocabSize:
		return interp.NewInt(int64(n.p.VocabSize())), nil
	case memberTokenID:
		return bind.CodeMethod(memberTokenID, false, n.tokenID), nil
	case memberPreTag: // 16.1: yalnız GGUF'ta var
		if tag, ok := n.p.PretokenizerTag(); ok {
			return &interp.StringVal{Val: tag}, nil
		}
		return interp.Null, nil
	case memberDeclaredEOS:
		ids, ok, err := n.p.DeclaredEOSTokens()
		if err != nil {
			return nil, err
		}
		if ok {
			return bind.IntList(ids), nil
		}
		return interp.Null, nil
	}
	return nil, bind.MemberError(interp.ValueType(TypeName), prop, members)
}

// ----------------------------------------------------------------------------
// Kayıt ve yükleme
// ----------------------------------------------------------------------------

// Register parçalayıcı yerleşiklerini VM'ye kaydeder.
func Register(v *interp.VM) {
	v.RegisterBuiltinCode(codes.FnLoadTokenizer, bind.SafeBuiltin(interp.CallPrefix(codes.FnLoadTokenizer), load))
}

// warned aynı dosyanın yükleme uyarılarının süreçte bir kez yazılmasını sağlar.
var warned sync.Map

func load(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnLoadTokenizer)
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, messages.New(messages.H5608, fn, len(args))
	}
	raw, ok, pathErr := interp.PathArg(fn, args[0])
	if !ok {
		return nil, messages.New(messages.H5609, fn, args[0].Type())
	}
	if pathErr != nil {
		return nil, pathErr
	}
	path, tried := bind.ResolvePathTried(m, raw)
	var p *tokenize.Tokenizer
	var err error
	var info os.FileInfo
	file, shown := path, raw
	// Dosyadan yükleme programın durumunu değiştirmez: durdurma onu bırakır (1.5).
	if waitErr := m.Abandonable(func() {
		if info, err = os.Stat(path); err != nil {
			return
		}
		if info.IsDir() {
			// Model klasörü (16.1): klasörün tokenizer.json'u; aile config.json ve tokenizer_config.json'dan.
			file, shown = filepath.Join(path, "tokenizer.json"), filepath.ToSlash(filepath.Join(raw, "tokenizer.json"))
			if _, e := os.Stat(file); e != nil {
				return
			}
		}
		if p, err = tokenize.Load(file); err == nil && info.IsDir() {
			err = p.ApplyFolderFamily(folderKeys(path))
		}
	}); waitErr != nil {
		return nil, waitErr
	}
	if err != nil {
		return nil, loadError(shown, err, tried)
	}
	if p == nil {
		return nil, messages.New(messages.H5610, fn, raw)
	}
	writeWarnings(file, p)
	// 16.2'nin ailesiz yolu: klasörle açılanda klasör zaten denendi; tek başına açılan dosyanın klasörü önerilebilir.
	folder, arch, marker := raw, "", ""
	if !info.IsDir() {
		folder = filepath.ToSlash(filepath.Dir(raw))
		arch, marker = folderKeys(filepath.Dir(file))
	}
	return &Object{p: p, path: file, trFalse: p.FamilylessError(arch, marker, folder)}, nil
}

// genericTokenizerClasses tokenizer_class'ı aileyi söylemeyen sınıflardır: işaret bos_token'ın adıdır
// (lib/model_family.tr'nin _GENEL_SINIFLAR'ı ve _klasör_işareti'yle aynı kural).
var genericTokenizerClasses = map[string]bool{"PreTrainedTokenizer": true, "PreTrainedTokenizerFast": true}

// folderKeys model klasörünün aile anahtarlarıdır (15.55'in seçimi): config.json'un model_type'ı ve
// tokenizer_config.json'un işareti (tokenizer_class, sondaki Fast atılarak; genel sınıfta bos_token'ın
// adı). Dosya yoksa ya da okunamıyorsa anahtar boştur: parçalayıcı ailesiz açılır.
func folderKeys(dir string) (arch, marker string) {
	var config struct {
		ModelType string `json:"model_type"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
		_ = json.Unmarshal(b, &config)
	}
	var tc struct {
		Class string          `json:"tokenizer_class"`
		BOS   json.RawMessage `json:"bos_token"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "tokenizer_config.json")); err == nil {
		_ = json.Unmarshal(b, &tc)
	}
	if !genericTokenizerClasses[tc.Class] {
		return config.ModelType, strings.TrimSuffix(tc.Class, "Fast")
	}
	var name string
	var obj struct {
		Content string `json:"content"`
	}
	if json.Unmarshal(tc.BOS, &name) != nil && json.Unmarshal(tc.BOS, &obj) == nil {
		name = obj.Content
	}
	return config.ModelType, name
}

// isUnsupported err'in, parçalayıcı dosyasındaki desteklenmeyen bir özelliği bildirip bildirmediğini
// söyler.
func isUnsupported(err error) bool {
	return messages.HasAny(err, messages.H5202, messages.H5257, messages.H5275, messages.H5276)
}

func loadError(path string, err error, tried []string) error {
	fn := codes.NameRef(codes.FnLoadTokenizer)
	if bind.IsNotExist(err) {
		return turkish.FileError(fn, path, messages.H2107, err,
			messages.New(messages.H5612), interp.SearchedPlaces(tried))
	}
	switch {
	case isUnsupported(err):
		// Çekirdeğin iletisi kendi başına tamdır ("desteklenmeyen ... türü"); metni değiştirilmez.
		return messages.New(messages.H5614, fn, path, err)
	case messages.Has(err, messages.H5248):
		return messages.New(messages.H5615, fn, path, err)
	case messages.Has(err, messages.H5274):
		var se *json.SyntaxError
		var te *json.UnmarshalTypeError
		switch {
		case errors.As(err, &se):
			return messages.New(messages.H5617, fn, path, se.Offset)
		case errors.As(err, &te):
			return messages.New(messages.H5618, fn, path, te.Field)
		}
		return messages.New(messages.H5619, fn, path)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		// İzin, kilitli dosya, disk hatası...: işletim sisteminin metni yerine Türkçe neden
		return turkish.FileError(fn, path, messages.H2107, pe)
	}
	var oe *tokenize.FamilyOverlapError
	if errors.As(err, &oe) {
		// Ailenin biçim kümesi protokol etiketiyle kesişiyor: Go tablosunun hatası (16.2).
		return oe.Message(fn)
	}
	return messages.New(messages.H5620, fn, path, err)
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
	fmt.Fprintln(os.Stderr, messages.Text(messages.H5621, filepath.Base(path)))
	for _, x := range u {
		fmt.Fprintf(os.Stderr, "  - %s\n", x)
	}
}

// ----------------------------------------------------------------------------
// Kodlama
// ----------------------------------------------------------------------------

var (
	nameSpecial = bind.NamedArg(codes.ArgSpecialTokens)
	nameSecond  = bind.NamedArg(codes.ArgPair)
)

// longText bu bayttan uzun tek metinler yorumlayıcı kilidi bırakılarak kodlanır.
const longText = 1 << 16

func stepError(fn any, err error) error {
	var ah *tokenize.StepLimitError
	if errors.As(err, &ah) && ah.Memory {
		return messages.New(messages.H5622,
			fn, ah.Pattern, ah.Pos)
	}
	if ah != nil {
		return messages.New(messages.H5623,
			fn, ah.Pattern, ah.Budget, ah.Pos)
	}
	return messages.New(messages.H2000, fn, err)
}

// encodeOptions kodlamanın seçenekleridir (TR_special_tokens:). 16.2: TR_false'ta metindeki eklenmiş parça yazımı da
// kimlik olmaz, özel olsun olmasın; ailesiz parçalayıcıda boşluk parçaları baytlara bölünmez, kodlama hatadır (trFalse).
func (n *Object) encodeOptions(fn any, named map[string]interp.Value) (tokenize.Options, error) {
	special, err := bind.OptionalBool(fn, named, nameSpecial, true)
	if err != nil {
		return tokenize.Options{}, err
	}
	if !special && n.trFalse != nil {
		return tokenize.Options{}, n.trFalse
	}
	return tokenize.Options{DontAddSpecial: !special, DontParseAdded: !special}, nil
}

func (n *Object) encode(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(memberEncode)
	if err := bind.CheckNamed(fn.Called(), named, nameSpecial, nameSecond); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, messages.New(messages.H5624, fn, len(args))
	}
	pick, err := n.encodeOptions(fn, named)
	if err != nil {
		return nil, err
	}
	secondV, hasSecond := bind.GetNamed(named, nameSecond)

	switch x := args[0].(type) {
	case *interp.StringVal:
		if hasSecond {
			ik, ok := secondV.(*interp.StringVal)
			if !ok {
				return nil, messages.New(messages.H5625, fn, nameSecond, secondV.Type())
			}
			var k *tokenize.Encoding
			var err error
			n.maybeRelease(m, len(x.Val)+len(ik.Val), func() { k, err = n.p.SafeEncodePair(x.Val, &ik.Val, pick) })
			if err != nil {
				return nil, stepError(fn, err)
			}
			return bind.IntList(k.IDs), nil
		}
		var ids []int
		var err error
		n.maybeRelease(m, len(x.Val), func() { ids, err = n.p.SafeEncode(x.Val, pick) })
		if err != nil {
			return nil, stepError(fn, err)
		}
		return bind.IntList(ids), nil
	case *interp.ListVal:
		texts, err := stringList(fn, messages.New(messages.H5626), x)
		if err != nil {
			return nil, err
		}
		var seconds []string
		if hasSecond {
			il, ok := secondV.(*interp.ListVal)
			if !ok {
				return nil, messages.New(messages.H5627, fn, nameSecond, secondV.Type())
			}
			if seconds, err = stringList(fn, nameSecond, il); err != nil {
				return nil, err
			}
			if len(seconds) != len(texts) {
				return nil, messages.New(messages.H5628, fn, nameSecond, len(seconds), len(texts))
			}
		}
		var result [][]int
		var errs []error
		m.Blocking(func() {
			if seconds == nil {
				result, errs = n.p.SafeEncodeBatch(texts, pick, 0)
				return
			}
			result, errs = n.p.SafeEncodePairBatch(texts, seconds, pick, 0)
		})
		for i, h := range errs {
			if h != nil {
				return nil, stepError(messages.New(messages.H5629, fn, i+1), h)
			}
		}
		elems := make([]interp.Value, len(result))
		for i, ids := range result {
			elems[i] = bind.IntList(ids)
		}
		return &interp.ListVal{Elements: elems}, nil
	}
	return nil, messages.New(messages.H5630, fn, args[0].Type())
}

// maybeRelease uzun metinlerde işi yorumlayıcı kilidini bırakarak yapar.
func (n *Object) maybeRelease(m *interp.VM, size int, f func()) {
	if size >= longText {
		m.Blocking(f)
		return
	}
	f()
}

func stringList(fn any, what any, l *interp.ListVal) ([]string, error) {
	out := make([]string, len(l.Elements))
	for i, e := range l.Elements {
		s, ok := e.(*interp.StringVal)
		if !ok {
			return nil, messages.New(messages.H5631, fn, what, i+1, e.Type())
		}
		out[i] = s.Val
	}
	return out, nil
}

// ----------------------------------------------------------------------------
// Sözlük sorguları
// ----------------------------------------------------------------------------

func (n *Object) tokenID(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if err := bind.CheckNamed(codes.NameRef(codes.MemTokenId).Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, messages.New(messages.H5640, codes.NameRef(codes.MemTokenId), len(args))
	}
	s, ok := args[0].(*interp.StringVal)
	if !ok {
		return nil, messages.New(messages.H5641, codes.NameRef(codes.MemTokenId), args[0].Type())
	}
	id, ok := n.p.TokenToID(s.Val)
	if !ok {
		return interp.Null, nil
	}
	return interp.NewInt(int64(id)), nil
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*Object)(nil)
)
