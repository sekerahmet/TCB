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

// Inspect is the value's display text, in the person's writing language (TR_DIL).
func (n *Object) Inspect() string { return n.Text(codes.Display()) }

var _ interp.LanguageText = (*Object)(nil)

// Text is the value's text in lang: str() and yazdır give it in the language of the file they
// are written in (docs/spec/03_tipler.md).
func (n *Object) Text(lang *codes.Language) string {
	return messages.TextIn(lang.Code, messages.H5600, lang.Name(codes.A234), n.p.VocabSize(), filepath.Base(n.path))
}

// WatchKind İzleme Masası'nda ikon seçimi için türdür.
func (n *Object) WatchKind() string { return "diger" }

// WatchLabel tip sütununda gösterilen özettir.
func (n *Object) WatchLabel() string {
	return messages.Text(messages.H5601, codes.NameRef(codes.A234), n.p.VocabSize())
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
		return bind.Method(codes.Name(memberEncode), n.encode), nil
	case memberDecode:
		return bind.Method(codes.Name(memberDecode), n.decode), nil
	case memberVocabSize:
		return interp.NewInt(int64(n.p.VocabSize())), nil
	case memberTokenID:
		return bind.Method(codes.Name(memberTokenID), n.tokenID), nil
	}
	return nil, bind.MemberError(TypeName, prop, members)
}

// ----------------------------------------------------------------------------
// Kayıt ve yükleme
// ----------------------------------------------------------------------------

// Register parçalayıcı yerleşiklerini VM'ye kaydeder.
func Register(v *interp.VM) {
	v.RegisterBuiltinCode(codes.V200, bind.SafeBuiltin(codes.Name(codes.V200), load))
}

// warned aynı dosyanın yükleme uyarılarının süreçte bir kez yazılmasını sağlar.
var warned sync.Map

func load(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.V200)
	if err := bind.CheckNamed(fn+"()", named); err != nil {
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
		if _, e := os.Stat(candidate); e == nil {
			return nil, messages.New(messages.H5611, fn, raw, filepath.ToSlash(filepath.Join(raw, "tokenizer.json")))
		}
		return nil, messages.New(messages.H5610, fn, raw)
	}
	writeWarnings(path, p)
	return &Object{p: p, path: path}, nil
}

// isUnsupported err'in, parçalayıcı dosyasındaki desteklenmeyen bir özelliği bildirip bildirmediğini
// söyler.
func isUnsupported(err error) bool {
	return messages.HasAny(err, messages.H5202, messages.H5257, messages.H5275, messages.H5276)
}

func loadError(path string, err error, tried []string) error {
	fn := codes.DisplayName(codes.V200)
	if bind.IsNotExist(err) {
		return turkish.FileError(fn, path, messages.H2107, err,
			messages.Text(messages.H5612), interp.SearchedPlaces(tried))
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
	nameSpecial = bind.NamedArg(codes.V910)
	nameSecond  = bind.NamedArg(codes.V911)
)

// longText bu bayttan uzun tek metinler yorumlayıcı kilidi bırakılarak kodlanır.
const longText = 1 << 16

func stepError(fn string, err error) error {
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

func (n *Object) encode(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(memberEncode)
	if err := bind.CheckNamed(fn+"()", named, nameSpecial, nameSecond); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, messages.New(messages.H5624, fn, len(args))
	}
	var pick tokenize.Options
	special, err := bind.OptionalBool(fn, named, nameSpecial, true)
	if err != nil {
		return nil, err
	}
	pick.DontAddSpecial = !special
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
		texts, err := stringList(fn, messages.Text(messages.H5626), x)
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
				return nil, stepError(messages.Text(messages.H5629, fn, i+1), h)
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

func stringList(fn string, what any, l *interp.ListVal) ([]string, error) {
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
// Çözme
// ----------------------------------------------------------------------------

func (n *Object) decode(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(memberDecode)
	if err := bind.CheckNamed(fn+"()", named, nameSpecial); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, messages.New(messages.H5632, fn, len(args))
	}
	// Varsayılan, başvuru kütüphanesininki gibi özel parçaları ATLAMAKTIR;
	// özel_parçalar: doğru onları da çıktıya yazar.
	special, err := bind.OptionalBool(fn, named, nameSpecial, false)
	if err != nil {
		return nil, err
	}
	skip := !special
	seqs, single, err := idSeqs(args[0])
	if err != nil {
		return nil, err
	}
	var result []string
	var errs []error
	job := func() { result, errs = n.p.SafeDecodeBatch(seqs, skip, 0) }
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
			return nil, stepError(messages.Text(messages.H5633, fn, i+1), h)
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
	fn := codes.DisplayName(memberDecode)
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
					return nil, false, messages.New(messages.H5634, fn, i+1, e.Type())
				}
				if seqs[i], err = intList(i+1, l.Elements); err != nil {
					return nil, false, err
				}
			}
			return seqs, false, nil
		}
		d, err := intList(0, x.Elements)
		if err != nil {
			return nil, false, err
		}
		return [][]int{d}, true, nil
	case *ml.TensorValue:
		t := x.Tensor()
		sh := t.Shape()
		if len(sh) > 2 {
			return nil, false, messages.New(messages.H5636, fn, len(sh))
		}
		ids, bad, ok := x.IntElements()
		if !ok {
			return nil, false, messages.New(messages.H5637, fn, bad)
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
	return nil, false, messages.New(messages.H5638, fn, v.Type())
}

// intList kimlik listesini tam sayılara çevirir; row iç içe listenin 1'den başlayan satırıdır, düz
// listede 0 (iletide "listenin" ya da "3. listenin" diye yazılır, H5639).
func intList(row int, elems []interp.Value) ([]int, error) {
	out := make([]int, len(elems))
	for i, e := range elems {
		n, ok := bind.Int(e)
		if !ok {
			return nil, messages.New(messages.H5639, codes.NameRef(memberDecode), row, i+1, interp.Shown(e))
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
		return nil, messages.New(messages.H5640, codes.NameRef(codes.Y613), len(args))
	}
	s, ok := args[0].(*interp.StringVal)
	if !ok {
		return nil, messages.New(messages.H5641, codes.NameRef(codes.Y613), args[0].Type())
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
