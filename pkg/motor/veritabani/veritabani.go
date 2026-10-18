package veritabani

import (
	"encoding/json"
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
	Tables   map[string]*dbTable `json:"tables"`
	mu       sync.RWMutex
	vmRef    *vm.VM
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

func (db *DatabaseVal) saveToDisk() error {
	if db.FilePath == "" || db.FilePath == ":bellek:" || db.FilePath == ":memory:" {
		return nil
	}
	data, err := json.MarshalIndent(db.Tables, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(db.yol(), data, 0644); err != nil {
		return turkce.DosyaHatasi("veritabanı", db.FilePath, "kaydedilemedi", err)
	}
	return nil
}

// yol veritabanı dosyasının diskteki yoludur.
func (db *DatabaseVal) yol() string {
	if db.diskYolu != "" {
		return db.diskYolu
	}
	return db.FilePath
}

func (db *DatabaseVal) loadFromDisk() error {
	if db.FilePath == "" || db.FilePath == ":bellek:" || db.FilePath == ":memory:" {
		return nil
	}
	data, err := os.ReadFile(db.yol())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return turkce.DosyaHatasi("veritabanı_aç", db.FilePath, "yüklenemedi", err)
	}
	if err := json.Unmarshal(data, &db.Tables); err != nil {
		return fmt.Errorf("veritabanı_aç: %q yüklenemedi: dosya geçerli bir TR veritabanı (JSON) değil", db.FilePath)
	}
	// Dosya "null" içeriyorsa veya tablolar eksik alanlarla geldiyse nil map/slice paniklerini önle
	if db.Tables == nil {
		db.Tables = make(map[string]*dbTable)
	}
	for k, t := range db.Tables {
		if t == nil {
			delete(db.Tables, k)
			continue
		}
		if t.Name == "" {
			t.Name = k
		}
		var rows []map[string]interface{}
		for _, r := range t.Rows {
			if r != nil {
				rows = append(rows, r)
			}
		}
		t.Rows = rows
		if t.NextID < 1 {
			t.NextID = 1
		}
	}
	return nil
}

func (db *DatabaseVal) ensureTable(name string) *dbTable {
	key := strings.ToLower(name)
	t, ok := db.Tables[key]
	if !ok {
		t = &dbTable{
			Name:    name,
			Columns: []string{},
			Rows:    []map[string]interface{}{},
			NextID:  1,
		}
		db.Tables[key] = t
	}
	return t
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
				var names []vm.Value
				for _, t := range db.Tables {
					names = append(names, &vm.StringVal{Val: t.Name})
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
				t := db.ensureTable(args[0].Inspect())
				if len(args) >= 2 {
					if lv, ok := args[1].(*vm.ListVal); ok {
						t.Columns = nil
						for _, col := range lv.Elements {
							colName := strings.Split(col.Inspect(), ":")[0]
							t.Columns = append(t.Columns, strings.TrimSpace(colName))
						}
					}
				}
				_ = db.saveToDisk()
				return db, nil
			},
		}, nil

	case uyeEkle:
		return &vm.BuiltinVal{
			Name: "vt.ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("vt.ekle(tablo_adı, sözlük_veya_liste) 2 argüman bekler")
				}
				db.mu.Lock()
				defer db.mu.Unlock()
				t := db.ensureTable(args[0].Inspect())

				insertDict := func(d *vm.DictVal) {
					row := make(map[string]interface{})
					if _, hasID := d.Pairs["id"]; !hasID {
						row["id"] = t.NextID
					}
					t.NextID++
					for k, val := range d.Pairs {
						row[k] = vm.TRToGoValue(val)
						hasCol := false
						for _, c := range t.Columns {
							if c == k {
								hasCol = true
								break
							}
						}
						if !hasCol {
							t.Columns = append(t.Columns, k)
						}
					}
					t.Rows = append(t.Rows, row)
				}

				switch data := args[1].(type) {
				case *vm.DictVal:
					insertDict(data)
				case *vm.ListVal:
					for _, el := range data.Elements {
						if d, ok := el.(*vm.DictVal); ok {
							insertDict(d)
						}
					}
				default:
					return nil, fmt.Errorf("vt.ekle ikinci argüman olarak Sözlük veya Sözlük Listesi bekler")
				}

				_ = db.saveToDisk()
				return &vm.IntVal{Val: int64(len(t.Rows))}, nil
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
				if len(args) < 2 {
					return nil, fmt.Errorf("vt.güncelle(tablo_adı, yeni_değerler_sözlüğü, [koşul]) en az 2 argüman bekler")
				}
				tableName := strings.ToLower(args[0].Inspect())
				updates, ok := args[1].(*vm.DictVal)
				if !ok {
					return nil, fmt.Errorf("vt.güncelle ikinci argüman olarak Sözlük bekler")
				}
				cond := ""
				if len(args) >= 3 {
					cond = args[2].Inspect()
				}
				params, err := paramsArg("vt.güncelle", args, 3, named)
				if err != nil {
					return nil, err
				}
				db.mu.Lock()
				defer db.mu.Unlock()
				t, exists := db.Tables[tableName]
				if !exists {
					return &vm.IntVal{Val: 0}, nil
				}
				matched, _, err := filterRows(t.Rows, cond, tableColumnSet(t), params)
				if err != nil {
					return nil, err
				}
				for _, row := range matched {
					for k, val := range updates.Pairs {
						row[k] = vm.TRToGoValue(val)
					}
				}
				_ = db.saveToDisk()
				return &vm.IntVal{Val: int64(len(matched))}, nil
			},
		}, nil

	case uyeSil:
		return &vm.BuiltinVal{
			Name: "vt.sil",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("vt.sil(tablo_adı, [koşul]) en az 1 argüman bekler")
				}
				tableName := strings.ToLower(args[0].Inspect())
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
				t, exists := db.Tables[tableName]
				if !exists {
					return &vm.IntVal{Val: 0}, nil
				}
				deleted, kept, err := filterRows(t.Rows, cond, tableColumnSet(t), params)
				if err != nil {
					return nil, err
				}
				t.Rows = kept
				_ = db.saveToDisk()
				return &vm.IntVal{Val: int64(len(deleted))}, nil
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
				t, exists := db.Tables[strings.ToLower(args[0].Inspect())]
				if !exists {
					return &vm.ListVal{Elements: []vm.Value{}}, nil
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
				t, exists := db.Tables[strings.ToLower(args[0].Inspect())]
				if !exists {
					return &vm.IntVal{Val: 0}, nil
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

func scanSQLWordsOutsideQuotes(s string) []sqlWordSpan {
	var spans []sqlWordSpan
	inQuote := byte(0)
	n := len(s)
	i := 0
	for i < n {
		ch := s[i]
		if (ch == '\'' || ch == '"') && (i == 0 || s[i-1] != '\\') {
			if inQuote == 0 {
				inQuote = ch
			} else if inQuote == ch {
				inQuote = 0
			}
			i++
			continue
		}
		if inQuote != 0 {
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

func findSQLKeyword(s string, keywords ...string) (int, int) {
	spans := scanSQLWordsOutsideQuotes(s)
	for idx := 0; idx < len(spans); idx++ {
		for _, kw := range keywords {
			kwParts := strings.Fields(foldUpperASCII(kw))
			if len(kwParts) == 1 && spans[idx].fold == kwParts[0] {
				return spans[idx].start, spans[idx].end
			}
			if len(kwParts) == 2 && idx+1 < len(spans) && spans[idx].fold == kwParts[0] && spans[idx+1].fold == kwParts[1] {
				return spans[idx].start, spans[idx+1].end
			}
		}
	}
	return -1, -1
}

// executeSQL Türkçe SQL ("SEÇ * TABLO personel NEREDE maas > 50000 SIRALA maas AZALAN LİMİT 5")
// ve standart SQL ("SELECT * FROM personel WHERE maas > 50000 ORDER BY maas DESC LIMIT 5") sorgularını çalıştırır.
// Koşuldaki '?' yer tutucuları params listesindeki değerlere güvenli şekilde bağlanır.
func (db *DatabaseVal) executeSQL(rawQuery string, params []vm.Value) (vm.Value, error) {
	q := strings.TrimSpace(rawQuery)
	tokens := strings.Fields(q)
	if len(tokens) == 0 {
		return &vm.ListVal{Elements: []vm.Value{}}, nil
	}

	firstFold := foldUpperASCII(tokens[0])
	switch firstFold {
	case "SEC", "SELECT":
		return db.executeSelectSQL(q, params)
	case "SIL", "DELETE":
		return db.executeDeleteSQL(q, params)
	default:
		return nil, fmt.Errorf("Desteklenmeyen SQL komutu: %q (SEÇ / SELECT veya SİL / DELETE kullanın)", tokens[0])
	}
}

func (db *DatabaseVal) executeSelectSQL(q string, params []vm.Value) (vm.Value, error) {
	// 1. FROM / TABLO / DAN bul
	fromStart, fromEnd := findSQLKeyword(q, "TABLO", "DAN", "FROM")
	if fromStart == -1 {
		return nil, fmt.Errorf("SQL hatası: 'TABLO' veya 'FROM' anahtar kelimesi bulunamadı (%s)", q)
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

	// LİMİT / LIMIT
	limit := -1
	if limStart, limEnd := findSQLKeyword(afterFrom, "LIMIT"); limStart != -1 {
		limStr := strings.TrimSpace(afterFrom[limEnd:])
		if n, err := strconv.Atoi(limStr); err == nil {
			limit = n
		}
		afterFrom = strings.TrimSpace(afterFrom[:limStart])
	}

	// SIRALA / ORDER BY
	orderByCol := ""
	orderDesc := false
	if ordStart, ordEnd := findSQLKeyword(afterFrom, "SIRALA", "ORDER BY"); ordStart != -1 {
		ordPart := strings.TrimSpace(afterFrom[ordEnd:])
		ordFields := strings.Fields(ordPart)
		if len(ordFields) >= 1 {
			orderByCol = ordFields[0]
		}
		if len(ordFields) >= 2 {
			dir := foldUpperASCII(ordFields[1])
			if dir == "AZALAN" || dir == "DESC" {
				orderDesc = true
			}
		}
		afterFrom = strings.TrimSpace(afterFrom[:ordStart])
	}

	// NEREDE / WHERE
	whereCond := ""
	tableName := afterFrom
	if whereStart, whereEnd := findSQLKeyword(afterFrom, "NEREDE", "WHERE"); whereStart != -1 {
		tableName = strings.TrimSpace(afterFrom[:whereStart])
		whereCond = strings.TrimSpace(afterFrom[whereEnd:])
	}

	db.mu.RLock()
	defer db.mu.RUnlock()
	t, exists := db.Tables[strings.ToLower(tableName)]
	if !exists {
		return &vm.ListVal{Elements: []vm.Value{}}, nil
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
	fromStart, fromEnd := findSQLKeyword(q, "TABLO", "DAN", "FROM")
	if fromStart == -1 {
		return nil, fmt.Errorf("SQL SİL hatası: 'TABLO' veya 'FROM' belirtilmeli")
	}
	afterFrom := strings.TrimSpace(q[fromEnd:])
	tableName := afterFrom
	whereCond := ""
	if whereStart, whereEnd := findSQLKeyword(afterFrom, "NEREDE", "WHERE"); whereStart != -1 {
		tableName = strings.TrimSpace(afterFrom[:whereStart])
		whereCond = strings.TrimSpace(afterFrom[whereEnd:])
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	t, exists := db.Tables[strings.ToLower(tableName)]
	if !exists {
		return &vm.IntVal{Val: 0}, nil
	}
	deleted, kept, err := filterRows(t.Rows, whereCond, tableColumnSet(t), params)
	if err != nil {
		return nil, err
	}
	t.Rows = kept
	_ = db.saveToDisk()
	return &vm.IntVal{Val: int64(len(deleted))}, nil
}

func sortRowsByColumn(rows []map[string]interface{}, col string, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		vi := vm.GoToTRValue(rows[i][col])
		vj := vm.GoToTRValue(rows[j][col])
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
	var elements []vm.Value
	for _, r := range rows {
		d := vm.NewDictVal()
		if len(cols) > 0 {
			for _, c := range cols {
				if val, exists := r[c]; exists {
					d.Set(c, vm.GoToTRValue(val))
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
				d.Set("id", vm.GoToTRValue(val))
			}
			for _, k := range keys {
				d.Set(k, vm.GoToTRValue(r[k]))
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
		path := ":bellek:"
		if len(args) >= 1 {
			path = args[0].Inspect()
		}
		db := &DatabaseVal{
			FilePath: path,
			Tables:   make(map[string]*dbTable),
			vmRef:    v,
		}
		if path != ":bellek:" && path != ":memory:" {
			db.diskYolu, _ = v.YolCoz(path)
		}
		if err := db.loadFromDisk(); err != nil {
			return nil, err
		}
		return db, nil
	}
	v.RegisterBuiltinKod(sozluk.W+200, openDB)

	// 2. Bellekteki listeyi tablo gibi sorgulama:
	// tablo_sorgula(siparisler, "tutar > 20000", sırala: "tutar:azalan", alanlar: ["musteri", "tutar"], limit: 5)
	v.RegisterBuiltinKod(sozluk.W+210, func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

		params, err := paramsArg("tablo_sorgula", args, 2, named, adKosul, adSirala, adLimit, adAlanlar)
		if err != nil {
			return nil, err
		}

		var all []map[string]interface{}
		known := map[string]bool{}
		for _, el := range lv.Elements {
			if d, ok := el.(*vm.DictVal); ok {
				row := make(map[string]interface{})
				for k, val := range d.Pairs {
					row[k] = vm.TRToGoValue(val)
					known[k] = true
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
					dir := strings.ToUpper(strings.TrimSpace(parts[1]))
					if dir == "AZALAN" || dir == "DESC" {
						desc = true
					}
				}
				sortRowsByColumn(rows, col, desc)
			}
		}

		// Limit
		if limVal, ok := baglama.AdliAl(named, adLimit); ok {
			if iv, ok := limVal.(*vm.IntVal); ok && int(iv.Val) < len(rows) {
				rows = rows[:int(iv.Val)]
			}
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
