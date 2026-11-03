package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// 'tr check --json dosya.tr' (dil tanımı 2.13): çalışmadan önceki hatalar ve uyarılar, kullanım
// hataları dahil, standart çıktıya JSON Lines olarak yazılır: her kayıt bir satırda bir JSON nesnesi,
// başka metin yok, kayıt yoksa çıktı boş. Çıkış kodu metin kipindekiyle aynıdır (hata 1, kullanım
// hatası 2, temiz ya da yalnız uyarı 0). Alan adları dilden bağımsızdır; yalnız message okuyanın
// dilindedir (TR_LANG).

// jsonPosition satır ve sütundur (1'den başlar; sütun Unicode karakteridir, 2.1). Sütun bilinmiyorsa
// yazılmaz.
type jsonPosition struct {
	Line   int `json:"line"`
	Column int `json:"column,omitempty"`
}

// jsonRelated hatayla ilgili öbür bir yerdir. File hatanın kendi dosyasıysa da yazılır.
type jsonRelated struct {
	File    string        `json:"file,omitempty"`
	Start   *jsonPosition `json:"start,omitempty"`
	End     *jsonPosition `json:"end,omitempty"`
	Message string        `json:"message"`
}

// jsonFix önerilen bir düzeltmedir: Start'tan End'e (dışlayıcı) olan metnin yerine Replacement
// yazılır (boş metin silmektir). Applicable yalnız olduğu gibi uygulanınca doğru olan düzeltmede true.
type jsonFix struct {
	Start       jsonPosition `json:"start"`
	End         jsonPosition `json:"end"`
	Replacement string       `json:"replacement"`
	Applicable  bool         `json:"applicable"`
}

// jsonRecord bir hata ya da uyarıdır. Alanların sırası kayıtta böyle yazılır.
type jsonRecord struct {
	Code     string        `json:"code"`
	Severity string        `json:"severity"`
	File     string        `json:"file,omitempty"`
	Start    *jsonPosition `json:"start,omitempty"`
	End      *jsonPosition `json:"end,omitempty"`
	Message  string        `json:"message"`
	Related  []jsonRelated `json:"related,omitempty"`
	Fixes    []jsonFix     `json:"fixes,omitempty"`
}

const (
	severityError   = "error"
	severityWarning = "warning"
)

// jsonUsage 'tr check --json' komutunun kullanım hatalarının da JSON olarak yazılacağını söyler
// (exitUsage). Seçenekler çözülmeden önce, komut satırı taranarak kurulur.
var jsonUsage bool

// writeJSONRecord kaydı standart çıktıya tek satır olarak yazar.
func writeJSONRecord(r jsonRecord) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false) // '<' ve '&' okunur kalır
	_ = enc.Encode(r)        // standart çıktı kapalıysa yapacak bir şey yok
}

// recordOf hatadan (ya da uyarıdan) kaydı kurar. Kod en içteki koddur (ayrıştırıcının her
// sözdizimi hatasını sardığı H0100 değil); ileti konum önekisizdir, çünkü konum ayrı alanlardadır.
// Konumu bilinmeyen hatada file, start ve end yazılmaz. file, hatanın dosyasıdır.
func recordOf(err error, severity, file string) jsonRecord {
	r := jsonRecord{Code: messages.H9000.String(), Severity: severity, Message: err.Error()}
	if code, ok := messages.InnerCode(err); ok {
		r.Code = code.String()
	}
	pos, located := messages.Find(err)
	if !located {
		return r
	}
	r.Message = pos.Err.Error()
	r.File = file
	r.Start, r.End = positionsOf(pos.Range())
	for _, rel := range pos.Related {
		item := jsonRelated{File: rel.File, Message: rel.Err.Error()}
		if item.File == "" {
			item.File = file
		}
		item.Start, item.End = positionsOf(rel.Range)
		r.Related = append(r.Related, item)
	}
	for _, f := range pos.Fixes {
		start, end := positionsOf(f.Range)
		if start == nil || end == nil {
			continue // yeri tam bilinmeyen düzeltme uygulanamaz
		}
		r.Fixes = append(r.Fixes, jsonFix{Start: *start, End: *end, Replacement: f.Replacement, Applicable: f.Applicable})
	}
	return r
}

// positionsOf aralığın başlangıç ve bitişidir; sütunu bilinmeyen kayıtta başlangıç yalnız satır
// taşır ve bitiş yoktur.
func positionsOf(r messages.Range) (start, end *jsonPosition) {
	if r.Line <= 0 {
		return nil, nil
	}
	start = &jsonPosition{Line: r.Line, Column: r.Col}
	if r.Col > 0 && r.EndLine > 0 && r.EndCol > 0 {
		end = &jsonPosition{Line: r.EndLine, Column: r.EndCol}
	}
	return start, end
}

// usageRecord tr'nin kendi kullanım hatasının kaydıdır: konumu yoktur.
func usageRecord(err *messages.Error) jsonRecord { return recordOf(err, severityError, "") }

// shownFile dosya yolunun kayıtta yazılan biçimidir: komutun çalıştığı klasörün altındaysa ona göre,
// değilse mutlak; ayırıcı '/'. Aktarılan modülün dosyası (interp.Source.Name) aynı kuralla yazılır.
func shownFile(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, absolute); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(absolute)
}

// checkFileJSON dosyayı ve aktardığı modülleri çalıştırmadan denetler (1.3) ve sonucu JSON olarak
// yazar; 'tr check'ün metin kipiyle aynı denetimdir, yalnız çıktı değişir. Hata varsa 1 ile çıkar.
func checkFileJSON(path string) {
	file := shownFile(path)
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSONRecord(recordOf(messages.New(messages.H8657, turkish.FileError("", path, messages.H2100, err)), severityError, ""))
		exit(1)
	}
	s := source.Prepare(string(data), source.Options{TypeCheck: true, Library: interp.LibraryFileName(path, string(data))})
	failed := s.Stage == source.StageSyntax || s.Stage == source.StageType
	for _, w := range s.Warnings {
		writeJSONRecord(recordOf(w, severityWarning, file))
	}
	for _, e := range s.Errors {
		writeJSONRecord(recordOf(e, severityError, file))
	}
	if failed {
		exit(1)
	}
	if err := interp.CheckImportedModules(path, s.Program); err != nil {
		for _, r := range moduleRecords(err, file, s.Program) {
			writeJSONRecord(r)
		}
		exit(1)
	}
}

// moduleRecords aktarılan modüllerin denetim hatalarının kayıtlarıdır. Hata bir modülün kendi
// dosyasındaysa (interp.ModuleError) kayıtlar o dosyayı yazar; bir 'aktar' satırının hatasıysa
// (modül bulunamadı, okunamadı) dosya, satırı ve modül yolu o satırdaki 'aktar'la uyuşan ana dosyadır.
// İç içe bir modülün 'aktar' hatası kendi dosyasına bağlanamaz (yorumlayıcı dosyasını taşımıyor): onda
// dosya yazılmaz.
func moduleRecords(err error, mainFile string, prog *ast.Program) []jsonRecord {
	var moduleErr *interp.ModuleError
	if errors.As(err, &moduleErr) && len(moduleErr.Errors) > 0 {
		out := make([]jsonRecord, len(moduleErr.Errors))
		for i, e := range moduleErr.Errors {
			out[i] = recordOf(e, severityError, moduleErr.Source.Name)
		}
		return out
	}
	file := ""
	if pos, ok := messages.Find(err); ok && importsAt(prog, pos.Line, err) {
		file = mainFile
	}
	r := recordOf(err, severityError, file)
	if file == "" {
		r.Start, r.End = nil, nil // satırı hangi dosyanın satırı olduğu bilinmeden verilmez
	}
	return []jsonRecord{r}
}

// importsAt ana programın line satırında yolu err'in iletisinde geçen bir 'aktar' deyimi olduğudur.
func importsAt(prog *ast.Program, line int, err error) bool {
	found := false
	text := err.Error()
	ast.Walk(prog, func(n any) bool {
		if im, ok := n.(*ast.ImportStatement); ok && im.Line() == line && strings.Contains(text, im.Path) {
			found = true
		}
		return true
	})
	return found
}

// checkOptions 'tr check'ün seçeneklerini (dosya adından önce) çözer: json açıksa true.
func checkOptions(command any, args []string) (jsonOut bool, rest []string) {
	allowed := []codes.Code{codes.OptJson}
	for len(args) > 0 && looksLikeOption(args[0]) {
		if !isOption(args[0], codes.OptJson) {
			unknownOption(command, args[0], allowed)
		}
		jsonOut, args = true, args[1:]
	}
	return jsonOut, args
}

// scanJSON komut satırının baştaki seçenekleri arasında --json olup olmadığına bakar; kullanım
// hatalarının hangi kipte yazılacağı seçenekler çözülmeden önce bilinmelidir.
func scanJSON(args []string) bool {
	for _, a := range args {
		if !looksLikeOption(a) {
			return false
		}
		if isOption(a, codes.OptJson) {
			return true
		}
	}
	return false
}
