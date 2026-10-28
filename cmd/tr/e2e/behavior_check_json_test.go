package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// 'tr check --json' (dil tanımı 2.13): çalışmadan önceki hatalar ve uyarılar, kullanım hataları dahil,
// standart çıktıya JSON Lines olarak yazılır. Programlar dilin sonraki değişikliklerinden bağımsız,
// eski ve yeni yazımda aynı anlama gelen yazımdadır (değişken bildirimi, çağrı, kimlik yazımı).

// jsonPos kaydın bir konumudur.
type jsonPos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type jsonRelatedRecord struct {
	File    string   `json:"file"`
	Start   *jsonPos `json:"start"`
	End     *jsonPos `json:"end"`
	Message string   `json:"message"`
}

type jsonFixRecord struct {
	Start       jsonPos `json:"start"`
	End         jsonPos `json:"end"`
	Replacement string  `json:"replacement"`
	Applicable  bool    `json:"applicable"`
}

// jsonRecordOut tr'nin yazdığı bir kayıttır; alanlar 2.13'ün sözleşmesidir.
type jsonRecordOut struct {
	Code     string              `json:"code"`
	Severity string              `json:"severity"`
	File     string              `json:"file"`
	Start    *jsonPos            `json:"start"`
	End      *jsonPos            `json:"end"`
	Message  string              `json:"message"`
	Related  []jsonRelatedRecord `json:"related"`
	Fixes    []jsonFixRecord     `json:"fixes"`
}

// checkRecords standart çıktının her satırını bir kayıt olarak çözer; satır JSON nesnesi değilse ya da
// bilinmeyen alan taşıyorsa testi düşürür. Ham satırlar da döner (alanların varlığı için).
func checkRecords(t *testing.T, stdout string) (records []jsonRecordOut, raw []map[string]json.RawMessage) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		var r jsonRecordOut
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&r); err != nil {
			t.Fatalf("satır bir kayıt değil (%v): %q\nçıktının tamamı:\n%s", err, line, stdout)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatalf("satır bir JSON nesnesi değil: %q", line)
		}
		records, raw = append(records, r), append(raw, fields)
	}
	return records, raw
}

// textBetween kaynakta start ile end arasındaki metindir (satır ve sütun 1'den başlar, sütun Unicode
// karakteridir, end dışlayıcıdır).
func textBetween(t *testing.T, source string, start, end jsonPos) string {
	t.Helper()
	lines := strings.Split(source, "\n")
	var out []string
	for line := start.Line; line <= end.Line; line++ {
		if line > len(lines) {
			t.Fatalf("satır %d dosyada yok (%d satır)", line, len(lines))
		}
		text := []rune(lines[line-1])
		from, to := 1, len(text)+1
		if line == start.Line {
			from = start.Column
		}
		if line == end.Line {
			to = end.Column
		}
		if from < 1 || to > len(text)+1 || from > to {
			t.Fatalf("konum satır %d dışında: %d-%d, satır %d karakter: %q", line, from, to, len(text), lines[line-1])
		}
		out = append(out, string(text[from-1:to-1]))
	}
	return strings.Join(out, "\n")
}

// applyFix düzeltmeyi kaynağa uygular.
func applyFix(t *testing.T, source string, f jsonFixRecord) string {
	t.Helper()
	lines := strings.Split(source, "\n")
	offset := func(p jsonPos) int {
		n := 0
		for _, l := range lines[:p.Line-1] {
			n += len(l) + 1
		}
		return n + len(string([]rune(lines[p.Line-1])[:p.Column-1]))
	}
	return source[:offset(f.Start)] + f.Replacement + source[offset(f.End):]
}

// checkJSON 'tr check --json dosya' komutunu verilen dilde çalıştırır.
func checkJSON(t *testing.T, dir, lang, file string) result {
	t.Helper()
	return testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=" + lang}, "check", "--json", file)
}

const (
	// typeStageProgram bir uyarı (ad yazılışı), bir tip hatası ve bir tanımsız ad taşır; sözdizimi doğrudur.
	typeStageProgram = "# TR 4\nTR_var MyVar = 1\nTR_var n: TR_Int = \"x\"\nTR_print(prnt)\nTR_print(MyVar)\n"
	// syntaxStageProgram iki sözdizimi hatası taşır: büyük harfli sayı ön eki ve satır başında C yorumu.
	syntaxStageProgram = "# TR 4\nTR_var sayı = 0X1F\n// yorum\nTR_print(sayı)\n"
)

// Her kayıt 2.13'ün alanlarını taşır: kod, tür, dosya, başlangıç ve bitiş (dışlayıcı), ileti; konum
// kaynakta tam o yeri gösterir; çıkış kodu metin kipindeki gibi hata varsa 1'dir; standart hata boştur.
func TestCheckJSONRecordsHaveTheirFieldsAndPlaces(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "prog.tr", typeStageProgram)
	r := checkJSON(t, dir, "en", "prog.tr")
	if r.Code != 1 || r.Stderr != "" {
		t.Fatalf("çıkış %d (1 bekleniyordu), standart hata %q", r.Code, r.Stderr)
	}
	records, raw := checkRecords(t, r.Stdout)
	want := []struct {
		code, severity, text string
		line                 int
	}{
		{"H1012", "warning", "MyVar", 2},
		{"H1001", "error", `TR_var n: TR_Int = "x"`, 3},
		{"H1106", "error", "prnt", 4},
	}
	if len(records) != len(want) {
		t.Fatalf("%d kayıt bekleniyordu: %s", len(want), r.Stdout)
	}
	for i, w := range want {
		rec := records[i]
		if rec.Code != w.code || rec.Severity != w.severity || rec.File != "prog.tr" || rec.Message == "" {
			t.Errorf("kayıt %d: %+v", i, rec)
		}
		for _, field := range []string{"code", "severity", "file", "start", "end", "message"} {
			if _, ok := raw[i][field]; !ok {
				t.Errorf("kayıt %d: %q alanı yok: %v", i, field, raw[i])
			}
		}
		if rec.Start == nil || rec.End == nil || rec.Start.Line != w.line {
			t.Errorf("kayıt %d: konum %+v-%+v, satır %d bekleniyordu", i, rec.Start, rec.End, w.line)
			continue
		}
		if got := textBetween(t, typeStageProgram, *rec.Start, *rec.End); got != w.text {
			t.Errorf("kayıt %d: konum %q metnini gösteriyor, %q bekleniyordu (end dışlayıcıdır)", i, got, w.text)
		}
	}
	// Alanı olmayan kayıt alanı yazmaz (boş dizi ya da null değil): bu kayıtlar ilgili konum taşımaz.
	if _, ok := raw[1]["related"]; ok {
		t.Errorf("ilgili konumu olmayan kayıt related yazıyor: %v", raw[1])
	}
}

// Yalnız uyarı varsa çıkış 0'dır; temiz dosyada çıktı boştur; metin kipi JSON yazmaz.
func TestCheckJSONExitCodesAndEmptyOutput(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "warn.tr", "# TR 4\nTR_var MyVar = 1\nTR_print(MyVar)\n")
	writeTR(t, dir, "clean.tr", "# TR 4\nTR_var yaş = 1\nTR_print(yaş)\n")
	warn := checkJSON(t, dir, "en", "warn.tr")
	if records, _ := checkRecords(t, warn.Stdout); warn.Code != 0 || len(records) != 1 || records[0].Severity != "warning" {
		t.Errorf("yalnız uyarı: çıkış %d, kayıtlar %+v", warn.Code, records)
	}
	clean := checkJSON(t, dir, "en", "clean.tr")
	if clean.Code != 0 || clean.Stdout != "" || clean.Stderr != "" {
		t.Errorf("temiz dosya: çıkış %d, çıktı %q, hata %q", clean.Code, clean.Stdout, clean.Stderr)
	}
	// Seçenek yoksa metin kipi: standart çıktı JSON değildir, hatalar standart hatadadır.
	writeTR(t, dir, "prog.tr", typeStageProgram)
	text := tr(t, dir, "check", "prog.tr")
	if text.Code != 1 || strings.Contains(text.Stdout, `"severity"`) || text.Stderr == "" {
		t.Errorf("metin kipi: çıkış %d, çıktı %q", text.Code, text.Stdout)
	}
}

// Sütun Unicode karakteridir: öncesinde çok baytlı karakterler olan hata doğru sütunu verir.
func TestCheckJSONColumnsCountCharacters(t *testing.T) {
	dir := t.TempDir()
	program := "# TR 4\nTR_print(\"çğış\", prnt)\n"
	writeTR(t, dir, "unicode.tr", program)
	records, _ := checkRecords(t, checkJSON(t, dir, "tr", "unicode.tr").Stdout)
	if len(records) != 1 || records[0].Start == nil || records[0].End == nil {
		t.Fatalf("kayıtlar %+v", records)
	}
	if got := textBetween(t, program, *records[0].Start, *records[0].End); got != "prnt" {
		t.Errorf("konum %q metnini gösteriyor, %q bekleniyordu (%+v)", got, "prnt", records[0].Start)
	}
}

// Kod, konum ve düzeltme dilden bağımsızdır; yalnız ileti okuyanın dilindedir (TR_DIL).
func TestCheckJSONIsTheSameInBothLanguagesExceptTheMessage(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "type.tr", typeStageProgram)
	writeTR(t, dir, "syntax.tr", syntaxStageProgram)
	for _, file := range []string{"type.tr", "syntax.tr"} {
		tr1, _ := checkRecords(t, checkJSON(t, dir, "tr", file).Stdout)
		en, _ := checkRecords(t, checkJSON(t, dir, "en", file).Stdout)
		if len(tr1) == 0 || len(tr1) != len(en) {
			t.Fatalf("%s: kayıt sayısı tr %d, en %d", file, len(tr1), len(en))
		}
		for i := range tr1 {
			if tr1[i].Message == en[i].Message {
				t.Errorf("%s kayıt %d: ileti iki dilde aynı: %q", file, i, en[i].Message)
			}
			a, b := tr1[i], en[i]
			a.Message, b.Message = "", ""
			a.Related, b.Related = nil, nil // ilgili konumun iletisi de okuyanın dilindedir
			ja, _ := json.Marshal(a)
			jb, _ := json.Marshal(b)
			if string(ja) != string(jb) {
				t.Errorf("%s kayıt %d: kod, konum ya da düzeltme dile göre değişiyor:\ntr %s\nen %s", file, i, ja, jb)
			}
		}
	}
}

// 'applicable' düzeltme olduğu gibi uygulanınca dosya hatasız geçer (2.13'ün iki örneği: 0X1F → 0x1F,
// satır başındaki // → #); öneri düzeltmeleri (en yakın ad, ad yazılışı) applicable değildir.
func TestCheckJSONApplicableFixesMakeTheFileClean(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "syntax.tr", syntaxStageProgram)
	records, _ := checkRecords(t, checkJSON(t, dir, "en", "syntax.tr").Stdout)
	source := syntaxStageProgram
	applied := 0
	// Sondan başa: önceki düzeltmenin konumları kaymasın.
	for i := len(records) - 1; i >= 0; i-- {
		for _, f := range records[i].Fixes {
			if f.Applicable {
				source, applied = applyFix(t, source, f), applied+1
			}
		}
	}
	if applied != 2 {
		t.Fatalf("iki applicable düzeltme bekleniyordu (0X1F ve //), %d: %+v", applied, records)
	}
	if source == syntaxStageProgram || !strings.Contains(source, "0x1F") || !strings.Contains(source, "\n# yorum\n") {
		t.Fatalf("düzeltmeler beklenen yazımı vermedi:\n%s", source)
	}
	writeTR(t, dir, "fixed.tr", source)
	if r := checkJSON(t, dir, "en", "fixed.tr"); r.Code != 0 || r.Stdout != "" {
		t.Errorf("düzeltilmiş dosya hatasız geçmeli: çıkış %d\n%s", r.Code, r.Stdout)
	}

	writeTR(t, dir, "type.tr", typeStageProgram)
	typed, _ := checkRecords(t, checkJSON(t, dir, "en", "type.tr").Stdout)
	suggestions := 0
	for _, rec := range typed {
		for _, f := range rec.Fixes {
			suggestions++
			if f.Applicable {
				t.Errorf("%s: öneri düzeltmesi applicable olamaz: %+v", rec.Code, f)
			}
		}
	}
	if suggestions != 2 {
		t.Errorf("ad yazılışı ve tanımsız ad için iki öneri bekleniyordu, %d", suggestions)
	}
}

// Birbirine benzeyen iki ad: hata ikinciyi gösterir, ilgili konum ilkini (dosyası, yeri ve iletisiyle).
func TestCheckJSONRelatedPlace(t *testing.T) {
	dir := t.TempDir()
	program := "# TR 4\nTR_var m = 1\nTR_var rn = 2\nTR_print(m, rn)\n"
	writeTR(t, dir, "like.tr", program)
	records, _ := checkRecords(t, checkJSON(t, dir, "en", "like.tr").Stdout)
	var found *jsonRecordOut
	for i := range records {
		if records[i].Code == "H1020" {
			found = &records[i]
		}
	}
	if found == nil || len(found.Related) != 1 {
		t.Fatalf("H1020 ve bir ilgili konum bekleniyordu: %+v", records)
	}
	rel := found.Related[0]
	if rel.File != "like.tr" || rel.Start == nil || rel.End == nil || rel.Message == "" ||
		textBetween(t, program, *rel.Start, *rel.End) != "m" || textBetween(t, program, *found.Start, *found.End) != "rn" {
		t.Errorf("ilgili konum: %+v, hata %+v", rel, found)
	}
}

// Aktarılan modüldeki hatanın kaydı modülün dosyasını yazar, denetlenen dosyayı değil (1.3).
func TestCheckJSONNamesTheImportedModuleFile(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "ana.tr", "# TR 4\nTR_import \"modul\"\nTR_print(\"ana\")\n")
	writeTR(t, dir, "modul.tr", "# TR 4\nTR_var n: TR_Int = \"x\"\n")
	records, _ := checkRecords(t, checkJSON(t, dir, "en", "ana.tr").Stdout)
	if len(records) != 1 || records[0].File != "modul.tr" || records[0].Code != "H1001" || records[0].Start == nil || records[0].Start.Line != 2 {
		t.Errorf("modülün dosyası ve satırı bekleniyordu: %+v", records)
	}
	// Bulunamayan modül: hata ana dosyanın 'aktar' satırındadır.
	writeTR(t, dir, "eksik.tr", "# TR 4\nTR_import \"yok_boyle_modul\"\n")
	missing, _ := checkRecords(t, checkJSON(t, dir, "en", "eksik.tr").Stdout)
	if len(missing) != 1 || missing[0].File != "eksik.tr" || missing[0].Start == nil || missing[0].Start.Line != 2 {
		t.Errorf("ana dosyanın aktar satırı bekleniyordu: %+v", missing)
	}
	// İç içe modülün 'aktar' hatası hangi dosyanın satırı olduğu bilinmeden verilmez: yanlış dosyayı
	// göstermektense file ve start yazılmaz.
	writeTR(t, dir, "ic.tr", "# TR 4\nTR_import \"yok_boyle_modul\"\n")
	writeTR(t, dir, "dis.tr", "# TR 4\nTR_import \"ic\"\n")
	nested, raw := checkRecords(t, checkJSON(t, dir, "en", "dis.tr").Stdout)
	if len(nested) != 1 || nested[0].Message == "" {
		t.Fatalf("iç içe modül: %+v", nested)
	}
	for _, field := range []string{"file", "start", "end"} {
		if _, ok := raw[0][field]; ok {
			t.Errorf("iç içe modülün kaydı dosyasız olmalı, %q yazıyor: %v", field, raw[0])
		}
	}
}

// Kullanım hataları da kayıttır: konumları yoktur (file, start, end yazılmaz), çıkış kodu 2'dir; dosya
// okunamazsa çıkış 1'dir. --json dosya adından sonra yazılırsa programın argümanı sayılıp yutulmaz:
// kullanım hatasıdır ve ileti çalışan yazımı verir.
func TestCheckJSONUsageErrors(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "clean.tr", "# TR 4\nTR_print(1)\n")
	for _, c := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"dosya adı yok", []string{"check", "--json"}, 2, "H8605"},
		{"bilinmeyen seçenek", []string{"check", "--json", "--bogus", "clean.tr"}, 2, "H8674"},
		{"bilinmeyen seçenek json'dan önce", []string{"check", "--bogus", "--json", "clean.tr"}, 2, "H8674"},
	} {
		r := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, c.args...)
		records, raw := checkRecords(t, r.Stdout)
		if r.Code != c.code || len(records) != 1 || records[0].Code != c.want || records[0].Severity != "error" || records[0].Message == "" {
			t.Errorf("%s: çıkış %d, kayıtlar %+v, hata %q", c.name, r.Code, records, r.Stderr)
			continue
		}
		for _, field := range []string{"file", "start", "end"} {
			if _, ok := raw[0][field]; ok {
				t.Errorf("%s: konumsuz kayıt %q yazıyor: %v", c.name, field, raw[0])
			}
		}
	}
	// Dosya okunamıyor: çıkış 1, kayıt konumsuz.
	r := checkJSON(t, dir, "en", "yok.tr")
	if records, _ := checkRecords(t, r.Stdout); r.Code != 1 || len(records) != 1 || records[0].Start != nil || records[0].File != "" {
		t.Errorf("okunamayan dosya: çıkış %d, kayıtlar %+v", r.Code, records)
	}

	// --json dosya adından sonra: metin kipi kullanım hatası, standart çıktı boş.
	late := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "check", "clean.tr", "--json")
	if late.Code != 2 || late.Stdout != "" || !strings.Contains(late.Stderr, "check --json clean.tr") {
		t.Fatalf("dosyadan sonraki --json: çıkış %d, çıktı %q, hata %q", late.Code, late.Stdout, late.Stderr)
	}
	// İletinin önerdiği komut kopyalanınca çalışır.
	_, suggestion, _ := strings.Cut(strings.TrimSpace(late.Stderr), "E.g. tr ")
	fixed := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, strings.Fields(suggestion)...)
	if fixed.Code != 0 || fixed.Stdout != "" {
		t.Errorf("önerilen komut %q çalışmadı: çıkış %d, çıktı %q, hata %q", suggestion, fixed.Code, fixed.Stdout, fixed.Stderr)
	}
}

// Dosya yolları komutun çalıştığı klasöre göre göreli ve '/' ayraçlıdır (alt klasördeki dosya dahil).
func TestCheckJSONFileIsRelativeWithSlashes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "alt"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTR(t, dir, filepath.Join("alt", "prog.tr"), "# TR 4\nTR_print(prnt)\n")
	records, _ := checkRecords(t, checkJSON(t, dir, "en", "alt/prog.tr").Stdout)
	if len(records) != 1 || records[0].File != "alt/prog.tr" {
		t.Errorf("alt/prog.tr bekleniyordu: %+v", records)
	}
}
