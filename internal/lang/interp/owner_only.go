package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ownerOnlyArg TR_owner_only: adlı argümanını okur (18.13); verilmezse yanlıştır. Başka adlı
// argüman hatadır.
func ownerOnlyArg(kd codes.Code, named map[string]Value) (bool, error) {
	ownerOnly := false
	for name, val := range named {
		if !isSpelling(codes.ArgOwnerOnly, name) {
			return false, messages.New(messages.H7901, CallPrefix(kd), name, codes.NameRef(codes.ArgOwnerOnly))
		}
		b, ok := val.(*BoolVal)
		if !ok {
			return false, messages.New(messages.H7905, CallPrefix(kd), codes.NameRef(codes.ArgOwnerOnly), val.Type())
		}
		ownerOnly = b.Val
	}
	return ownerOnly, nil
}
