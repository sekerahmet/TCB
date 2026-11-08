package tensor

// Yaprağın "gradyan eklendi" bilgisi (dil tanımı 15.34). Geri yayılım gradyan eklediği her
// yaprakta bilgiyi kurar; iyileştiricinin adımı onu tüketir (stepper adımlayan iyileştiricidir,
// bir sonraki geri yayılıma kadar), gradyanı sıfırlamak ve iyileştiricinin durumunu yüklemek
// (geri dönüş, 20.14) siler. Adım bilgisi olmayan parametreyi atlar: sıfırlanmış gradyan
// "eklenmiş" sayılmaz, bu yüzden hiç gradyan almamış parametrenin ağırlığı, momentleri,
// sayacı ve çürümesi değişmez.

// markAdded başarıyla biten geri yayılımın gradyan eklediği yaprakları işaretler.
func markAdded(leaves []*node) {
	for _, n := range leaves {
		if n.grad != nil {
			n.added, n.stepper = true, nil
		}
	}
}

// GradAdded son adımdan (ya da gradyanı sıfırlamaktan) bu yana geri yayılımın t'ye gradyan
// ekleyip eklemediğidir.
func (t *Tensor) GradAdded() bool { return t.n != nil && t.n.added }

// GradStepper bu geri yayılımın t'ye eklediği gradyanı adımlamış iyileştiricidir (yoksa nil).
func (t *Tensor) GradStepper() Optimizer {
	if t.n == nil {
		return nil
	}
	return t.n.stepper
}

// ForgetGrads parametrelerin "gradyan eklendi" bilgisini siler (yüklenen durum, 20.14).
func ForgetGrads(params []*Tensor) {
	for _, p := range params {
		if p.n != nil {
			p.n.added, p.n.stepper = false, nil
		}
	}
}

// DropGrad yaprağın gradyanını ve "gradyan eklendi" bilgisini siler: .TR_grad TR_none olur
// (kontrol noktasının yüklemesi ve geri dönüşü, 20.14; gradyanlar kaydedilmez, 20.3). Daha önce
// alınmış bir gradyan değeri depoyu tutar, parametreden kopar (Freeze gibi).
func (t *Tensor) DropGrad() {
	if t.n != nil {
		t.n.grad, t.n.added, t.n.stepper = nil, false, nil
	}
}

// IsParameter t'nin türev izlemek için kurulmuş ya da açılmış bir yaprak olduğudur (15.33):
// dondurmak onu parametre olmaktan çıkarmaz (düğümü kalır); hiç türev izlememiş tensörün
// (tablo, tampon) düğümü yoktur. İşlemin türev izleyen sonucu, geri yayılımdan sonra da, ara
// sonuçtur: onu çağıran ayırır (ml'nin intermediate'i).
func (t *Tensor) IsParameter() bool { return t.n != nil && t.IsLeaf() }

// OwnsStorage t'nin depoyu başka bir tensörle paylaşmadığıdır: görünüm, Detach'in ve Grad'ın
// sonucu paylaşır (15.34'te türev izlemeyi açmak yalnız kendi deposu olan yaprakta geçerlidir).
func (t *Tensor) OwnsStorage() bool {
	return !t.shared && t.offset == 0 && t.IsContiguous() && t.storage.Len() == t.Numel()
}

// Freeze yaprağın türev izlemesini kapatır ve gradyanını siler (15.34): daha önce alınmış bir
// gradyan değeri depoyu tutar, parametreden kopar.
func (t *Tensor) Freeze() {
	t.SetRequiresGrad(false)
	t.DropGrad()
}
