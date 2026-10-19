package gpu

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ============================================================================
// PTX benzetimi: vektörel erişimler ve cp.async
// ============================================================================
//
// Vektörel ld/st (.v2/.v4) tek erişimdir: adres toplam boya (ör. 16 bayt) hizalı olmalı.
//
// cp.async donanımdaki gibi eşzamansız ele alınır: kopya iş parçacığının bekleyen
// listesine konur ve ancak o iş parçacığı cp.async.wait_group ile beklediğinde yapılır
// (kaynak o anda okunur, hedef o anda yazılır ve yarış dedektörüne yazma olarak
// bildirilir). Böylece beklemeden ya da beklemeden sonraki engel olmadan okunan kopya
// hedefi benzetimde de eski değeri verir ve sınamada yanlış sonuç olarak görünür. Bekleyen
// kopyası kalan bir iş parçacığının bitmesi hatadır.

// simCpJob bekleyen bir eşzamansız kopya.
type simCpJob struct {
	group      int32
	smAddr     int
	source     uint64
	size, read int
}

// vector vektör işlenenini (yer tutucu "@V") komutun yazmaç ya da terim listesine çözer.
func (a *simParser) vector(k *simInst, t, list []string) error {
	if list == nil {
		if k.vec != 0 {
			return fmt.Errorf("vektörel komutta vektör işleneni yok")
		}
		return nil
	}
	if int(k.vec) != len(list) || (k.op != simLd && k.op != simSt) {
		return fmt.Errorf("vektör işleneni (%d eleman) komutla uyuşmuyor", len(list))
	}
	if k.dtype != simF32 && k.dtype != simU32 {
		return fmt.Errorf("vektörel erişim yalnızca 32 bitlik türlerle")
	}
	for i, s := range list {
		if k.op == simLd {
			y, ok := a.reg[s]
			if !ok {
				return fmt.Errorf("tanımsız hedef yazmaç %s", s)
			}
			k.vy[i] = y
			continue
		}
		v, err := a.term(s, k.dtype)
		if err != nil {
			return err
		}
		k.vo[i] = v
	}
	place := 0
	if k.op == simSt {
		place = 1
	}
	if len(t) != 2 || t[place] != "@V" {
		return fmt.Errorf("vektör işleneninin yeri yanlış")
	}
	return nil
}

// cpTerms cp.async.cg.shared.global [hedef], [kaynak], boy[, kaynak_boyu].
func (a *simParser) cpTerms(k *simInst, t []string) error {
	if len(t) != 3 && len(t) != 4 {
		return fmt.Errorf("cp.async 3 ya da 4 işlenen ister")
	}
	k.space = spaceShared
	if err := a.memory(k, t[0]); err != nil {
		return err
	}
	source := strings.TrimSuffix(strings.TrimPrefix(t[1], "["), "]")
	if source == t[1] {
		return fmt.Errorf("cp.async kaynağı bellek işleneni olmalı")
	}
	if i := strings.IndexByte(source, '+'); i >= 0 {
		var n int64
		if _, err := fmt.Sscan(source[i+1:], &n); err != nil {
			return err
		}
		k.offset2, source = n, source[:i]
	}
	var err error
	if k.baseB, err = a.term(source, simU64); err != nil {
		return err
	}
	if k.o[0], err = a.term(t[2], simU32); err != nil {
		return err
	}
	if k.o[0].variant != tmConst || (k.o[0].s != 4 && k.o[0].s != 8 && k.o[0].s != 16) {
		return fmt.Errorf("cp.async boyu 4, 8 ya da 16 sabiti olmalı")
	}
	k.o[1] = k.o[0]
	if len(t) == 4 {
		k.o[1], err = a.term(t[3], simU32)
	}
	return err
}

// vecLoad vektörel ld.
func (b *simBlock) vecLoad(t *simJob, k *simInst) error {
	n := int(k.vec)
	d, err := b.addressSlice(t, k, 4*n, false)
	if err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		t.y[k.vy[i]] = uint64(binary.LittleEndian.Uint32(d[4*i:]))
	}
	return nil
}

// vecStore vektörel st.
func (b *simBlock) vecStore(t *simJob, k *simInst) error {
	if k.space == spaceParam {
		return fmt.Errorf("parametre alanına yazılamaz")
	}
	n := int(k.vec)
	d, err := b.addressSlice(t, k, 4*n, true)
	if err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(d[4*i:], uint32(b.read(t, &k.vo[i])))
	}
	return nil
}

// cpAdd eşzamansız kopyayı iş parçacığının bekleyen listesine koyar.
func (b *simBlock) cpAdd(t *simJob, k *simInst) error {
	size := int(k.o[0].s)
	read := int(uint32(b.read(t, &k.o[1])))
	if read > size {
		return fmt.Errorf("cp.async kaynak boyu %d kopya boyundan (%d) büyük", read, size)
	}
	sm := int64(uint32(b.read(t, &k.base2))) + k.offset
	if sm < 0 || int(sm)+size > len(b.shared) || sm%int64(size) != 0 {
		return fmt.Errorf("cp.async hedefi geçersiz ya da hizasız: %d", sm)
	}
	source := b.read(t, &k.baseB) + uint64(k.offset2)
	if read > 0 && source%uint64(size) != 0 {
		return fmt.Errorf("cp.async kaynağı hizasız: 0x%x", source)
	}
	t.cp = append(t.cp, simCpJob{group: t.cpGroup, smAddr: int(sm), source: source, size: size, read: read})
	return nil
}

// cpWait en yeni n grup dışındaki kopyaları yapar.
func (b *simBlock) cpWait(t *simJob, n int32) error {
	rest := t.cp[:0]
	for _, c := range t.cp {
		if c.group >= t.cpGroup-n {
			rest = append(rest, c)
			continue
		}
		if err := b.race(t, c.smAddr, c.size, true); err != nil {
			return err
		}
		target := b.shared[c.smAddr : c.smAddr+c.size]
		clear(target)
		if c.read > 0 {
			d, err := b.mem.slice(c.source, c.read)
			if err != nil {
				return fmt.Errorf("cp.async kaynağı: %v", err)
			}
			copy(target, d)
		}
	}
	t.cp = rest
	return nil
}

// cpRun cp.async ailesinin komutlarını yürütür.
func (b *simBlock) cpRun(t *simJob, k *simInst) error {
	switch k.op {
	case simCp:
		return b.cpAdd(t, k)
	case simCpGroup:
		t.cpGroup++
		return nil
	}
	return b.cpWait(t, int32(k.o[0].s))
}
