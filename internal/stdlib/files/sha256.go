package files

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Sha256Hex r'nin sonuna kadar okunan baytlarının sha256'sını 64 küçük onaltılık karakterle verir
// (22.5'in biçimi; 20.5'in digest.json'u ve Hugging Face'in lfs.oid'i de böyledir).
func Sha256Hex(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// sha256Builtin sha256(b) yerleşiğidir (22.5): Baytlar'ın sha256'sı. Baytlar değişmez olduğu için
// karma doğrudan hesaplanır.
func sha256Builtin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnSha256, 1, len(args))
	}
	b, ok := args[0].(*interp.BytesVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.FnSha256, 1, args[0], codes.TypeBytes)
	}
	sum := sha256.Sum256(b.Data())
	return &interp.StringVal{Val: hex.EncodeToString(sum[:])}, nil
}

// sha256FileBuiltin dosya_sha256(yol) yerleşiğidir (22.5): diskteki baytların sha256'sı. .gz
// açılmaz (dosya_baytları gibi, 18.12). Dosya akışla okunur, bellek dosyanın boyundan bağımsızdır;
// okuma yorumlayıcı kilidi bırakılarak yapılır ve durumu değiştirmediği için durdurmada bırakılır
// (Abandonable, 10.9). Yol okuma sırasıyla çözülür (18.16); klasör H7729, olmayan dosya 18'in okuma
// hatasıdır.
func sha256FileBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnSha256File, 1, len(args))
	}
	raw, err := readPath(codes.FnSha256File, 1, args[0])
	if err != nil {
		return nil, err
	}
	fn := interp.CallPrefix(codes.FnSha256File)
	resolved, tried := m.ResolvePath(raw)
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, interp.ReadError(fn, raw, messages.H2100, err, tried)
	}
	if info.IsDir() {
		return nil, messages.New(messages.H7729, fn, raw)
	}
	var sum string
	var readErr error
	if waitErr := m.Abandonable(func() { sum, readErr = sha256OfFile(resolved) }); waitErr != nil {
		return nil, waitErr
	}
	if readErr != nil {
		return nil, interp.ReadError(fn, raw, messages.H2100, readErr, tried)
	}
	return &interp.StringVal{Val: sum}, nil
}

// sha256OfFile dosyayı açar ve akışla karmalar.
func sha256OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return Sha256Hex(f)
}
