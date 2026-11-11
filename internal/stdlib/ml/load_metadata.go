package ml

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// TR_load_metadata (15.55): GGUF üstverisi TR_JSON olarak. Parçalayıcının anahtarları girmez
// (16.1), yalnız değeri metin olan ön bölme etiketi girer (Sınır A'nın etiketi; parçalayıcıdan
// bağımsız okunur); değerler tensörlerden önce okunur, veri bölümüne bakılmaz.

// ggufTokenizerPrefix üstveriye girmeyen anahtarların önekidir; ggufTokenizerPre o önekten giren tek
// anahtardır, değeri metinse.
const (
	ggufTokenizerPrefix = "tokenizer.ggml."
	ggufTokenizerPre    = "tokenizer.ggml.pre"
)

// metadataLoad TR_load_metadata yerleşiğidir.
func metadataLoad(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnLoadMetadata)
	if len(args) != 1 {
		return nil, messages.New(messages.H5608, fn, len(args))
	}
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
		return nil, err
	}
	fpath, err := yolArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	decoded, tried := bind.ResolvePathTried(m, fpath)
	var text []byte
	var readErr, osErr error
	// Dosyadan yükleme programın durumunu değiştirmez: durdurma onu bırakır (1.5).
	if waitErr := m.Abandonable(func() { text, readErr, osErr = readMetadata(decoded) }); waitErr != nil {
		return nil, waitErr
	}
	if osErr != nil {
		return nil, interp.ReadError(fn, fpath, messages.H2107, osErr, tried)
	}
	if readErr != nil {
		return nil, messages.New(messages.H6626, fn, fpath, readErr)
	}
	v, err := interp.JSONToTR(text)
	if err != nil {
		return nil, messages.New(messages.H6626, fn, fpath, err)
	}
	return interp.NewJSON(v), nil
}

// readMetadata dosyanın başlığını çözer ve üstveriyi JSON metni olarak verir. İlk hata dosya
// biçimi hatası, ikincisi işletim sistemi hatasıdır.
func readMetadata(path string) ([]byte, error, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		return nil, messages.New(messages.H6629), nil
	}
	file, err := gguf.Decode(bufio.NewReaderSize(f, 1<<20), info.Size())
	if err != nil {
		return nil, err, nil
	}
	text, err := metadataJSON(file)
	return text, err, nil
}

// metadataJSON üstveriyi dosyadaki sırayla JSON metnine yazar.
func metadataJSON(f *gguf.File) ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	first := true
	for _, key := range f.Keys {
		if strings.HasPrefix(key, ggufTokenizerPrefix) && !isTokenizerPreText(f, key) {
			continue
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		writeJSONString(&out, key)
		out.WriteByte(':')
		if err := writeMetadataValue(&out, key, f.Values[key].Data); err != nil {
			return nil, err
		}
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

// isTokenizerPreText anahtar ön bölme etiketiyse ve değeri metinse doğrudur; metin olmayan etiket
// (dizi, sayı) yalnız bozuk dosyada olur ve öbür parçalayıcı anahtarları gibi girmez (TCB 02:28).
func isTokenizerPreText(f *gguf.File, key string) bool {
	if key != ggufTokenizerPre {
		return false
	}
	_, ok := f.Values[key].Data.(string)
	return ok
}

// writeMetadataValue değeri yazar; dizi öğe öğe gider.
func writeMetadataValue(out *bytes.Buffer, key string, data any) error {
	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Slice:
		out.WriteByte('[')
		for i := range v.Len() {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeMetadataValue(out, key, v.Index(i).Interface()); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case reflect.String:
		writeJSONString(out, v.String())
	case reflect.Bool:
		out.WriteString(strconv.FormatBool(v.Bool()))
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		out.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.Uint() > math.MaxInt64 {
			return messages.New(messages.H6685, gguf.Clip(key), v.Uint())
		}
		out.WriteString(strconv.FormatUint(v.Uint(), 10))
	default:
		return writeMetadataFloat(out, key, v.Float())
	}
	return nil
}

// writeMetadataFloat ondalığı yazar: float32 değerin float64'teki tam karşılığı (eps ve RoPE tabanı
// llama.cpp'nin gördüğüyle aynı bittedir), yazımı ondalık kalır (3.7a: 2.0 TR_Float'tur).
func writeMetadataFloat(out *bytes.Buffer, key string, x float64) error {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return messages.New(messages.H6685, gguf.Clip(key), x)
	}
	s := strconv.FormatFloat(x, 'g', -1, 64)
	out.WriteString(s)
	if !strings.ContainsAny(s, ".e") {
		out.WriteString(".0")
	}
	return nil
}

// writeJSONString metni JSON yazımıyla ekler. Metin UTF-8 denetiminden geçmiştir.
func writeJSONString(out *bytes.Buffer, s string) {
	b, err := json.Marshal(s)
	if err != nil {
		panic(messages.Internal("json string: " + err.Error()))
	}
	out.Write(b)
}
