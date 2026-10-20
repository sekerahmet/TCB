package dataset

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// sentinelDetail önbellek iletisinin (H5008, H5012, H5016, H5027) nöbetçi hatadan sonraki
// ayrıntısını verir: ileti "{error}: ayrıntı" biçimindedir, {error} parametresi boşaltılarak
// ayrıntı metne bakılmadan ayrılır.
func sentinelDetail(err error) string {
	m, ok := messages.As(err)
	if !ok || len(m.Args) == 0 {
		return clean(err.Error())
	}
	if _, isErr := m.Args[0].(error); !isErr {
		return clean(err.Error())
	}
	args := append([]any{""}, m.Args[1:]...)
	return clean(strings.TrimPrefix(messages.Text(m.Code, args...), ": "))
}
