# TR Yol Haritası

TR, yapay zekânın bütün işleri için bir programlama dilidir: dil, görüntü ya da ses, her türden modeli yazmak ve eğitmek; hazır modelleri çalıştırmak ve ince ayarla kendi işine uyarlamak; modeli robot gibi bir makinenin içinde çalıştırmak; uzaktaki bir modeli API ile kullanmak. Her karar bu işlerin hepsine göre verilir.

## Yöntem

Dil tanımı ([`docs/spec/`](spec/README.md)) baştan, bölüm bölüm yazılıyor. Her bölüm amaca göre yazılır ve iki denetimden geçer: kararlar doğru mu (Gemini araştırır, DENETİM doğrular) ve metin doğru mu (DENETİM). Sonra kararları uygulanır. Bu yol haritası yalnız yeniden yazılmış bölümlerden gelen işleri tutar. Eski yol haritası arşivdedir ([`docs/archive/roadmap-2026-10-27.md`](archive/roadmap-2026-10-27.md)); oradaki bir iş, ilgili bölüm yeniden yazılınca o bölümün kararlarına göre buraya gelir.

## 1. bölüm: Program

Taslak yazıldı ve DENETİM'den geçti (2026-10-27). Kararlar Gemini'nin araştırmasından sonra kesinleşir; uygulama ondan sonra başlar. Her maddenin bugünkü durumu ve yapılacakları [`status.md`](status.md)'dedir; her iş testiyle kapanır.

## Sıradaki

2. bölüm (sözcük yapısı), aynı yöntemle.

## Kilometre taşları

Kilometre taşı, dili gerçek bir işte sınayan ve bitiş ölçütü ölçülebilen bir hedeftir. Sıradakiler bir öncekinden sonra onayla eklenir.

### İlk kilometre taşı: hazır modelleri CPU'da ve GPU'da hızlı çalıştırmak

Ahmet, 2026-10-27. Hazır modeli çalıştırmak amacın en yaygın işidir; modeli sunmak, robotun içinde çalıştırmak ve ince ayar ona dayanır.

**Bitiş ölçütü:**
- Bir dil modeli (Llama ya da Qwen3 ailesi), bir görüntü modeli ve bir ses modeli (Whisper gibi konuşma tanıma) hazır ağırlıklarıyla çalışır ve özgün uygulamayla aynı sonucu verir.
- Hız aynı model, aynı hassasiyet ve aynı makinede ölçülür: GPU'da PyTorch ve llama.cpp'ye, CPU'da PyTorch CPU ve llama.cpp'ye karşı. Hedef oran ilk ölçümden sonra buraya yazılır.
- CPU'da hem x86 hem ARM64 işlemcide çalışır.
- Ağırlık 16 bit ve nicemlenmiş (8 ve 4 bit) olarak yüklenir.

**Bugün** (ölçüm 2026-10-22, ayrıntı arşivde): GPU'da üretim transformers'ın en hızlı yolunu geçti, llama.cpp f32'nin %63–83'ü; CPU'da llama.cpp'nin yaklaşık 1/5'i. Yalnız dil modelleri çalışıyor; evrişim ve ses işlemleri, nicemleme ve ARM vektör kernelleri yok.
