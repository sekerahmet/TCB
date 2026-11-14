// Package download TR'nin indirme yerleşiğidir: indir(adres, hedef, sha256: k, bayt: n) (28.10) ve
// istemcinin güvenlik kuralları (28.11).
//
// Baytlar hedefin yanındaki <hedef>.part'a yazılır; .part açılırken kilitlenir, adı yalnız kilit
// tutulurken değişir, hiçbir zaman silinmez, içeriği sıfırlanır. Hedef ancak boy ve karma (verilmişse)
// tutunca tek adımda yerine konur. İndirme çağıran görevde koşar; her okuma yorumlayıcının kilidi
// bırakılarak yapılır (AwaitExternal) ve iptal bir okuma sınırında durur: .part kalır, son ad değişmez.
// İstemci yalnız https ile (127.0.0.1 dışında) ve kendi kurallarıyla bağlanır: yerel adrese çözülen
// ad, yerel adrese yönlendirme, ortamdan örtük vekil, sıkıştırılmış gövde yoktur.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package download

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Adlı argümanlar; adları kod listesindedir.
var (
	nameSha256     = bind.NamedArg(codes.ArgSha256)
	nameSize       = bind.NamedArg(codes.ArgSize)
	nameOnProgress = bind.NamedArg(codes.ArgOnProgress)
	nameProxy      = bind.NamedArg(codes.ArgProxy)
	nameOverwrite  = bind.NamedArg(codes.ArgOverwrite)
)

// Register indirme yerleşiğini VM'ye kaydeder.
func Register(v *interp.VM) {
	v.RegisterBuiltinCode(codes.FnDownload, bind.SafeBuiltin(interp.CallPrefix(codes.FnDownload), downloadBuiltin))
}

// call bir indirme çağrısının denetlenmiş argümanlarıdır.
type call struct {
	fn        any
	url       *url.URL
	target    string // çözülmüş hedef
	userPath  string // programın yazdığı hedef (iletilerde)
	hash      string
	hasHash   bool
	size      int64
	hasSize   bool
	progress  interp.Value
	proxy     *url.URL
	overwrite bool
}

func (c *call) partPath() string { return c.target + ".part" }

// downloadBuiltin indir(adres, hedef, sha256: k, bayt: n) yerleşiğidir (28.10).
func downloadBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	c, err := readCall(m, args, named)
	if err != nil {
		return nil, err
	}
	if err := c.checkTarget(); err != nil {
		return nil, err
	}
	p, err := openPart(c.partPath())
	if err != nil {
		return nil, c.partError(err)
	}
	defer p.close()
	sum, err := c.run(m, p)
	if err != nil {
		return nil, err
	}
	return &interp.StringVal{Val: sum}, nil
}

// readCall argümanları okur ve denetler; istekten önce biter.
func readCall(m *interp.VM, args []interp.Value, named map[string]interp.Value) (*call, error) {
	fn := interp.CallPrefix(codes.FnDownload)
	if len(args) != 2 {
		return nil, interp.ArgCountError(codes.FnDownload, 2, len(args))
	}
	if err := bind.CheckNamed(fn, named, nameSha256, nameSize, nameOnProgress, nameProxy, nameOverwrite); err != nil {
		return nil, err
	}
	raw, ok := args[0].(*interp.StringVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.FnDownload, 1, args[0], codes.TypeString)
	}
	target, ok, err := interp.PathArg(fn, args[1])
	if !ok {
		return nil, interp.ArgTypeError(codes.FnDownload, 2, args[1], codes.TypeString, codes.TypePath)
	}
	if err != nil {
		return nil, err
	}
	c := &call{fn: fn, userPath: target}
	if c.url, err = parseTarget(fn, raw.Val); err != nil {
		return nil, err
	}
	if err := c.readHash(named); err != nil {
		return nil, err
	}
	if err := c.readSize(named); err != nil {
		return nil, err
	}
	if err := c.readOptional(named); err != nil {
		return nil, err
	}
	c.target, _ = m.ResolveWritePath(filepath.FromSlash(target))
	return c, nil
}

// readHash TR_sha256:'yı okur: zorunludur (H7700); TR_none karmasızdır; büyük harf H7702, başka
// biçim H7703.
func (c *call) readHash(named map[string]interp.Value) error {
	v, ok := bind.GetNamed(named, nameSha256)
	if !ok {
		return messages.New(messages.H7700, c.fn)
	}
	if v == interp.Null {
		return nil
	}
	s, ok := v.(*interp.StringVal)
	if !ok {
		return messages.New(messages.H7604, c.fn, nameSha256, interp.TypeNames(codes.TypeString, codes.TypeNoneType), v.Type())
	}
	if !isLowerHex64(s.Val) {
		if isLowerHex64(strings.ToLower(s.Val)) {
			return messages.New(messages.H7702, c.fn)
		}
		return messages.New(messages.H7703, c.fn, s.Val)
	}
	c.hash, c.hasHash = s.Val, true
	return nil
}

func isLowerHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// readSize TR_size:'ı okur: zorunludur (H7701); TR_none boysuzdur; eksi H7704.
func (c *call) readSize(named map[string]interp.Value) error {
	v, ok := bind.GetNamed(named, nameSize)
	if !ok {
		return messages.New(messages.H7701, c.fn)
	}
	if v == interp.Null {
		return nil
	}
	n, ok := bind.Int(v)
	if !ok {
		return messages.New(messages.H7604, c.fn, nameSize, interp.TypeNames(codes.TypeInt, codes.TypeNoneType), v.Type())
	}
	if n < 0 {
		return messages.New(messages.H7704, c.fn, n)
	}
	c.size, c.hasSize = n, true
	return nil
}

// readOptional TR_on_progress:, TR_proxy: ve TR_overwrite:'ı okur.
func (c *call) readOptional(named map[string]interp.Value) error {
	if v, ok := bind.GetNamed(named, nameOnProgress); ok && v != interp.Null {
		switch v.(type) {
		case *interp.FuncVal, *interp.BuiltinVal:
			c.progress = v
		default:
			return messages.New(messages.H7604, c.fn, nameOnProgress, interp.TypeNames(codes.TypeFunction), v.Type())
		}
	}
	if v, ok := bind.GetNamed(named, nameProxy); ok && v != interp.Null {
		s, ok := v.(*interp.StringVal)
		if !ok {
			return messages.New(messages.H7604, c.fn, nameProxy, interp.TypeNames(codes.TypeString, codes.TypeNoneType), v.Type())
		}
		u, err := parseProxy(c.fn, s.Val)
		if err != nil {
			return err
		}
		c.proxy = u
	}
	var err error
	c.overwrite, err = bind.OptionalBool(c.fn, map[string]interp.Value(named), nameOverwrite, false)
	return err
}

// checkTarget hedefin kurallarını istekten önce denetler: klasörü yoksa H7724, klasörse H7725,
// varsa ve TR_overwrite: yoksa H7726; hedef ya da .part bağlantıysa H7723 (izlenmez).
func (c *call) checkTarget() error {
	dir := filepath.Dir(c.target)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return messages.New(messages.H7724, c.fn, filepath.ToSlash(filepath.Dir(c.userPath)))
	}
	for _, p := range []string{c.target, c.partPath()} {
		if fi, err := os.Lstat(p); err == nil && isLink(fi) {
			return messages.New(messages.H7723, c.fn, filepath.ToSlash(p))
		}
	}
	if fi, err := os.Stat(c.target); err == nil {
		if fi.IsDir() {
			return messages.New(messages.H7725, c.fn, c.userPath)
		}
		if !c.overwrite {
			return messages.New(messages.H7726, c.fn, c.userPath)
		}
	}
	return nil
}

// isLink sembolik bağlantı ve Windows'ta öbür yeniden ayrıştırma noktalarıdır (18.7 gibi).
func isLink(fi os.FileInfo) bool {
	return fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

// partError .part'ın açılış hatasını iletiye çevirir: kilit başkasındaysa H7722, öbürleri H7728.
func (c *call) partError(err error) error {
	if err == errBusy {
		return messages.New(messages.H7722, c.fn, c.userPath+".part")
	}
	return messages.New(messages.H7728, c.fn, c.userPath+".part")
}
