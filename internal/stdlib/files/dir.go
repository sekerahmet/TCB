package files

import (
	"os"
	"path/filepath"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// makeDirBuiltin klasör_oluştur(yol) yerleşiğidir: klasörü üst klasörleriyle oluşturur
// (varsa sorun değildir) ve Yol olarak döndürür.
func makeDirBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G122)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.G122, 1, len(args), messages.Text(messages.H2318))
	}
	raw, err := readPath(codes.G122, 1, args[0])
	if err != nil {
		return nil, err
	}
	resolved, _ := m.ResolvePath(filepath.FromSlash(raw))
	if b, err := os.Stat(resolved); err == nil && !b.IsDir() {
		return nil, messages.New(messages.H7123, fn, raw)
	}
	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return nil, turkish.FileError(fn, raw, messages.H2105, err)
	}
	return NewPath(raw), nil
}
