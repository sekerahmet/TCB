package internal_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// oldCodeForm madde 15'in 0. adımından önceki numaralı kod biçimidir (A001, G100, Y400); ileti kodları (H ve dört
// rakam) bu biçimde değildir.
var oldCodeForm = regexp.MustCompile(`(^|[^\p{L}\p{N}_])([AGKTVWY][0-9]{3})([^\p{L}\p{N}_]|$)`)

// oldCodeAllowed eski kod biçiminde ama kod olmayan ya da eski biçimi bilerek anan sözcüklerdir: dosya → sözcük →
// gerekçe ("*" dosyanın bütün eşleşmeleri). Liste küçük tutulur; yeni bir kayıt gerekçesiyle girer.
var oldCodeAllowed = map[string]map[string]string{
	"docs/code_identities.md":                 {"*": "eski kodu kimliğe eşleyen belge: eski kodları bilerek taşır"},
	"internal/old_codes_test.go":              {"*": "bu testin kendisi: eski biçimin ve izin listesinin örnekleri"},
	"internal/compute/tokenize/norm_table.go": {"*": "Unicode kod noktası tablosu (A770 ...), kod değil"},
	"internal/compute/gpu/gpu_bench_test.go":  {"T256": "tensör boyutu (T = 256), kod değil"},
	"tools/record/main_test.go":               {"A100": "NVIDIA A100 kartının adı"},
	"docs/roadmap.md": {"A100": "NVIDIA A100 kartının adı", "A001": "madde 15: eski kodun kimliğe geçişinin örneği",
		"G400": "madde 15: eski kodun kimliğe geçişinin örneği", "T910": "madde 15: eski kodun kimliğe geçişinin örneği"},
	"docs/README.md":                       {"A001": "eski kod biçiminin örneği (eşleme belgesinin tanıtımı)"},
	"docs/spec/02_sozcuk_yapisi.md":        {"A001": "2.3a: eski biçim artık sıradan addır", "G100": "2.3a: eski biçim artık sıradan addır"},
	"internal/lang/codes/README.md":        {"A001": "eski kodların eşleme belgesine yönlendirme"},
	"internal/lang/codes/language_test.go": {"A001": "eski biçim kimlik değildir ve ayrılmış değildir", "G100": "eski biçim kimlik değildir ve ayrılmış değildir"},
	"tests/negative/kodlu_surum.tr":        {"A030": "eski kodlu biçim sürümü (2) okunmaz"},
	"tests/sozcuk_yapisi_test.tr":          {"G100": "2.3a: eski kod biçimi sıradan addır"},
	"tools/codegraph/codehist.go":          {"A001": "geçmişteki eski anahtarlar da okunur"},
	"tools/codegraph/testdata/planning_questions.txt": {"K102": "silinmiş kodun geçmişi (codegraph deleted sorusu)",
		"G400": "eski anahtarın kimliğe yönlendirilmesi ve geçmişi (query, history, deleted soruları)",
		"A001": "kimliğe yeniden adlandırılan kod silinmiş sayılmaz (deleted sorusu)"},
}

// oldCodeBinary metin olmayan izlenen dosyalardır (uzantı ya da ad): ret listesi. UTF-8 olmayan dosya da atlanır.
var oldCodeBinary = map[string]bool{".npy": true, ".safetensors": true, ".trt": true, "go.sum": true}

// Kural (dil tanımı 17, madde 15 adım 0): kodlar kimlikleriyle anılır (TR_fn_print); izlenen metin dosyalarında
// (uzantısı ne olursa olsun; ikili biçimler ve UTF-8 olmayanlar hariç) eski numaralı kod biçimi (G100) kalmaz. Neden:
// eşleme belgesi dışında kalan eski anılış, okuyanı (ve hafızası olmayan bir oturumu) artık var olmayan bir kod
// düzenine götürür.
func TestNoOldCodeForm(t *testing.T) {
	root := reporoot.Root()
	var bad []string
	used := map[string]bool{} // "dosya sözcük": izin listesinin kullanılan kayıtları
	files := 0
	err := reporoot.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if oldCodeBinary[filepath.Ext(path)] || oldCodeBinary[d.Name()] {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if strings.HasPrefix(rel, "internal/compute/tokenize/testdata/") {
			return nil // parçalayıcının altın çıktıları: rastgele metin parçaları (T000), kod değil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(data) {
			return nil // ikili dosya
		}
		files++
		allowed := oldCodeAllowed[rel]
		for i, line := range strings.Split(string(data), "\n") {
			for _, m := range oldCodeForm.FindAllStringSubmatch(line, -1) {
				if allowed["*"] != "" {
					used[rel+" *"] = true
					continue
				}
				if _, ok := allowed[m[2]]; ok {
					used[rel+" "+m[2]] = true
					continue
				}
				bad = append(bad, rel+":"+strconv.Itoa(i+1)+": "+m[2])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 500 {
		t.Fatalf("only %d files scanned; the repository was not read", files)
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("%d eski kod anılışı (kimliği docs/code_identities.md'de; kod değilse gerekçesiyle oldCodeAllowed'a):\n  %s",
			len(bad), strings.Join(bad, "\n  "))
	}
	for file, words := range oldCodeAllowed {
		for w := range words {
			if !used[file+" "+w] {
				t.Errorf("oldCodeAllowed: %s dosyasında %s artık geçmiyor; kaydı silin", file, w)
			}
		}
	}
}
