package profiler

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// jsonKeys bir tipin JSON anahtarlarını yol olarak sıralı verir ("tensör.işlemler.kod");
// etiketsiz dışa açık alan ya da "-" olmayan her alan etiketli olmalıdır (eksikse errs'e yazılır).
func jsonKeys(typ reflect.Type, prefix string, keys *[]string, errs *[]string) {
	for typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" {
			*errs = append(*errs, prefix+f.Name)
			continue
		}
		if name == "-" {
			continue
		}
		path := prefix + name
		*keys = append(*keys, path)
		jsonKeys(f.Type, path+".", keys, errs)
	}
}

// Profil dosyasının JSON anahtarları okuyan araçların (tr watch, tools/gpu/phase3_table.py)
// sözleşmesidir: her alan açık bir anahtar taşır ve anahtar kümesi biçim sürümüne bağlıdır.
// Anahtar eklemek, çıkarmak ya da değiştirmek FormatVersion'ı artırmayı gerektirir: yeni
// sürümün anahtar kümesi testdata/keys_v<sürüm>.txt'ye (sıralı yollar, satırda bir) eklenir,
// eskisi silinmez.
func TestSnapshotKeysFrozenPerVersion(t *testing.T) {
	var keys, errs []string
	jsonKeys(reflect.TypeOf(Snapshot{}), "", &keys, &errs)
	for _, e := range errs {
		t.Errorf("Snapshot.%s için json anahtarı TR_none; dosya biçimi Go adına bağlı kalır", e)
	}
	sort.Strings(keys)
	// Anahtarlar ve biçimin adı dilden bağımsız İngilizce tanımlayıcılardır (dil tanımı 17.3).
	testkit.FormatNames(t, keys...)
	testkit.FormatNames(t, FormatName, StatusFileName)
	data, err := os.ReadFile(filepath.Join("testdata", fmt.Sprintf("keys_v%d.txt", FormatVersion)))
	if err != nil {
		t.Fatalf("FormatVersion %d için testdata/keys_v%d.txt yok; yeni sürümün anahtarlarını ekleyin", FormatVersion, FormatVersion)
	}
	want := strings.Fields(string(data))
	if got := strings.Join(keys, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("sürüm %d anahtar kümesi değişti; biçim değiştiyse FormatVersion'ı artırıp yeni kümeyi ekleyin.\nşimdiki:\n%s", FormatVersion, got)
	}
}
