package tokenize

import "maps"

// ============================================================================
// Düz kodlama: eklenmiş parçaların kimliğini üretmeyen model kopyası
// ============================================================================
//
// DontParseSpecial ile metindeki özel parça yazımı eşleşmez (bkz. addedMatcher); ama modelin kendi
// bölütlemesi de o parçaya varabilir: Unigram dağarcığı "<s>"/"</s>"'yi parça olarak taşır ve
// metindeki "</s>"'yi dizi sonu kimliğine böler. Düz kopya bu kimlikleri bölütlemede hiç
// kullanmaz: Unigram'da kafese girmezler, BPE'de onlara varan birleştirme ve karakter kimliği
// yoktur, WordPiece ve WordLevel'da sözlük eşleşmesi onları vermez. Metin geri kalan parçalarla
// (bayt yedeği varsa baytlarla) kodlanır; kodlanamayan bölüm bilinmeyen kimliğini alır: bilinmeyen
// sözcüğün kimliği olarak çıkar, rol üretmez. Kopya sözlüğü ve kuralları özgünle paylaşır; önbelleği
// ayrıdır (aynı parça iki kipte başka kimlikler verebilir).

// barrer bazı kimlikleri bölütlemede kullanmayan bir kopya kurabilen modeldir.
type barrer interface {
	barred(ids map[int32]bool) Model
}

// plainModel modelin ids'i bölütlemede kullanmayan kopyasını verir; ids boşsa modelin kendisini.
// Bilinmeyen kimliği kümede olsa da kopyada kullanılır: rol üretmez (HF de metindeki "<unk>"'yi
// ona böler), onu yasaklamak yalnız aynı kimliği daha çok parçayla verirdi.
func plainModel(m Model, ids map[int32]bool) Model {
	if unk := unknownID(m); ids[unk] {
		ids = maps.Clone(ids)
		delete(ids, unk)
	}
	if b, ok := m.(barrer); ok && len(ids) > 0 {
		return b.barred(ids)
	}
	return m
}

// unknownID modelin bilinmeyen kimliğidir (yoksa -1).
func unknownID(m Model) int32 {
	switch x := m.(type) {
	case *bpeModel:
		return x.unknown
	case *unigram:
		return x.unknown
	case *wordPiece:
		return x.unknown
	case *wordLevel:
		return x.unknown
	}
	return -1
}

// cacheLike o'nun kapasitesinde boş bir önbellek verir (o nil ise nil).
func cacheLike(o *cache) *cache {
	if o == nil {
		return nil
	}
	return newCache(o.pieceLimit * cacheShardCount)
}

// barTable ids'ten birini kuran kural olmadan tabloyu verir (hiçbiri yoksa tablonun kendisini).
func barTable(t *mergeTable, ids map[int32]bool) *mergeTable {
	kept := make(map[uint64]mergeRule)
	removed := false
	for _, s := range t.slots {
		if s.key == emptyKey {
			continue
		}
		if ids[s.value.fresh] {
			removed = true
			continue
		}
		kept[s.key] = s.value
	}
	if !removed {
		return t
	}
	return newMergeTable(kept)
}

// barArray dizideki ids kimliklerini -1 yapar.
func barArray(a []int32, ids map[int32]bool) {
	for i, id := range a {
		if ids[id] {
			a[i] = -1
		}
	}
}

func (b *bpeModel) barred(ids map[int32]bool) Model {
	c := *b
	c.bar = ids
	c.table = barTable(b.table, ids)
	barArray(c.lowID[:], ids)
	barArray(c.byteCharID[:], ids)
	barArray(c.fallbackID[:], ids)
	c.charID = make(map[rune]int32, len(b.charID))
	for r, id := range b.charID {
		if !ids[id] {
			c.charID[r] = id
		}
	}
	c.cache = cacheLike(b.cache)
	return &c
}

func (u *unigram) barred(ids map[int32]bool) Model {
	c := *u
	c.bar = make([]bool, len(u.tokenList))
	for id := range ids {
		if id >= 0 && int(id) < len(c.bar) {
			c.bar[id] = true
		}
	}
	barArray(c.fallbackID[:], ids)
	c.cache = cacheLike(u.cache)
	return &c
}

func (w *wordPiece) barred(ids map[int32]bool) Model {
	c := *w
	c.bar = ids
	return &c
}

func (w *wordLevel) barred(ids map[int32]bool) Model {
	c := *w
	c.bar = ids
	return &c
}

// usable kimliğin bu kopyada bölütlemede kullanılıp kullanılamayacağını söyler.
func (u *unigram) usable(id int32) bool {
	return id >= 0 && (u.bar == nil || !u.bar[id])
}
