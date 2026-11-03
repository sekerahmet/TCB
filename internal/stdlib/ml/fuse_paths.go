package ml

import (
	"fmt"
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Kaynaştırmanın kapanış yolları (dil tanımı 15.44; kural fuse_captures.go'da): bir yolun
// okuyucusu kapanış ortamında, programın okumasıyla aynı yoldan değerlendirilir.

type indexVar struct {
	text string
	read reader
}

// pathReader e'yi kapanış ortamında değerlendiren bir okuyucu kurar: kök kapanışta bulunan bir ad
// (fonksiyonun kendi adı değil), sonra nesnenin alanı ya da modülün adı, tam sayı ya da metin değişmez indeks,
// ya da kapanıştan bir değişken indeks. Kapanışta olmayan ad (dilin sözcüğü, tip) yol değildir.
func (s scope) pathReader(e ast.Expression) (reader, string, []indexVar, bool) {
	switch x := e.(type) {
	case *ast.Identifier:
		return s.nameReader(x.Value)
	case *ast.MemberExpression:
		obj, text, ivs, ok := s.pathReader(x.Object)
		if !ok || x.Safe {
			return nil, "", nil, false
		}
		field := x.Property
		read := func(m *interp.VM) (interp.Value, bool) {
			o, ok := obj(m)
			if !ok {
				return nil, false
			}
			switch c := o.(type) {
			case *interp.StructInstanceVal:
				if v, ok := c.Fields[field]; ok {
					return v, true
				}
				return closureMember(m, o, field) // metot değeri
			case *interp.DictVal:
				return closureMember(m, o, field) // modülün adı (sıradan sözlükte nokta hatadır)
			}
			// Çekirdek değerin (liste, metin) yerleşik özelliği yol değildir: izlenemez (H6987).
			return nil, false
		}
		return read, text + "." + field, ivs, true
	case *ast.IndexExpression:
		return s.indexReader(x)
	}
	return nil, "", nil, false
}

// nameReader kökün okuyucusudur. Kapanıştaki nesnenin metodunda TR_self nesnenin yoludur.
func (s scope) nameReader(name string) (reader, string, []indexVar, bool) {
	if s.local[name] {
		return nil, "", nil, false
	}
	isSelf := name == codes.Name(codes.KwSelf)
	if isSelf && s.self != nil {
		return s.self, s.selfText, nil, true
	}
	fv := s.fv
	if _, found := fv.Env.Get(name); !found && !(isSelf && fv.BoundSelf != nil) {
		return nil, "", nil, false
	}
	read := func(*interp.VM) (interp.Value, bool) {
		if isSelf && fv.BoundSelf != nil {
			return fv.BoundSelf, true
		}
		return fv.Env.Get(name)
	}
	return read, name, nil, true
}

// indexReader 'x[i]' yolunun okuyucusudur; değişken indeksin değeri de yolun parçasıdır.
func (s scope) indexReader(x *ast.IndexExpression) (reader, string, []indexVar, bool) {
	obj, text, ivs, ok := s.pathReader(x.Left)
	if !ok {
		return nil, "", nil, false
	}
	var key reader
	var keyText string
	switch k := x.Index.(type) {
	case *ast.IntegerLiteral:
		v := &interp.IntVal{Val: k.Value}
		key, keyText = func(*interp.VM) (interp.Value, bool) { return v, true }, strconv.FormatInt(k.Value, 10)
	case *ast.StringLiteral:
		v := &interp.StringVal{Val: k.Value}
		key, keyText = func(*interp.VM) (interp.Value, bool) { return v, true }, strconv.Quote(k.Value)
	default:
		kr, kt, kivs, ok := s.pathReader(x.Index)
		if !ok {
			return nil, "", nil, false
		}
		ivs = append(append(ivs, kivs...), indexVar{kt, kr})
		key, keyText = kr, kt
	}
	read := func(m *interp.VM) (interp.Value, bool) {
		o, ok1 := obj(m)
		k, ok2 := key(m)
		if !ok1 || !ok2 {
			return nil, false
		}
		return indexValue(o, k)
	}
	return read, fmt.Sprintf("%s[%s]", text, keyText), ivs, true
}

// closureMember o.name'in bugünkü değeridir, programdaki 'o.ad' okumasıyla aynı yoldan: modülün
// adı (modülün kendi fonksiyonu en dış değişkenini değiştirmiş olabilir, 11.3; sözlüğün yükleme
// kopyası eski kalır) ya da nesnenin metodu (fonksiyon değeri). Okunamayan üye (sıradan sözlüğün
// noktalı okuması) yol değildir.
func closureMember(m *interp.VM, o interp.Value, name string) (interp.Value, bool) {
	env := interp.NewEnvironment(nil)
	env.Define("o", o, true)
	v, err := m.EvalExpression(&ast.MemberExpression{Object: &ast.Identifier{Value: "o"}, Property: name}, env)
	return v, err == nil
}

// coreValue o'nun üyeleri yerleşik olan bir çekirdek değer (liste, metin, sözlük, aralık...)
// olduğunu söyler: yapı, modül ve tensör değil. Kapanıştaki çekirdek değerin yerleşik özelliği ve
// metodu izlenmez (H6987).
func coreValue(m *interp.VM, o interp.Value, member string) bool {
	switch o.(type) {
	case *interp.StructInstanceVal, *TensorValue:
		return false
	case *interp.DictVal:
		_, module := closureMember(m, o, member)
		return !module
	}
	return true
}

// indexValue listenin tam sayı indeksini ya da sözlüğün anahtarını (metin, tam sayı...) okur;
// başka bileşim yol değildir.
func indexValue(o, k interp.Value) (interp.Value, bool) {
	switch c := o.(type) {
	case *interp.ListVal:
		i, ok := k.(*interp.IntVal)
		if !ok {
			return nil, false
		}
		n := int64(len(c.Elements))
		if i.Val < -n || i.Val >= n {
			return nil, false
		}
		if i.Val < 0 {
			return c.Elements[i.Val+n], true
		}
		return c.Elements[i.Val], true
	case *interp.DictVal:
		return c.GetValue(k)
	}
	return nil, false
}
