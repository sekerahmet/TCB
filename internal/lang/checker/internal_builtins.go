package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// internalUse, id kitaplığın iç yerleşiğiyse (definitions.json "internal_to", 11.7a) ve denetlenen
// metin o kitaplık modülünün gömülü kaynağı değilse H6998'i yazar ve true döner. Çağrı da anma da
// (TR_var f = TR_cache_buffer) aynı kuraldır: ikisi de ad okumasıdır. Kitaplığın kaynağı yalnız
// gömülü dosyanın metniyse kaynaktır (SetLibraryFile, interp.LibraryFileName): aynı metnin kopyası
// kullanıcının programıdır. Kullanıcının gölgelediği ad (bir dilde yazılan satırda kendi
// değişkeni) kitaplığın yerleşiği değildir.
func (d *nameChecker) internalUse(id *ast.Identifier, k *nameScope) bool {
	code := id.Token.Code
	if code == codes.None {
		// Önce dilin adı mı ve iç mi diye bakılır: kapsamları gezmek yalnız o adda.
		named, ok := d.lang.Find(codes.NamespaceGlobal, id.Value)
		if !ok || named.InternalTo() == "" {
			return false
		}
		if kind, _, _ := d.resolve(id.Value, k); kind != BindBuiltin {
			return false
		}
		code = named
	}
	module := code.InternalTo()
	if module == "" || d.c.libraryFile == strings.TrimPrefix(module, "tr/")+".tr" {
		return false
	}
	d.c.nameErrors = append(d.c.nameErrors, messages.AtRange(id.Token.Range(), messages.New(messages.H6998)))
	return true
}
