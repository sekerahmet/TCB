package main

import (
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/sysmem"
)

// Programın tek bellek ayarı bellek tavanıdır (dil tanımı 19.12a): TR_MEMORY_LIMIT, bayt cinsinden.
// trlang çöp toplayıcının hedefini seçmez, Go'nun 100'ü kalır; ayar programın türüne göre gizlice
// seçilmez. Go çalışma zamanının ortam değişkenleri (GOGC, GOMEMLIMIT ...) dilin ayarı değildir: Go
// onları uygular, trlang süreç başında bir satırla söyler; TR_MEMORY_LIMIT de verilmişse Go'nun
// sınırını tavan kurar.

// runtimeVariables Go'nun çalışma zamanının ortam değişkenleridir, uyarı satırındaki sırayla (go1.27.4
// src/runtime/extern.go). Testlerin listesi testkit.RuntimeVariables'tır; ikisi aynıdır
// (TestRuntimeVariableListsAgree).
var runtimeVariables = []string{"GOGC", "GOMEMLIMIT", "GODEBUG", "GOMAXPROCS", "GORACE", "GOTRACEBACK"}

// memoryRuntimeVariables tavanın yerine yazılan değişkenlerdir: uyarı onlar için tavanı önerir (H8893).
var memoryRuntimeVariables = []string{"GOGC", "GOMEMLIMIT"}

// applyMemorySettings TR_MEMORY_LIMIT verilmişse tavanı kurar ve toplayıcının payını izleyeni başlatır
// (collector_share.go); değer geçersizse hatayı döner ve program başlamaz. Sonra ortamdaki çalışma zamanı değişkenlerini stderr'e tek satırla yazar.
func applyMemorySettings(stderr io.Writer) error {
	if v, ok := codes.EnvValue(codes.EnvMemoryLimit); ok {
		n, err := parseMemoryLimit(v)
		if err != nil {
			return err
		}
		sysmem.SetLimit(n)
		watchCollectorShare(stderr, n)
	}
	if line, ok := runtimeVariablesWarning(os.LookupEnv); ok {
		fmt.Fprintln(stderr, "⚠️ "+line)
	}
	return nil
}

// runtimeVariablesWarning ortamda değeri olan çalışma zamanı değişkenlerinin uyarısıdır: GOGC ya da
// GOMEMLIMIT aralarındaysa tavanı öneren H8893, değilse H8895. Boş değer Go'yu etkilemez, sayılmaz.
func runtimeVariablesWarning(lookup func(string) (string, bool)) (string, bool) {
	var set []string
	memory := false
	for _, name := range runtimeVariables {
		if v, ok := lookup(name); ok && v != "" {
			set = append(set, name)
			memory = memory || containsName(memoryRuntimeVariables, name)
		}
	}
	switch {
	case len(set) == 0:
		return "", false
	case memory:
		return messages.Text(messages.H8893, len(set), strings.Join(set, ", "), codes.NameRef(codes.EnvMemoryLimit)), true
	default:
		return messages.Text(messages.H8895, len(set), strings.Join(set, ", ")), true
	}
}

func containsName(list []string, name string) bool {
	for _, x := range list {
		if x == name {
			return true
		}
	}
	return false
}

// unitValue birimli bir değerdir: sayı (ondalık ayırıcı nokta ya da virgül), isteğe bağlı boşluk, birim.
var unitValue = regexp.MustCompile(`^\s*([0-9]+(?:[.,][0-9]+)?)\s*([A-Za-z]+)\s*$`)

// binaryUnits tek okunan birimlerdir (2'nin kuvvetleri, B bayttır); ambiguousUnits hem 10'un hem 2'nin
// kuvvetiyle okunur. Değer, birimin 1024'lük ya da 1000'lik basamağıdır; harf büyüklüğü önemsizdir.
var (
	binaryUnits    = map[string]int{"b": 0, "kib": 1, "mib": 2, "gib": 3, "tib": 4}
	ambiguousUnits = map[string]int{"k": 1, "kb": 1, "m": 2, "mb": 2, "g": 3, "gb": 3, "t": 4, "tb": 4}
)

// Geçersiz değerin iki kolu (H8890): int64'e sığmayan değer üst sınırı söyler; "tavan yok" sayılmaz,
// sessiz olurdu.
const (
	limitTooLarge = "toolarge"
	limitInvalid  = "other"
)

// parseMemoryLimit TR_MEMORY_LIMIT'in değerini okur: geçerli değer sıfırdan büyük, int64'e sığan ASCII
// rakamlarıdır. Birimli değer bayt yazımını önerir (H8891, iki okunan birimde H8892); öbür her değer
// H8890'dır.
func parseMemoryLimit(v string) (int64, error) {
	name := codes.NameRef(codes.EnvMemoryLimit)
	invalid := func(kind string) error { return messages.New(messages.H8890, name, v, kind) }
	if isASCIIDigits(v) {
		n, err := strconv.ParseInt(v, 10, 64)
		switch {
		case err != nil: // yalnız rakam: tek hata aralığın aşılmasıdır
			return 0, invalid(limitTooLarge)
		case n == 0:
			return 0, invalid(limitInvalid)
		}
		return n, nil
	}
	m := unitValue.FindStringSubmatch(v)
	if m == nil {
		return 0, invalid(limitInvalid)
	}
	number, unit := strings.Replace(m[1], ",", ".", 1), strings.ToLower(m[2])
	if p, ok := binaryUnits[unit]; ok {
		bytes, kind := unitBytes(number, 1024, p)
		if kind != "" {
			return 0, invalid(kind)
		}
		return 0, messages.New(messages.H8891, name, v, bytes)
	}
	if p, ok := ambiguousUnits[unit]; ok {
		decimal, kindDecimal := unitBytes(number, 1000, p)
		binary, kindBinary := unitBytes(number, 1024, p)
		switch {
		case kindDecimal == limitTooLarge || kindBinary == limitTooLarge:
			return 0, invalid(limitTooLarge)
		case kindDecimal != "" || kindBinary != "":
			return 0, invalid(limitInvalid)
		}
		return 0, messages.New(messages.H8892, name, v, decimal, binary)
	}
	return 0, invalid(limitInvalid)
}

// unitBytes number × base^power'ı tam olarak hesaplar. int64'e sığmıyorsa limitTooLarge, tam sayı
// değilse ya da sıfırsa limitInvalid döner.
func unitBytes(number string, base int64, power int) (int64, string) {
	r, ok := new(big.Rat).SetString(number)
	if !ok {
		return 0, limitInvalid
	}
	factor := new(big.Int).Exp(big.NewInt(base), big.NewInt(int64(power)), nil)
	r.Mul(r, new(big.Rat).SetInt(factor))
	switch {
	case r.Cmp(new(big.Rat).SetInt64(math.MaxInt64)) > 0:
		return 0, limitTooLarge
	case !r.IsInt() || r.Sign() == 0:
		return 0, limitInvalid
	}
	return r.Num().Int64(), ""
}

func isASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
