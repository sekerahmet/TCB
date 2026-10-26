package codeindex

// Komutlar (Result.Command): JSON'daki değişmez tanımlayıcılar.
const (
	CommandDefinition = "definition"
	CommandReferences = "references"
	CommandTestedBy   = "tested-by"
	CommandImpact     = "impact"
	CommandRename     = "rename"
)

// Durumlar (Result.Status).
const (
	StatusOK        = "ok"
	StatusAmbiguous = "ambiguous" // ad birden çok düğüme uyuyor; Candidates adayları verir
	StatusNotFound  = "not_found" // hiçbir düğüme uymadı; DidYouMean yakın adları verir
	StatusRejected  = "rejected"  // yeniden adlandırılamaz (yerleşik ad, geçersiz ya da dilin sözcüğü olan yeni ad); Reason nedenidir
)

// Çıkış kodları: komut satırı sonucun durumunu çıkış koduyla da bildirir.
const (
	ExitOK        = 0
	ExitFail      = 1
	ExitUsage     = 2
	ExitAmbiguous = 3
	ExitNotFound  = 4
)

// ExitCode bir durumun çıkış kodudur.
func ExitCode(status string) int {
	switch status {
	case StatusAmbiguous:
		return ExitAmbiguous
	case StatusNotFound:
		return ExitNotFound
	case StatusRejected:
		return ExitUsage
	}
	return ExitOK
}

// Query sorulan şeydir.
type Query struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"` // çıktıdaki gösterim adlarının dili
}

// Use bir başvuru satırıdır: kim, nerede, ne yapıyor.
type Use struct {
	Role       Role       `json:"role"`
	Confidence Confidence `json:"confidence"`
	Pos        Pos        `json:"pos"`
	In         *Node      `json:"in,omitempty"`
	Steps      int        `json:"steps,omitempty"` // dolaylı sınama: kaç çağrı adımı uzakta (0: doğrudan)
}

// Counts sonuç sayılarıdır: listeler Limit ile kesilse de toplamlardır (metin çıktısının başlıkları bunları yazar).
type Counts struct {
	Total   int          `json:"total"`
	ByRole  map[Role]int `json:"by_role,omitempty"`
	Modules int          `json:"modules,omitempty"` // etki: aktaran modüller
	Tests   int          `json:"tests,omitempty"`   // etki: etkilenen testler
	Certain int          `json:"certain,omitempty"` // yeniden adlandırma: kesin değişiklikler
	Likely  int          `json:"likely,omitempty"`  // yeniden adlandırma: olası değişiklikler
}

// Affected `etki` sorgusunun sonucudur: değişirse etkilenenler.
type Affected struct {
	Modules []Node `json:"modules,omitempty"` // onu (dolaylı da) aktaran modüller
	Users   []Use  `json:"users,omitempty"`
	Tests   []Node `json:"tests,omitempty"`
}

// Result bütün komutların ortak JSON zarfıdır. Şema sürümü Schema'dadır; alan eklemek uyumludur.
type Result struct {
	Schema     string    `json:"schema"`
	Command    string    `json:"command"`
	Status     string    `json:"status"`
	Query      Query     `json:"query"`
	Target     *Node     `json:"target,omitempty"`
	Docs       string    `json:"docs,omitempty"`
	Members    []Node    `json:"members,omitempty"` // tanım: yapının alanları ve metotları, modülün bildirimleri
	Users      []Use     `json:"users,omitempty"`
	Tests      []Use     `json:"tests,omitempty"`
	Affected   *Affected `json:"affected,omitempty"`
	Candidates []Node    `json:"candidates,omitempty"`
	DidYouMean []Node    `json:"did_you_mean,omitempty"`
	Counts     *Counts   `json:"counts,omitempty"`
	Truncated  bool      `json:"truncated"`
	// Yeniden adlandırma etkisi (command: rename).
	NewName      string     `json:"new_name,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	Targets      []Node     `json:"targets,omitempty"`
	Certain      []Change   `json:"certain,omitempty"`
	Likely       []Change   `json:"likely,omitempty"`
	Conflicts    []Conflict `json:"conflicts,omitempty"`
	Linked       []Node     `json:"linked,omitempty"` // bağlı bildirimler: aynı adla iletilen ya da aynı adlı alana atanan (linked.go)
	SkippedWords int        `json:"skipped_other_words,omitempty"`
	Mode         string     `json:"mode,omitempty"` // "term": sembol aranmadı, yalnızca metin
}

// NewResult şema adı yazılmış boş bir sonuç kurar.
func NewResult(command string, q Query) *Result {
	return &Result{Schema: Schema, Command: command, Status: StatusOK, Query: q}
}

// EncodeJSON sonucu girintili JSON olarak yazar (sonunda satır sonu). Alan sırası yapıdaki sıradır,
// sözlük anahtarları sıralıdır: aynı sonuç aynı baytları verir.
func EncodeJSON(r *Result) ([]byte, error) {
	return marshalIndentLine(r, "  ")
}

// ModeTerm yeniden adlandırma etkisinin terim kipidir (Result.Mode).
const ModeTerm = "term"
