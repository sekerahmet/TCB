package gpu

import "encoding/binary"

// ============================================================================
// PTX benzetimi: atom (genel bellekte okuma-değiştirme-yazma)
// ============================================================================
//
// Benzetim iş parçacıklarını sırayla yürüttüğü için atom komutu tek okuma ve tek yazmadır;
// gerçek kartta aynı adrese gelen atomlar sıraya girer, sonuç sıradan bağımsızdır (min). Bu
// dilimde yalnız cihaz içi hata bayrağının komutu vardır: atom.global.min.u64 d, [adres], b
// (d eski değer; bellekte min(eski, b), işaretsiz).

// simAtomMin atom'un .min eki (mode2).
const simAtomMin = 1

// atomTerms atom'un işlenenleri: hedef yazmaç, bellek, değer.
func (a *simParser) atomTerms(k *simInst, t []string) error {
	if len(t) != 3 {
		return internalf("atom needs three operands")
	}
	if k.mode2 != simAtomMin || k.dtype != simU64 || k.space != spaceGlobal {
		return internalf("only atom.global.min.u64 is simulated")
	}
	if err := a.targetReg(k, t[0]); err != nil {
		return err
	}
	if err := a.memory(k, t[1]); err != nil {
		return err
	}
	v, err := a.term(t[2], k.dtype)
	k.o[0] = v
	return err
}

// atomic atom.global.min.u64'ü yürütür.
func (b *simBlock) atomic(t *simJob, k *simInst) error {
	d, err := b.addressSlice(t, k, 8, true)
	if err != nil {
		return err
	}
	old := binary.LittleEndian.Uint64(d)
	if v := b.read(t, &k.o[0]); v < old {
		binary.LittleEndian.PutUint64(d, v)
	}
	t.y[k.d] = old
	return nil
}
