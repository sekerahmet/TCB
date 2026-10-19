package tokenize

// idInfo bir kimliğin token metni ve çözme bilgileridir.
type idInfo struct {
	token   string
	byteVal []byte // yalnızca bayt düzeyi çözücüde: tokenın baytları
	var_    bool   // kimlik sözlükte ya da ek tokenlarda var
	special bool   // özel token (çözmede atlanabilir)
}

// sparsityLimit: en büyük kimlik bundan ve girdi sayısının 4 katından
// büyükse kimlik tablosu seyrek haritayla tutulur.
const sparsityLimit = 1 << 16

// tokID id'nin bilgisini verir (yoksa nil).
func (p *Tokenizer) tokID(id int) *idInfo {
	if p.sparse != nil {
		if id < 0 || id > 1<<31-1 {
			return nil
		}
		i, ok := p.sparse[int32(id)]
		if !ok {
			return nil
		}
		return &p.idValues[i]
	}
	if id < 0 || id >= len(p.idValues) || !p.idValues[id].var_ {
		return nil
	}
	return &p.idValues[id]
}

// buildIDTable kimlik -> token tablosunu kurar.
func (p *Tokenizer) buildIDTable(vocab map[string]int32, nrmContent map[int]string, specialContent map[string]bool) {
	largest := -1
	for _, id := range vocab {
		largest = max(largest, int(id))
	}
	for _, e := range p.extras {
		largest = max(largest, e.ID)
	}
	input := len(vocab) + len(p.extras)
	put := func(id int) *idInfo {
		if p.sparse == nil {
			return &p.idValues[id]
		}
		if i, ok := p.sparse[int32(id)]; ok {
			return &p.idValues[i]
		}
		p.sparse[int32(id)] = int32(len(p.idValues))
		p.idValues = append(p.idValues, idInfo{})
		return &p.idValues[len(p.idValues)-1]
	}
	if largest+1 > sparsityLimit && largest+1 > 4*input {
		p.sparse = make(map[int32]int32, input)
		p.idValues = make([]idInfo, 0, input)
	} else {
		p.idValues = make([]idInfo, largest+1)
	}
	for tok, id := range vocab {
		k := put(int(id))
		k.token, k.var_ = tok, true
	}
	for _, e := range p.extras {
		k := put(e.ID)
		k.token, k.var_ = e.Content, true
		if s, ok := nrmContent[e.ID]; ok {
			k.token = s
		}
	}
	for i := range p.idValues {
		k := &p.idValues[i]
		if !k.var_ {
			continue
		}
		k.special = specialContent[k.token]
		if p.byteDecoder {
			k.byteVal = tokenBytes(k.token)
		}
	}
}
