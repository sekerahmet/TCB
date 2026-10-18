package veritabani

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

// Bu dosya veritabanının diskle ilişkisini toplar:
//   - Süreç içinde dosya başına tek nesne: aynı dosyayı ikinci kez açan veritabanı_aç ilk
//     nesneyi döndürür; böylece iki nesne birbirinin kaydını ezemez.
//   - Süreçler arası kilit: dosyanın yanındaki "<dosya>.kilit" işletim sistemi kilidiyle
//     tutulur; aynı veritabanını aynı anda ikinci bir program açamaz. Kilit süreç bitince
//     işletim sistemince bırakılır (çöken programın kilidi kalmaz).
//   - Atomik kayıt: içerik aynı klasörde geçici bir dosyaya yazılır, diske işlenir ve asıl
//     dosyanın yerine adlandırılır; yazma yarıda kalırsa eski dosya bozulmaz.
//   - Sayı türü: Tam sayılar tam (int64), Ondalık sayılar ondalık kalır (2.0 diske "2.0"
//     yazılır, yeniden açınca yine Ondalık'tır); 2^53'ten büyük Tam sayılar bozulmaz.

var (
	aciklarMu sync.Mutex
	aciklar   = map[string]*DatabaseVal{}
)

// kayitAnahtari diskteki yolun süreç içi kayıt anahtarıdır: mutlak, temiz ve büyük/küçük
// harf duyarsız dosya sistemlerinde küçük harfli.
func kayitAnahtari(diskYolu string) string {
	abs, err := filepath.Abs(diskYolu)
	if err != nil {
		abs = diskYolu
	}
	abs = filepath.Clean(abs)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		abs = strings.ToLower(abs)
	}
	return abs
}

// errKilitli kilidi başka bir sürecin tuttuğunu bildirir.
var errKilitli = errors.New("kilitli")

// kilitAl dosyanın kilidini alır (zaten tutuluyorsa bir şey yapmaz). Kilit dosyası
// açılamazsa (ör. klasör yok) o hata, kilit başkasındaysa errKilitli döner.
func (db *DatabaseVal) kilitAl() error {
	if db.kilit != nil || !kilitDestekli {
		return nil
	}
	f, err := os.OpenFile(db.yol()+".kilit", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	if err := dosyaKilitle(f); err != nil {
		f.Close()
		return errKilitli
	}
	db.kilit = f
	return nil
}

func (db *DatabaseVal) kilitHatasi(fn string) error {
	return fmt.Errorf("%s: %q başka bir program tarafından açık; aynı veritabanını aynı anda iki program kullanamaz (öteki programı kapatıp yeniden deneyin)", fn, db.FilePath)
}

// veritabaniAc dosyayı açar ya da bu süreçte zaten açıksa aynı nesneyi döndürür.
func veritabaniAc(v *vm.VM, path string) (*DatabaseVal, error) {
	db := &DatabaseVal{FilePath: path, Tables: make(map[string]*dbTable)}
	if path == bellekYolu {
		return db, nil
	}
	db.diskYolu, _ = v.YolCoz(path)
	anahtar := kayitAnahtari(db.yol())
	aciklarMu.Lock()
	defer aciklarMu.Unlock()
	if onceki, ok := aciklar[anahtar]; ok {
		return onceki, nil
	}
	// Klasör henüz yoksa kilit ilk kayıtta alınır (o zaman da klasör yoksa kayıt hatası verir)
	if err := db.kilitAl(); errors.Is(err, errKilitli) {
		return nil, db.kilitHatasi("veritabanı_aç")
	}
	if err := db.loadFromDisk(); err != nil {
		if db.kilit != nil {
			db.kilit.Close()
		}
		return nil, err
	}
	aciklar[anahtar] = db
	return db, nil
}

// yenidenAdlandir Windows'ta, dosyayı kısa süre tutan başka programlar (virüs tarayıcı,
// dizin oluşturucu) yüzünden ara sıra başarısız olan yeniden adlandırmayı birkaç kez dener.
func yenidenAdlandir(eski, yeni string) error {
	err := os.Rename(eski, yeni)
	for i := 0; err != nil && runtime.GOOS == "windows" && i < 5; i++ {
		time.Sleep(20 * time.Millisecond)
		err = os.Rename(eski, yeni)
	}
	return err
}

func (db *DatabaseVal) saveToDisk() error {
	if db.FilePath == "" || db.FilePath == bellekYolu {
		return nil
	}
	data, err := json.MarshalIndent(db.Tables, "", "  ")
	if err != nil {
		return fmt.Errorf("veritabanı: %q kaydedilemedi: %v", db.FilePath, err)
	}
	if err := db.kilitAl(); err != nil {
		if errors.Is(err, errKilitli) {
			return db.kilitHatasi("veritabanı")
		}
		return turkce.DosyaHatasi("veritabanı", db.FilePath, "kaydedilemedi", err)
	}
	yol := db.yol()
	gecici, err := os.CreateTemp(filepath.Dir(yol), "."+filepath.Base(yol)+".*.gecici")
	if err != nil {
		return turkce.DosyaHatasi("veritabanı", db.FilePath, "kaydedilemedi", err)
	}
	geciciAd := gecici.Name()
	_, err = gecici.Write(data)
	if err == nil {
		err = gecici.Sync()
	}
	if kerr := gecici.Close(); err == nil {
		err = kerr
	}
	if err == nil {
		kip := os.FileMode(0644)
		if bilgi, serr := os.Stat(yol); serr == nil && bilgi.Mode().IsRegular() {
			kip = bilgi.Mode().Perm()
		}
		_ = os.Chmod(geciciAd, kip)
		err = yenidenAdlandir(geciciAd, yol)
	}
	if err != nil {
		os.Remove(geciciAd)
		return turkce.DosyaHatasi("veritabanı", db.FilePath, "kaydedilemedi", err)
	}
	return nil
}

func (db *DatabaseVal) loadFromDisk() error {
	if db.FilePath == "" || db.FilePath == bellekYolu {
		return nil
	}
	data, err := os.ReadFile(db.yol())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return turkce.DosyaHatasi("veritabanı_aç", db.FilePath, "yüklenemedi", err)
	}
	bozuk := func(neden string) error {
		return fmt.Errorf("veritabanı_aç: %q yüklenemedi: dosya geçerli bir TR veritabanı (JSON) değil%s", db.FilePath, neden)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var okunan map[string]*dbTable
	if err := dec.Decode(&okunan); err != nil {
		return bozuk("")
	}
	tablolar := make(map[string]*dbTable, len(okunan))
	for k, t := range okunan {
		// Dosya "null" içeriyorsa veya tablolar eksik alanlarla geldiyse nil map/slice paniklerini önle
		if t == nil {
			continue
		}
		if t.Name == "" {
			t.Name = k
		}
		rows := make([]map[string]interface{}, 0, len(t.Rows))
		for _, r := range t.Rows {
			if r == nil {
				continue
			}
			for alan, d := range r {
				c, err := diskten(d)
				if err != nil {
					return bozuk(fmt.Sprintf(" (%s tablosu, %s alanı: %v)", t.Name, alan, err))
				}
				r[alan] = c
			}
			rows = append(rows, r)
		}
		t.Rows = rows
		if t.Columns == nil {
			t.Columns = []string{}
		}
		t.idSayaciniDuzelt()
		anahtar := tabloAnahtari(t.Name)
		if _, var_ := tablolar[anahtar]; var_ {
			return bozuk(fmt.Sprintf(" (%q adı Türkçe büyük/küçük harf kuralıyla başka bir tabloyla aynı)", t.Name))
		}
		tablolar[anahtar] = t
	}
	db.Tables = tablolar
	return nil
}

// ondalik bellekteki satırlarda Ondalık değerdir. Diske her zaman ondalık olarak yazılır
// (2.0 → "2.0"), böylece yeniden açınca Tam'a dönmez.
type ondalik float64

func (o ondalik) MarshalJSON() ([]byte, error) {
	f := float64(o)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, fmt.Errorf("%s sayısı veritabanına yazılamaz", (&vm.FloatVal{Val: f}).Inspect())
	}
	bicim := byte('f')
	if a := math.Abs(f); a != 0 && (a < 1e-6 || a >= 1e21) {
		bicim = 'e'
	}
	b := strconv.AppendFloat(nil, f, bicim, -1, 64)
	if bytes.IndexAny(b, ".eE") < 0 {
		b = append(b, '.', '0')
	}
	return b, nil
}

// diskten diskten okunan JSON değerini bellekteki satır biçimine çevirir: nokta ya da üs
// içermeyen sayılar Tam (int64; aralık dışıysa hata), ötekiler Ondalık'tır.
func diskten(d interface{}) (interface{}, error) {
	switch x := d.(type) {
	case json.Number:
		s := string(x)
		if !strings.ContainsAny(s, ".eE") {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%s Tam aralığının dışında", s)
			}
			return n, nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("%s geçerli bir sayı değil", s)
		}
		return ondalik(f), nil
	case []interface{}:
		for i, el := range x {
			c, err := diskten(el)
			if err != nil {
				return nil, err
			}
			x[i] = c
		}
		return x, nil
	case map[string]interface{}:
		for k, el := range x {
			c, err := diskten(el)
			if err != nil {
				return nil, err
			}
			x[k] = c
		}
		return x, nil
	}
	return d, nil
}

// satirDegeri bir TR değerini veritabanı satırında saklanacak biçime çevirir. Döngüsel ya da
// çok derin değerler, Tanımsız (NaN) ve sonsuz sayılar diske yazılamayacağı için eklenirken
// reddedilir (eskiden kabul edilip sonraki her kaydı sessizce bozuyordu).
func satirDegeri(fn string, val vm.Value) (interface{}, error) { return satiraCevir(fn, val, true) }

// bellekSatirDegeri tablo_sorgula'nın bellekteki satırları içindir: diske yazılmadığı için
// Tanımsız ve sonsuz sayılar serbesttir.
func bellekSatirDegeri(fn string, val vm.Value) (interface{}, error) {
	return satiraCevir(fn, val, false)
}

func satiraCevir(fn string, val vm.Value, disk bool) (interface{}, error) {
	g, err := vm.TRToGo(val)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", fn, err)
	}
	c, err := ondaligaCevir(g, disk)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", fn, err)
	}
	return c, nil
}

func ondaligaCevir(g interface{}, disk bool) (interface{}, error) {
	switch x := g.(type) {
	case float64:
		if disk && (math.IsNaN(x) || math.IsInf(x, 0)) {
			return nil, fmt.Errorf("%s sayısı veritabanına yazılamaz (Tanımsız ve sonsuz sayılar saklanamaz)", (&vm.FloatVal{Val: x}).Inspect())
		}
		return ondalik(x), nil
	case []interface{}:
		for i, el := range x {
			c, err := ondaligaCevir(el, disk)
			if err != nil {
				return nil, err
			}
			x[i] = c
		}
	case map[string]interface{}:
		for k, el := range x {
			c, err := ondaligaCevir(el, disk)
			if err != nil {
				return nil, err
			}
			x[k] = c
		}
	}
	return g, nil
}

// trDegeri satırdaki bir değeri TR değerine çevirir; Tam/Ondalık ayrımı korunur. İç içe
// sözlüklerin anahtarları (JSON nesnesi sırasız olduğundan) alfabetik sıradadır.
func trDegeri(raw interface{}) vm.Value {
	switch x := raw.(type) {
	case nil:
		return vm.NULL
	case bool:
		return vm.BoolToVal(x)
	case int64:
		return vm.NewInt(x)
	case ondalik:
		return &vm.FloatVal{Val: float64(x)}
	case float64:
		return &vm.FloatVal{Val: x}
	case string:
		return &vm.StringVal{Val: x}
	case []interface{}:
		el := make([]vm.Value, len(x))
		for i, e := range x {
			el[i] = trDegeri(e)
		}
		return &vm.ListVal{Elements: el}
	case map[string]interface{}:
		anahtarlar := make([]string, 0, len(x))
		for k := range x {
			anahtarlar = append(anahtarlar, k)
		}
		sort.Strings(anahtarlar)
		d := vm.NewDictVal()
		for _, k := range anahtarlar {
			d.Set(k, trDegeri(x[k]))
		}
		return d
	}
	return vm.GoToTRValue(raw)
}
