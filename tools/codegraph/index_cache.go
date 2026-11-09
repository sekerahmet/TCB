package main

// Çağıran yürüyüşünün iki önbelleği (hız işi). Yürüyüş (codeindex.WalkCallers) her adımda bir düğümün çağıranlarını
// ve paket klasörünü sorar; tests-for aynı düğümü yüzlerce kez sorar. Önbellek indeksin içindedir: indeks kurulduktan
// sonra kenarları değişmez, başka bir graf başka bir indekstir (paket düzeyindeki bir önbellek iki grafı karıştırırdı).

// callersKey çağıran önbelleğinin anahtarıdır: aynı düğümün iki yürüyüşü farklı kenar görür (callersOf argümanı izler,
// nameCallersOf izlemez).
type callersKey struct {
	id          string
	throughArgs bool
}

// callers id'nin çağıranlarıdır (buildCallers), düğüm ve yürüyüş başına bir kez kurulur. Dönen dilim paylaşılır ve
// salt okunurdur; kapasitesi boyuna eşit, append kopyalar (TestCachedCallersStayAsBuilt).
func (ix *index) callers(id string, throughArgs bool) []string {
	k := callersKey{id, throughArgs}
	if out, ok := ix.callerMemo[k]; ok {
		return out
	}
	if ix.callerMemo == nil {
		ix.callerMemo = map[callersKey][]string{}
	}
	out := ix.buildCallers(id, throughArgs)
	out = out[:len(out):len(out)]
	ix.callerMemo[k] = out
	return out
}

// pkgOf düğümün paket klasörüdür (buildPkgOf), düğüm başına bir kez.
func (ix *index) pkgOf(id string) string {
	if p, ok := ix.pkgMemo[id]; ok {
		return p
	}
	if ix.pkgMemo == nil {
		ix.pkgMemo = map[string]string{}
	}
	p := ix.buildPkgOf(id)
	ix.pkgMemo[id] = p
	return p
}
