package interp

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Modülün kimliği (11.3): aynı dosya aynı modüldür. Başka bir yazımla (bağlantı, dizin kavşağı,
// sabit bağ, 8.3 kısa ad, sürücü harfinin büyüklüğü) ulaşılan dosya da aynı modüldür; anahtarı
// dosyaya ilk ulaşan yazımın mutlak yoludur. Dosya os.Stat ile (bağlantı açılarak) okunur ve
// os.SameFile ile karşılaştırılır: filepath.EvalSymlinks dizin kavşağında hata verir, sabit bağın
// da tek bir gerçek yolu yoktur. Modülün göreli yolları sözcükseldir (`..`, library.go'daki
// filepath.Join): iki yazımın klasör yolu (klasör ve bütün üst klasörleri) aynı değilse modülün
// göreli yolları hangi aktarımın önce geldiğine bağlanırdı; bu bir hatadır (H2726). Çalışma
// anı, çalışmadan önceki denetim, komut satırı taraması ve 'trlang build' aynı ModuleIdentities'i
// kullanır.

// ModuleIdentities, bir koşunun (ya da bir denetimin) gördüğü modül dosyalarıdır. Sıfır değeri
// kullanılmaz; NewModuleIdentities ile kurulur. Eşzamanlı kullanımda kilidi çağıran tutar.
type ModuleIdentities struct {
	byKey map[string]*moduleIdentity
	list  []*moduleIdentity
}

// moduleIdentity bir modül dosyasının kaydıdır: ilk yazımın mutlak yolu, os.Stat bilgisi ve
// ilk aktarımın yeri (H2726 onu gösterir).
type moduleIdentity struct {
	key  string
	info os.FileInfo
	file string // ilk aktarımın dosyasının gösterilen adı
	line int
	main bool // ana program (11.3: ana programı aktarmak döngüdür)
}

// ModuleKey, Key'in cevabıdır.
type ModuleKey struct {
	Key   string // dosyanın anahtarı: ona ilk ulaşan yazımın mutlak yolu
	Known bool   // dosya daha önce görüldü (aynı yol ya da aynı dosya)
	Main  bool   // dosya ana programdır
	// Moved, dosyaya klasör yolu farklı bir yazımla ulaşıldığında H2726'dır (Known ile).
	Moved error
}

// NewModuleIdentities boş bir küme kurar; mainFile ana programsa ("" ve "<...>" değilse) kümeye
// ana program olarak girer: modülden ana programa giden yazım da (bağlantıyla bile) tanınır.
func NewModuleIdentities(mainFile string) *ModuleIdentities {
	m := &ModuleIdentities{byKey: map[string]*moduleIdentity{}}
	if mainFile != "" && !strings.HasPrefix(mainFile, "<") {
		if id := m.add(absPath(mainFile), "", 0); id != nil {
			id.main = true
		}
	}
	return m
}

// Key, importer dosyasının line satırında written diye aktarılan ve resolved'a çözülen modülün
// anahtarıdır. Dosya ilk kez görülüyorsa kümeye bu aktarımla girer. Aynı yol kümede ise Moved
// yoktur; aynı dosyaya başka bir yazımla ulaşıldıysa ve iki yazımın klasör yolu aynı değilse
// Moved H2726'dır (önerisi ilk yazımın dosyası, importer'ın klasörüne göre). Okunamayan dosya
// kümeye girmez, anahtarı mutlak yoludur (okuma hatasını çağıranın yolu verir).
func (m *ModuleIdentities) Key(written, resolved, importer string, line int) ModuleKey {
	abs := absPath(resolved)
	if id := m.byKey[abs]; id != nil {
		return ModuleKey{Key: id.key, Known: true, Main: id.main}
	}
	info, err := os.Stat(abs)
	if err != nil {
		return ModuleKey{Key: abs}
	}
	for _, id := range m.list {
		if !os.SameFile(id.info, info) {
			continue
		}
		k := ModuleKey{Key: id.key, Known: true, Main: id.main}
		if !sameFolderPath(id.key, abs) {
			k.Moved = messages.New(messages.H2726, written, id.file, id.line, firstSpelling(written, id.key, importer))
		}
		return k
	}
	m.list = append(m.list, &moduleIdentity{key: abs, info: info, file: importerName(importer), line: line})
	m.byKey[abs] = m.list[len(m.list)-1]
	return ModuleKey{Key: abs}
}

// add abs'yi kümeye koyar (okunamazsa nil).
func (m *ModuleIdentities) add(abs, file string, line int) *moduleIdentity {
	info, err := os.Stat(abs)
	if err != nil {
		return nil
	}
	id := &moduleIdentity{key: abs, info: info, file: file, line: line}
	m.list = append(m.list, id)
	m.byKey[abs] = id
	return id
}

// sameFolderPath iki mutlak yolun klasör zincirlerinin (filepath.Dir'leri, kökün kendisine kadar)
// aynı uzunlukta ve her basamağının aynı klasör olduğunu söyler (os.Stat, os.SameFile).
func sameFolderPath(a, b string) bool {
	da, db := filepath.Dir(a), filepath.Dir(b)
	for {
		ia, errA := os.Stat(da)
		ib, errB := os.Stat(db)
		if errA != nil || errB != nil || !os.SameFile(ia, ib) {
			return false
		}
		pa, pb := filepath.Dir(da), filepath.Dir(db)
		rootA, rootB := pa == da, pb == db
		if rootA || rootB {
			return rootA && rootB
		}
		da, db = pa, pb
	}
}

// firstSpelling H2726'nın önerisidir: ilk yazımın dosyası, aktaran dosyanın klasörüne göre
// göreli (sürücüler farklıysa ya da aktaranın klasörü yoksa mutlak), '/' ile; yazılan yolda .tr
// yoksa öneride de olmaz. Göreli yol sözcükseldir: aktaranın klasörüyle birleşince ilk yazıma varır.
func firstSpelling(written, key, importer string) string {
	path := key
	if dirs := FileDirs(importer); len(dirs) > 0 && !strings.HasPrefix(importer, LibraryPrefix) {
		if rel, err := filepath.Rel(absPath(dirs[0]), key); err == nil {
			path = rel
		}
	}
	return writtenLike(written, filepath.ToSlash(path))
}

// importerName aktaran dosyanın iletide gösterilen adıdır (kitaplık dosyasında yolu).
func importerName(importer string) string {
	if strings.HasPrefix(importer, LibraryPrefix) {
		return importer
	}
	return shownPath(importer)
}

// moduleIdentities koşunun kümesidir (v.isImported tutulurken çağrılır); ilk modülde ana
// programla kurulur.
func (v *VM) moduleIdentities() *ModuleIdentities {
	if v.moduleIDs == nil {
		v.moduleIDs = NewModuleIdentities(v.SourceFile)
	}
	return v.moduleIDs
}

// moduleKey diskteki dest modülünün anahtarıdır (v.isImported tutulurken); path 'aktar' yolunun
// .tr'li hâlidir. Gömülü modülün anahtarı yoludur. Ana programa ulaşan yazım döngüdür (H2481);
// aktaran görevin yükleme zincirindeki dosyaya ulaşan yazım klasör yolu tutmasa da döngü olarak
// kalır (loadModule H2481'i verir: döngünün doğru yazımı yoktur); zincirde olmayan dosyaya klasör
// yolu farklı bir yazımla ulaşmak H2726'dır.
func (v *VM) moduleKey(s *ast.ImportStatement, path string, dest *Source) (string, error) {
	if dest.Embedded {
		return dest.Path, nil
	}
	importer := v.SourceFile
	if v.source != nil {
		importer = v.source.Path
	}
	k := v.moduleIdentities().Key(s.Path, dest.Path, importer, s.Line())
	switch {
	case k.Main:
		return "", messages.At(s.Line(), 0, messages.New(messages.H2481, path))
	case k.Moved != nil && !slices.Contains(v.loading, k.Key):
		return "", messages.At(s.Line(), 0, k.Moved)
	}
	return k.Key, nil
}
