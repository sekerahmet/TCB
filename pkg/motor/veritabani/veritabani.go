package veritabani

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"tr_dili/pkg/motor/internal/baglama"
	"tr_dili/pkg/sozluk"
	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

// Adlı argümanlar; adları sözlüktedir (pkg/sozluk, W9xx).
var (
	adKosul        = baglama.KodAd(sozluk.W + 930)
	adSirala       = baglama.KodAd(sozluk.W + 931)
	adLimit        = baglama.KodAd(sozluk.W + 932)
	adAlanlar      = baglama.KodAd(sozluk.W + 933)
	adParametreler = baglama.KodAd(sozluk.W + 934)
)

// parametreYazimlari parametre listesinin adlı argüman yazımlarıdır; sonra gelen öncekini
// geçersiz kılar.
var parametreYazimlari = adParametreler.Yazimlar()

var VAL_DATABASE = vm.TipAdi(sozluk.A + 240) // Veritabanı

// dbTable bir tablodur. Satır değerleri: nil, bool, int64 (Tam), ondalik (Ondalık), string,
// []interface{}, map[string]interface{} (bkz. depo.go).
type dbTable struct {
	Name    string                   `json:"name"`
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	NextID  int64                    `json:"next_id"`
}

type DatabaseVal struct {
	FilePath string `json:"file_path"`
	// diskYolu FilePath'in TR'nin tek yol kuralıyla (vm.YolCoz) çözülmüş hâlidir;
	// okuma ve yazma hep bu dosyaya yapılır, mesajlarda kullanıcının yazdığı FilePath görünür.
	diskYolu string
	// Tables tablolardır; anahtar tablo adının Türkçe kurala göre küçük harfli hâlidir.
	Tables map[string]*dbTable `json:"tables"`
	mu     sync.RWMutex
	// kilit süreçler arası kilidi tutan "<dosya>.kilit" dosyasıdır (bkz. depo.go).
	kilit *os.File
}

func (db *DatabaseVal) Type() vm.ValueType { return VAL_DATABASE }
func (db *DatabaseVal) Inspect() string {
	loc := db.FilePath
	if loc == "" || loc == ":bellek:" {
		loc = ":bellek:"
	}
	return fmt.Sprintf("<Veritabanı %q (%d tablo)>", loc, len(db.Tables))
}
func (db *DatabaseVal) IsTruthy() bool { return true }

// yol veritabanı dosyasının diskteki yoludur.
func (db *DatabaseVal) yol() string {
	if db.diskYolu != "" {
		return db.diskYolu
	}
	return db.FilePath
}

// tabloAnahtari tablo adının karşılaştırma biçimidir: Türkçe kurala göre küçük harf
// (IŞIK ≡ ışık, İşler ≡ işler).
func tabloAnahtari(ad string) string { return turkce.KucukHarf(ad) }

// tabloBul var olan bir tabloyu verir; yoksa en yakın adı öneren bir hata döner (yanlış
// yazılmış tablo adı sessizce boş sonuç vermez).
func (db *DatabaseVal) tabloBul(fn, ad string) (*dbTable, error) {
	if t, ok := db.Tables[tabloAnahtari(ad)]; ok {
		return t, nil
	}
	var adlar []string
	for _, t := range db.Tables {
		adlar = append(adlar, t.Name)
	}
	sort.Strings(adlar)
	mesaj := fmt.Sprintf("%s: %q adında tablo yok", fn, ad)
	if o := turkce.EnYakin(ad, adlar); o != "" {
		mesaj += fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	} else if len(adlar) > 0 {
		mesaj += " (var olan tablolar: " + strings.Join(adlar, ", ") + ")"
	} else {
		mesaj += " (veritabanında henüz tablo yok)"
	}
	return nil, fmt.Errorf("%s; yeni tablo tablo_oluştur ya da ekle ile oluşturulur", mesaj)
}

func (t *dbTable) kopya() *dbTable {
	return &dbTable{
		Name:    t.Name,
		Columns: append([]string{}, t.Columns...),
		Rows:    append([]map[string]interface{}{}, t.Rows...),
		NextID:  t.NextID,
	}
}

// degistir bir tabloyu kopyası üzerinde değiştirir ve diske yazar. islem ya da kayıt hata
// verirse bellekteki tablo eski hâlinde kalır; böylece bellek ile disk hiç ayrışmaz ve her
// yazma hatası kullanıcıya döner. olustur doğruysa olmayan tablo oluşturulur, değilse
// tabloBul hatası verilir. Çağıran db.mu'yu yazma için tutar.
func (db *DatabaseVal) degistir(fn, ad string, olustur bool, islem func(t *dbTable) error) error {
	anahtar := tabloAnahtari(ad)
	eski, vardi := db.Tables[anahtar]
	var yeni *dbTable
	switch {
	case vardi:
		yeni = eski.kopya()
	case olustur:
		yeni = &dbTable{Name: ad, Columns: []string{}, Rows: []map[string]interface{}{}, NextID: 1}
	default:
		_, err := db.tabloBul(fn, ad)
		return err
	}
	if err := islem(yeni); err != nil {
		return err
	}
	db.Tables[anahtar] = yeni
	if err := db.saveToDisk(); err != nil {
		if vardi {
			db.Tables[anahtar] = eski
		} else {
			delete(db.Tables, anahtar)
		}
		return err
	}
	return nil
}

// idAnahtari bir id değerinin teklik denetimindeki anahtarıdır; id Tam, Ondalık, Metin ya
// da Mantık olabilir.
func idAnahtari(id interface{}) (string, bool) {
	switch x := id.(type) {
	case int64:
		return "s" + strconv.FormatInt(x, 10), true
	case ondalik:
		if float64(x) == float64(int64(x)) {
			return "s" + strconv.FormatInt(int64(x), 10), true
		}
		return "s" + strconv.FormatFloat(float64(x), 'g', -1, 64), true
	case string:
		return "m" + x, true
	case bool:
		return "b" + strconv.FormatBool(x), true
	}
	return "", false
}

func idHatasi(fn string, t *dbTable, id interface{}) error {
	return fmt.Errorf("%s: id %s %q tablosunda zaten var (her satırın id'si tektir; id verilmezse otomatik verilir)", fn, trDegeri(id).Inspect(), t.Name)
}

// idleriDenetle tablodaki id'lerin tek olduğunu denetler ve otomatik id sayacını en büyük
// Tam id'nin ötesine taşır.
func (t *dbTable) idleriDenetle(fn string) error {
	gorulen := make(map[string]bool, len(t.Rows))
	for _, r := range t.Rows {
		id, var_ := r["id"]
		if !var_ {
			continue
		}
		a, ok := idAnahtari(id)
		if !ok {
			return fmt.Errorf("%s: id Tam, Ondalık, Metin ya da Mantık olmalı, %s verildi", fn, trDegeri(id).Type())
		}
		if gorulen[a] {
			return idHatasi(fn, t, id)
		}
		gorulen[a] = true
	}
	t.idSayaciniDuzelt()
	return nil
}

// idSayaciniDuzelt otomatik id sayacını en az 1 ve en büyük Tam id'den büyük yapar.
func (t *dbTable) idSayaciniDuzelt() {
	if t.NextID < 1 {
		t.NextID = 1
	}
	for _, r := range t.Rows {
		if n, ok := r["id"].(int64); ok && n >= t.NextID && n < 1<<62 {
			t.NextID = n + 1
		}
	}
}

// satirEkle d sözlüğünü tabloya yeni satır olarak ekler; id verilmemişse otomatik id verir.
func (t *dbTable) satirEkle(fn string, d *vm.DictVal, idler map[string]bool) error {
	row := make(map[string]interface{}, len(d.Order)+1)
	for _, k := range d.Order {
		val, ok := d.Pairs[k]
		if !ok {
			continue
		}
		ad := d.DisplayKey(k)
		g, err := satirDegeri(fn, val)
		if err != nil {
			return err
		}
		row[ad] = g
		if !sutunVar(t, ad) {
			t.Columns = append(t.Columns, ad)
		}
	}
	if id, var_ := row["id"]; var_ {
		a, ok := idAnahtari(id)
		if !ok {
			return fmt.Errorf("%s: id Tam, Ondalık, Metin ya da Mantık olmalı, %s verildi", fn, trDegeri(id).Type())
		}
		if idler[a] {
			return idHatasi(fn, t, id)
		}
		idler[a] = true
		if n, ok := id.(int64); ok && n >= t.NextID && n < 1<<62 {
			t.NextID = n + 1
		}
	} else {
		for {
			a, _ := idAnahtari(t.NextID)
			if !idler[a] {
				idler[a] = true
				break
			}
			t.NextID++
		}
		row["id"] = t.NextID
		t.NextID++
		if !sutunVar(t, "id") {
			t.Columns = append([]string{"id"}, t.Columns...)
		}
	}
	t.Rows = append(t.Rows, row)
	return nil
}

func sutunVar(t *dbTable, ad string) bool {
	for _, c := range t.Columns {
		if c == ad {
			return true
		}
	}
	return false
}

// Veritabanı üyeleri (adları sözlükte, pkg/sozluk Y7xx, Y004, Y005).
const (
	uyeTablolar     = sozluk.Y + 700
	uyeTabloOlustur = sozluk.Y + 701
	uyeEkle         = sozluk.Y + 4
	uyeSorgula      = sozluk.Y + 702
	uyeGuncelle     = sozluk.Y + 703
	uyeSil          = sozluk.Y + 704
	uyeHepsiniAl    = sozluk.Y + 705
	uyeSay          = sozluk.Y + 706
	uyeKaydet       = sozluk.Y + 5
)

func (db *DatabaseVal) GetProperty(kod sozluk.Kod, prop string) (vm.Value, error) {
	switch kod {
	case uyeTablolar:
		return &vm.BuiltinVal{
			Name: "vt.tablolar",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				db.mu.RLock()
				defer db.mu.RUnlock()
				var adlar []string
				for _, t := range db.Tables {
					adlar = append(adlar, t.Name)
				}
				sort.Strings(adlar)
				names := []vm.Value{}
				for _, a := range adlar {
					names = append(names, &vm.StringVal{Val: a})
				}
				return &vm.ListVal{Elements: names}, nil
			},
		}, nil

	case uyeTabloOlustur:
		return &vm.BuiltinVal{
			Name: "vt.tablo_oluştur",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("tablo_oluştur(tablo_adı, [sütunlar]) en az 1 argüman bekler")
				}
				db.mu.Lock()
				defer db.mu.Unlock()
				err := db.degistir("vt.tablo_oluştur", args[0].Inspect(), true, func(t *dbTable) error {
					if len(args) >= 2 {
						if lv, ok := args[1].(*vm.ListVal); ok {
							t.Columns = nil
							for _, col := range lv.Elements {
								colName := strings.Split(col.Inspect(), ":")[0]
								t.Columns = append(t.Columns, strings.TrimSpace(colName))
							}
						}
					}
					return nil
				})
				if err != nil {
					return nil, err
				}
				return db, nil
			},
		}, nil

	case uyeEkle:
		return &vm.BuiltinVal{
			Name: "vt.ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				const fn = "vt.ekle"
				if len(args) < 2 {
					return nil, fmt.Errorf("vt.ekle(tablo_adı, sözlük_veya_liste) 2 argüman bekler")
				}
				var sozlukler []*vm.DictVal
				switch data := args[1].(type) {
				case *vm.DictVal:
					sozlukler = append(sozlukler, data)
				case *vm.ListVal:
					for i, el := range data.Elements {
						d, ok := el.(*vm.DictVal)
						if !ok {
							return nil, fmt.Errorf("vt.ekle: listenin %d. elemanı Sözlük olmalı, %s verildi", i+1, vm.TipVeOzet(el))
						}
						sozlukler = append(sozlukler, d)
					}
				default:
					return nil, fmt.Errorf("vt.ekle ikinci argüman olarak Sözlük veya Sözlük Listesi bekler")
				}
				db.mu.Lock()
				defer db.mu.Unlock()
				var satirSayisi int
				err := db.degistir(fn, args[0].Inspect(), true, func(t *dbTable) error {
					idler := make(map[string]bool, len(t.Rows))
					for _, r := range t.Rows {
						if a, ok := idAnahtari(r["id"]); ok {
							idler[a] = true
						}
					}
					for _, d := range sozlukler {
						if err := t.satirEkle(fn, d, idler); err != nil {
							return err
						}
					}
					satirSayisi = len(t.Rows)
					return nil
				})
				if err != nil {
					return nil, err
				}
				return &vm.IntVal{Val: int64(satirSayisi)}, nil
			},
		}, nil

	case uyeSorgula:
		return &vm.BuiltinVal{
			Name: "vt.sorgula",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("vt.sorgula(sql_sorgusu, [parametreler]) en az 1 argüman bekler")
				}
				params, err := paramsArg("vt.sorgula", args, 1, named)
				if err != nil {
					return nil, err
				}
				return db.executeSQL(args[0].Inspect(), params)
			},
		}, nil

	case uyeGuncelle:
		return &vm.BuiltinVal{
			Name: "vt.güncelle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				const fn = "vt.güncelle"
				if len(args) < 2 {
					return nil, fmt.Errorf("vt.güncelle(tablo_adı, yeni_değerler_sözlüğü, [koşul]) en az 2 argüman bekler")
				}
				updates, ok := args[1].(*vm.DictVal)
				if !ok {
					return nil, fmt.Errorf("vt.güncelle ikinci argüman olarak Sözlük bekler")
				}
				cond := ""
				if len(args) >= 3 {
					cond = args[2].Inspect()
				}
				params, err := paramsArg(fn, args, 3, named)
				if err != nil {
					return nil, err
				}
				type alan struct {
					ad    string
					deger vm.Value
				}
				var alanlar []alan
				for _, k := range updates.Order {
					if val, ok := updates.Pairs[k]; ok {
						alanlar = append(alanlar, alan{updates.DisplayKey(k), val})
					}
				}
				db.mu.Lock()
				defer db.mu.Unlock()
				sayi := 0
				err = db.degistir(fn, args[0].Inspect(), false, func(t *dbTable) error {
					uyar, err := eslesmeler(t.Rows, cond, tableColumnSet(t), params)
					if err != nil {
						return err
					}
					for i, row := range t.Rows {
						if !uyar[i] {
							continue
						}
						yeni := make(map[string]interface{}, len(row)+len(alanlar))
						for k, d := range row {
							yeni[k] = d
						}
						for _, a := range alanlar {
							g, err := satirDegeri(fn, a.deger)
							if err != nil {
								return err
							}
							yeni[a.ad] = g
							if !sutunVar(t, a.ad) {
								t.Columns = append(t.Columns, a.ad)
							}
						}
						t.Rows[i] = yeni
						sayi++
					}
					return t.idleriDenetle(fn)
				})
				if err != nil {
					return nil, err
				}
				return &vm.IntVal{Val: int64(sayi)}, nil
			},
		}, nil

	case uyeSil:
		return &vm.BuiltinVal{
			Name: "vt.sil",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("vt.sil(tablo_adı, [koşul]) en az 1 argüman bekler")
				}
				cond := ""
				if len(args) >= 2 {
					cond = args[1].Inspect()
				}
				params, err := paramsArg("vt.sil", args, 2, named)
				if err != nil {
					return nil, err
				}
				db.mu.Lock()
				defer db.mu.Unlock()
				return db.satirSil("vt.sil", args[0].Inspect(), cond, params)
			},
		}, nil

	case uyeHepsiniAl:
		return &vm.BuiltinVal{
			Name: "vt.hepsini_al",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("vt.hepsini_al(tablo_adı) 1 argüman bekler")
				}
				db.mu.RLock()
				defer db.mu.RUnlock()
				t, err := db.tabloBul("vt.hepsini_al", args[0].Inspect())
				if err != nil {
					return nil, err
				}
				return rowsToListVal(t.Rows, nil), nil
			},
		}, nil

	case uyeSay:
		return &vm.BuiltinVal{
			Name: "vt.say",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("vt.say(tablo_adı, [koşul]) en az 1 argüman bekler")
				}
				db.mu.RLock()
				defer db.mu.RUnlock()
				t, err := db.tabloBul("vt.say", args[0].Inspect())
				if err != nil {
					return nil, err
				}
				if len(args) < 2 {
					return &vm.IntVal{Val: int64(len(t.Rows))}, nil
				}
				params, err := paramsArg("vt.say", args, 2, named)
				if err != nil {
					return nil, err
				}
				matched, _, err := filterRows(t.Rows, args[1].Inspect(), tableColumnSet(t), params)
				if err != nil {
					return nil, err
				}
				return &vm.IntVal{Val: int64(len(matched))}, nil
			},
		}, nil

	case uyeKaydet:
		return &vm.BuiltinVal{
			Name: "vt.kaydet",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				db.mu.Lock()
				defer db.mu.Unlock()
				if err := db.saveToDisk(); err != nil {
					return nil, err
				}
				return vm.TRUE, nil
			},
		}, nil
	}

	return nil, fmt.Errorf("Veritabanı üzerinde %q özelliği veya metodu bulunamadı", prop)
}

// satirSil koşula uyan satırları siler (vt.sil ve SİL sorgusu). Çağıran db.mu'yu yazma
// için tutar.
func (db *DatabaseVal) satirSil(fn, tablo, cond string, params []vm.Value) (vm.Value, error) {
	silinen := 0
	err := db.degistir(fn, tablo, false, func(t *dbTable) error {
		deleted, kept, err := filterRows(t.Rows, cond, tableColumnSet(t), params)
		if err != nil {
			return err
		}
		silinen = len(deleted)
		t.Rows = append([]map[string]interface{}{}, kept...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &vm.IntVal{Val: int64(silinen)}, nil
}

func foldUpperASCII(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case 'ı', 'I', 'i', 'İ':
			sb.WriteByte('I')
		case 'ş', 'Ş':
			sb.WriteByte('S')
		case 'ğ', 'Ğ':
			sb.WriteByte('G')
		case 'ü', 'Ü':
			sb.WriteByte('U')
		case 'ö', 'Ö':
			sb.WriteByte('O')
		case 'ç', 'Ç':
			sb.WriteByte('C')
		default:
			if r >= 'a' && r <= 'z' {
				sb.WriteByte(byte(r - 32))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
}

type sqlWordSpan struct {
	word  string
	fold  string
	start int
	end   int
}

// scanSQLWordsOutsideQuotes sorgu metnindeki tırnak dışı sözcükleri verir. Tırnaklar koşul
// ayrıştırıcısıyla (tokenizeCondition) aynı kuralla okunur: tırnak içinde '\' sonraki
// karakteri kaçırır ('a\\' tek ters bölüdür, 'a\” tırnaktır), aynı tırnağın ikilenmesi
// ('O”Brien') tırnağın kendisidir. Böylece sözcük tarayıcı ile koşul ayrıştırıcı bir
// tırnağın nerede bittiğinde hiç ayrışmaz.
func scanSQLWordsOutsideQuotes(s string) []sqlWordSpan {
	var spans []sqlWordSpan
	inQuote := byte(0)
	n := len(s)
	i := 0
	for i < n {
		ch := s[i]
		if inQuote != 0 {
			switch {
			case ch == '\\' && i+1 < n:
				i += 2
			case ch == inQuote && i+1 < n && s[i+1] == inQuote:
				i += 2
			case ch == inQuote:
				inQuote = 0
				i++
			default:
				i++
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			inQuote = ch
			i++
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}
		start := i
		for i < n {
			c := s[i]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\'' || c == '"' {
				break
			}
			i++
		}
		w := s[start:i]
		spans = append(spans, sqlWordSpan{
			word:  w,
			fold:  foldUpperASCII(w),
			start: start,
			end:   i,
		})
	}
	return spans
}

// findSQLKeyword tırnak dışındaki ilk Türkçe sorgu sözcüğünün (sozcukler.go) yerini verir.
func findSQLKeyword(s string, keyword string) (int, int) {
	for _, sp := range scanSQLWordsOutsideQuotes(s) {
		if sqlSozcuguMu(sp.word, keyword) {
			return sp.start, sp.end
		}
	}
	return -1, -1
}

// executeSQL Türkçe sorguları ("SEÇ * TABLO personel NEREDE maas > 50000 SIRALA maas AZALAN
// LİMİT 5", "SİL TABLO personel NEREDE id = 3") çalıştırır. Sözcükler yalnızca Türkçedir;
// İngilizce ya da ASCII yazım (SELECT, FROM, SEC ...) Türkçe karşılığı öneren bir hata verir.
// Koşuldaki '?' yer tutucuları params listesindeki değerlere güvenli şekilde bağlanır.
func (db *DatabaseVal) executeSQL(rawQuery string, params []vm.Value) (vm.Value, error) {
	q := strings.TrimSpace(rawQuery)
	tokens := strings.Fields(q)
	if len(tokens) == 0 {
		return &vm.ListVal{Elements: []vm.Value{}}, nil
	}

	switch sqlBuyuk(tokens[0]) {
	case sqlSec:
		return db.executeSelectSQL(q, params)
	case sqlSil:
		return db.executeDeleteSQL(q, params)
	}
	if t, ok := sqlOneri(tokens[0]); ok {
		return nil, fmt.Errorf("Desteklenmeyen SQL komutu: %s", sqlIpucu(tokens[0], t))
	}
	return nil, fmt.Errorf("Desteklenmeyen SQL komutu: %q (SEÇ ya da SİL kullanın)", tokens[0])
}

func (db *DatabaseVal) executeSelectSQL(q string, params []vm.Value) (vm.Value, error) {
	// 1. TABLO bul
	fromStart, fromEnd := findSQLKeyword(q, sqlTablo)
	if fromStart == -1 {
		if err := sqlOneriHatasi("SQL hatası", q); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("SQL hatası: 'TABLO' anahtar kelimesi bulunamadı (%s)", kosulOzeti(q))
	}

	firstSpace := strings.IndexAny(q, " \t\n\r")
	if firstSpace == -1 || firstSpace > fromStart {
		firstSpace = 0
	}
	selectPart := strings.TrimSpace(q[firstSpace:fromStart])
	var selectedCols []string
	if selectPart != "*" && selectPart != "" {
		for _, c := range strings.Split(selectPart, ",") {
			selectedCols = append(selectedCols, strings.TrimSpace(c))
		}
	}

	afterFrom := strings.TrimSpace(q[fromEnd:])

	// LİMİT
	limit := -1
	if limStart, limEnd := findSQLKeyword(afterFrom, sqlLimit); limStart != -1 {
		limStr := strings.TrimSpace(afterFrom[limEnd:])
		n, err := strconv.Atoi(limStr)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("SQL hatası: LİMİT'ten sonra sıfır ya da pozitif bir tam sayı gelmeli, %q yazılmış", kosulOzeti(limStr))
		}
		limit = n
		afterFrom = strings.TrimSpace(afterFrom[:limStart])
	}

	// SIRALA
	orderByCol := ""
	orderDesc := false
	if ordStart, ordEnd := findSQLKeyword(afterFrom, sqlSirala); ordStart != -1 {
		ordPart := strings.TrimSpace(afterFrom[ordEnd:])
		ordFields := strings.Fields(ordPart)
		if len(ordFields) >= 1 {
			orderByCol = ordFields[0]
		}
		if len(ordFields) >= 2 {
			if sqlSozcuguMu(ordFields[1], sqlAzalan) {
				orderDesc = true
			} else if err := sqlOneriHatasi("SQL hatası", ordFields[1]); err != nil {
				return nil, err
			}
		}
		afterFrom = strings.TrimSpace(afterFrom[:ordStart])
	}

	// NEREDE
	whereCond := ""
	tableName := afterFrom
	if whereStart, whereEnd := findSQLKeyword(afterFrom, sqlNerede); whereStart != -1 {
		tableName = strings.TrimSpace(afterFrom[:whereStart])
		whereCond = strings.TrimSpace(afterFrom[whereEnd:])
	}
	// Tablo adının yanında kalan İngilizce ya da ASCII sözcük (WHERE, ORDER, LIMIT ...)
	// sessizce tablo adının parçası sayılmaz.
	if err := sqlOneriHatasi("SQL hatası", tableName); err != nil {
		return nil, err
	}

	db.mu.RLock()
	defer db.mu.RUnlock()
	t, err := db.tabloBul("SQL hatası", tableName)
	if err != nil {
		return nil, err
	}

	found, _, err := filterRows(t.Rows, whereCond, tableColumnSet(t), params)
	if err != nil {
		return nil, err
	}
	// Sıralama tablonun kendi satır dizisini bozmasın diye kopya üzerinde yapılır
	matched := append([]map[string]interface{}(nil), found...)

	if orderByCol != "" {
		sortRowsByColumn(matched, orderByCol, orderDesc)
	}

	if limit >= 0 && limit < len(matched) {
		matched = matched[:limit]
	}

	return rowsToListVal(matched, selectedCols), nil
}

func (db *DatabaseVal) executeDeleteSQL(q string, params []vm.Value) (vm.Value, error) {
	fromStart, fromEnd := findSQLKeyword(q, sqlTablo)
	if fromStart == -1 {
		if err := sqlOneriHatasi("SQL SİL hatası", q); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("SQL SİL hatası: 'TABLO' belirtilmeli")
	}
	afterFrom := strings.TrimSpace(q[fromEnd:])
	tableName := afterFrom
	whereCond := ""
	if whereStart, whereEnd := findSQLKeyword(afterFrom, sqlNerede); whereStart != -1 {
		tableName = strings.TrimSpace(afterFrom[:whereStart])
		whereCond = strings.TrimSpace(afterFrom[whereEnd:])
	}
	if err := sqlOneriHatasi("SQL SİL hatası", tableName); err != nil {
		return nil, err
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	return db.satirSil("SQL SİL hatası", tableName, whereCond, params)
}

func sortRowsByColumn(rows []map[string]interface{}, col string, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		vi := trDegeri(rows[i][col])
		vj := trDegeri(rows[j][col])
		fi, ok1 := vm.ToFloat(vi)
		fj, ok2 := vm.ToFloat(vj)
		if ok1 && ok2 {
			if desc {
				return fi > fj
			}
			return fi < fj
		}
		si := vi.Inspect()
		sj := vj.Inspect()
		if desc {
			return si > sj
		}
		return si < sj
	})
}

func rowsToListVal(rows []map[string]interface{}, cols []string) *vm.ListVal {
	elements := []vm.Value{}
	for _, r := range rows {
		d := vm.NewDictVal()
		if len(cols) > 0 {
			for _, c := range cols {
				if val, exists := r[c]; exists {
					d.Set(c, trDegeri(val))
				}
			}
		} else {
			// Tutarlı anahtar sırası için sırala (id en başta)
			var keys []string
			for k := range r {
				if k != "id" {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			if val, hasID := r["id"]; hasID {
				d.Set("id", trDegeri(val))
			}
			for _, k := range keys {
				d.Set(k, trDegeri(r[k]))
			}
		}
		elements = append(elements, d)
	}
	return &vm.ListVal{Elements: elements}
}

// Kaydet veritabanı yerleşiklerini VM'ye kaydeder.
func Kaydet(v *vm.VM) {
	// 1. veritabanı_aç("dosya.trdb" veya ":bellek:")
	openDB := func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		path := bellekYolu
		if len(args) >= 1 {
			path = args[0].Inspect()
		}
		if path == ":memory:" {
			return nil, fmt.Errorf("veritabanı_aç: %s", sqlIpucu(path, bellekYolu))
		}
		db, err := veritabaniAc(v, path)
		if err != nil {
			return nil, err
		}
		return db, nil
	}
	v.RegisterBuiltinKod(sozluk.W+200, openDB)

	// 2. Bellekteki listeyi tablo gibi sorgulama:
	// tablo_sorgula(siparisler, "tutar > 20000", sırala: "tutar:azalan", alanlar: ["musteri", "tutar"], limit: 5)
	v.RegisterBuiltinKod(sozluk.W+210, func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		const fn = "tablo_sorgula"
		if len(args) < 1 {
			return nil, fmt.Errorf("tablo_sorgula(liste, [koşul], [sırala], [alanlar], [limit]) en az 1 argüman bekler")
		}
		lv, ok := args[0].(*vm.ListVal)
		if !ok {
			return nil, fmt.Errorf("tablo_sorgula ilk argüman olarak sözlüklerden oluşan bir Liste bekler")
		}
		cond := ""
		if len(args) >= 2 {
			cond = args[1].Inspect()
		}
		if c, ok := baglama.AdliAl(named, adKosul); ok {
			cond = c.Inspect()
		}

		params, err := paramsArg(fn, args, 2, named, adKosul, adSirala, adLimit, adAlanlar)
		if err != nil {
			return nil, err
		}

		limit := -1
		if limVal, ok := baglama.AdliAl(named, adLimit); ok {
			iv, ok := limVal.(*vm.IntVal)
			if !ok || iv.Val < 0 {
				return nil, fmt.Errorf("tablo_sorgula: %s sıfır ya da pozitif bir Tam olmalı, %s verildi", adLimit.TR, vm.TipVeOzet(limVal))
			}
			if iv.Val < int64(len(lv.Elements)) {
				limit = int(iv.Val)
			}
		}

		var all []map[string]interface{}
		known := map[string]bool{}
		for _, el := range lv.Elements {
			if d, ok := el.(*vm.DictVal); ok {
				row := make(map[string]interface{})
				for _, k := range d.Order {
					val, ok := d.Pairs[k]
					if !ok {
						continue
					}
					ad := d.DisplayKey(k)
					g, err := bellekSatirDegeri(fn, val)
					if err != nil {
						return nil, err
					}
					row[ad] = g
					known[ad] = true
				}
				all = append(all, row)
			}
		}
		rows, _, err := filterRows(all, cond, known, params)
		if err != nil {
			return nil, err
		}

		// Sıralama: sırala: "tutar:azalan" veya "tutar"
		for _, sortKey := range adSirala.Yazimlar() {
			if sv, ok := named[sortKey]; ok {
				parts := strings.Split(sv.Inspect(), ":")
				col := strings.TrimSpace(parts[0])
				desc := false
				if len(parts) > 1 {
					dir := strings.TrimSpace(parts[1])
					if sqlSozcuguMu(dir, sqlAzalan) {
						desc = true
					} else if err := sqlOneriHatasi(fn, dir); err != nil {
						return nil, err
					}
				}
				sortRowsByColumn(rows, col, desc)
			}
		}

		if limit >= 0 && limit < len(rows) {
			rows = rows[:limit]
		}

		// Alanlar (Projection)
		var cols []string
		if fVal, ok := baglama.AdliAl(named, adAlanlar); ok {
			if fl, ok := fVal.(*vm.ListVal); ok {
				for _, item := range fl.Elements {
					cols = append(cols, item.Inspect())
				}
			}
		}

		return rowsToListVal(rows, cols), nil
	})
}
