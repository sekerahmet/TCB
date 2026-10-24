// Package codeindex TR kodunun dizinidir: bir projenin .tr dosyalarındaki işlev, yapı, metot, alan,
// sabit, değişken, test ve yerleşik adlar; kimin neyi tanımladığı, çağırdığı, kullandığı, aktardığı
// ve sınadığı; her başvurunun yeri. "Bu nerede tanımlı", "kimler kullanıyor", "hangi testler
// sınıyor", "bunu değiştirirsem ne etkilenir" sorularını insana (düz metin), yapay zekâ ajanına
// (kararlı JSON şeması) ve ileride dil sunucusuna yanıtlar.
//
// Dizin kaynağı etkin yazım dilinden bağımsız okur (başlıksız dosya Türkçedir, kodlu dosya önce
// Türkçeye çözülür): içeriği, kimlikleri ve konumları her dilde aynıdır; yalnızca çıktı, dilin
// öğelerini okuyanın dilinde yazar (codes.DisplayName).
//
// Layer: developer tools (internal/devtools); yalnızca internal/lang paketlerini (ve gömülü
// kitaplığı) içe aktarır.
package codeindex

import "strconv"

// SchemaVersion JSON şemasının sürümüdür (Schema onun metin biçimi). Alan eklemek uyumludur;
// alan çıkarmak ya da anlamını değiştirmek sürümü artırır.
const SchemaVersion = 1

// Schema her JSON sonucuna yazılan şema adıdır.
const Schema = "tr-codeindex/1"

// IndexVersion dizinin kaynaktan türettiği içerik değişince artar (yeni düğüm türü, farklı
// çözümleme). Başka sürümün önbelleği hiçbir zaman kullanılmaz.
const IndexVersion = 16

// Kind bir düğümün türüdür; değerler JSON'a yazılan değişmez tanımlayıcılardır.
type Kind string

const (
	KindModule    Kind = "module"   // bir .tr dosyası
	KindFunction  Kind = "function" // üst düzey işlev
	KindStruct    Kind = "struct"
	KindMethod    Kind = "method" // yapı metodu
	KindField     Kind = "field"  // yapı alanı
	KindConstant  Kind = "constant"
	KindVariable  Kind = "variable"
	KindTest      Kind = "test_block" // bir test "..." bloğu
	KindBuiltin   Kind = "builtin"    // yerleşik ad (ad kodu: global işlev, üye ya da adlı argüman)
	KindParameter Kind = "parameter"  // işlev ya da metot parametresi
	KindOption    Kind = "option"     // komut_satırı sözlüğünün anahtarı (komut satırı seçeneği)
)

// EdgeKind iki düğüm arasındaki bağın türüdür.
type EdgeKind string

const (
	EdgeDefines EdgeKind = "defines" // modül → bildirim, yapı → metot ve alan
	EdgeCalls   EdgeKind = "calls"
	EdgeUses    EdgeKind = "uses" // sabit, değişken ya da yerleşik adına başvuru
	EdgeImports EdgeKind = "imports"
	EdgeTests   EdgeKind = "tests"
	// EdgeForwards parametrenin aynı adla iletildiği parametre ya da atandığı alandır (bağlı bildirim; linked.go).
	EdgeForwards EdgeKind = "forwards"
)

// Confidence bağın nasıl bulunduğunu söyler: EXTRACTED kesin (kapsamlarda ada göre çözüldü),
// INFERRED tahmindir (örneğin alıcı tipi bilinmeyen bir metot çağrısı).
type Confidence string

const (
	Extracted Confidence = "EXTRACTED"
	Inferred  Confidence = "INFERRED"
)

// Role bir başvurunun düğüme ne yaptığıdır (SCIP'in simge rolleri sözlüğünden TR'de karşılığı
// olanlar).
type Role string

const (
	RoleDefinition Role = "definition"
	RoleCall       Role = "call"
	RoleRead       Role = "read"
	RoleWrite      Role = "write"
	RoleImport     Role = "import"
	RoleTest       Role = "in_test"
	RoleArgument   Role = "argument_name" // çağrıdaki adlı argümanın adı (`f(ad: 1)`)
	RoleOptionKey  Role = "option_key"    // komut_satırı sözlüğünün anahtarı
)

// Pos bir kaynak dosyadaki konumdur. Satır ve sütun 1'den başlar; sütun, satırın Türkçe
// gösterimindeki Unicode kod noktalarını sayar (kodlu dosyada da aynı, 17.5). EndLine sıfırsa
// konumun sonu yoktur.
type Pos struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col,omitempty"`
	EndLine int    `json:"end_line,omitempty"`
	EndCol  int    `json:"end_col,omitempty"`
}

// String konumu dosya:satır[:sütun] olarak yazar.
func (p Pos) String() string {
	s := p.File + ":" + strconv.Itoa(p.Line)
	if p.Col > 0 {
		s += ":" + strconv.Itoa(p.Col)
	}
	return s
}

// Node bir bildirim, modül ya da yerleşik addır. Name kanonik addır: kullanıcı adlarında yazıldığı
// gibi, yerleşikte kodu (G502). Display yerleşiğin okuyanın dilindeki adıdır, kullanıcı adlarında
// boştur.
type Node struct {
	ID        string `json:"id"`
	Kind      Kind   `json:"kind"`
	Name      string `json:"name"`
	Display   string `json:"display,omitempty"`
	Pos       *Pos   `json:"pos,omitempty"`
	Signature string `json:"signature,omitempty"`
	// Qualified modül adıyla nitelenmiş ad ("katman.Doğrusal.ileri"): sorguda yazılan biçim.
	Qualified string `json:"qualified,omitempty"`
	// Aliases bir modülün başka adlarıdır (içe aktarma yolu "tr/katman").
	Aliases []string `json:"aliases,omitempty"`
	Coded   bool     `json:"coded,omitempty"` // modül: dosya kodlu biçimdeydi
	// Imports modülün `aktar` ifadeleridir (kaynak sırasıyla); artımlı yenileme değişmeyen
	// dosyaların aktarmalarını buradan kurar.
	Imports []Import `json:"imports,omitempty"`
	// Doc bildirimin üstündeki yorum satırlarıdır.
	Doc string `json:"doc,omitempty"`
	// InferredType çıkarılan yapı türüdür (yapı düğümünün kimliği; infer.go): işlev ve metotta dönüş türü, üst düzey
	// değişken ve sabitte değerin türü. Bilinmezse boş.
	InferredType string `json:"inferred_type,omitempty"`
}

// Import bir `aktar` ifadesidir: Path yazıldığı gibi, Target çözülen modülün dosyası (çözülemezse
// boş), Alias `olarak` adı (yoksa boş), Pos ifadenin yeri.
type Import struct {
	Nested bool   `json:"nested,omitempty"` // üst düzeyde değil, bir test bloğu ya da gövde içinde
	Path   string `json:"path"`
	Target string `json:"target,omitempty"`
	Alias  string `json:"alias,omitempty"`
	Pos    Pos    `json:"pos"`
}

// Edge iki düğümü bağlar. Pos bağın ilk görüldüğü yerdir (öbürleri başvurulardır).
type Edge struct {
	From       string     `json:"from"`
	To         string     `json:"to"`
	Kind       EdgeKind   `json:"kind"`
	Confidence Confidence `json:"confidence"`
	Pos        *Pos       `json:"pos,omitempty"`
	Count      int        `json:"count,omitempty"` // aynı bağın kaç kez görüldüğü
}

// Occurrence bir düğüme başvuran tek bir yerdir: dil sunucusunun "kullanımları bul", yeniden
// adlandırma ve `aktar` satırından modüle atlama için ihtiyacı olan şey. Kenarlar bunların
// özetidir.
type Occurrence struct {
	Node       string     `json:"node"`
	Role       Role       `json:"role"`
	Confidence Confidence `json:"confidence"`
	Pos        Pos        `json:"pos"`
	In         string     `json:"in,omitempty"` // başvuruyu içeren bildirim (ya da modül)
	// Basis üye erişiminin neye dayanarak bağlandığıdır: boşsa ada göre çözüldü; BasisSelf `kendi.x`;
	// BasisLocalType tek atamayla yapıcıdan kurulan yerel (`değişken d = Yapı()`); BasisNameMatch alıcı
	// tipi bilinmeden adı tutan metot adayı (kenarı vardır); BasisNameOnly yalnızca adı tutan üye adayı
	// (kenarı yoktur: yalnızca yeniden adlandırma etkisinde görünür, kullananlar listesinde değil).
	Basis Basis `json:"basis,omitempty"`
}

// Basis bir başvurunun dayanağıdır.
type Basis string

const (
	BasisSelf      Basis = "self"
	BasisLocalType Basis = "local_type"
	BasisNameMatch Basis = "name_match"
	BasisNameOnly  Basis = "name_only"
	// BasisInferredType alıcının türü bir işlevin çıkarılan dönüş türünden gelir (infer.go).
	BasisInferredType Basis = "inferred_type"
	// BasisCallableValue işlev yerleşiğinin çağrılmadan değer olarak kullanılıp çağrılabilir bir yere gittiğidir: bir
	// çağrının argümanı (`[3, 1, 2].indirge(maks)`) ya da aynı gövdede çağrılan bir değişkene atanması
	// (`değişken g = mutlak` ... `g(-2)`). Test bloğunda bu, yerleşiği sınamaktır (exercisesBuiltin).
	BasisCallableValue Basis = "callable_value"
)

// Index bir projenin bütün dizinidir. Düğümler, kenarlar ve başvurular sıralıdır: aynı proje aynı
// baytları verir.
type Index struct {
	Schema      int          `json:"schema_version"`
	Version     int          `json:"index_version"`
	Files       []string     `json:"files"`
	Skipped     []Skipped    `json:"skipped,omitempty"`
	Unresolved  []Unresolved `json:"unresolved,omitempty"`
	Nodes       []Node       `json:"nodes"`
	Edges       []Edge       `json:"edges"`
	Occurrences []Occurrence `json:"occurrences"`
	// Constructs bildirimlerin kullandığı dil yapılarıdır (constructs.go; kod grafiği Go uygulamasına bağlar).
	Constructs []Construct `json:"constructs,omitempty"`
}

// Skipped okunamayan ya da ayrıştırılamayan bir dosyadır: modül düğümü vardır, bildirimleri yoktur.
type Skipped struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// Unresolved hiçbir kapsamda ve dilin hiçbir adında bulunamayan bir addır (yazım hatası, dinamik
// tanım ya da dizinin göremediği bir modül).
type Unresolved struct {
	Name string `json:"name"`
	Pos  Pos    `json:"pos"`
}

// NodeID bir bildirimin kimliğini kurar: tür, dosya ve ad "function:lib/katman.tr#cevre"
// biçiminde birleşir; yerleşiğin dosyası yoktur ("builtin:G502"), modülün adı yoktur.
func NodeID(kind Kind, file, name string) string {
	switch kind {
	case KindBuiltin:
		return string(kind) + ":" + name
	case KindModule:
		return string(kind) + ":" + file
	}
	return string(kind) + ":" + file + "#" + name
}
