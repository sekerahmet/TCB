# TR Yol Haritası

TR, yapay zekânın bütün işleri için bir programlama dilidir: dil, görüntü ya da ses, her türden modeli yazmak ve eğitmek; hazır modelleri çalıştırmak ve ince ayarla kendi işine uyarlamak; modeli robot gibi bir makinenin içinde çalıştırmak; uzaktaki bir modeli API ile kullanmak. Her karar bu işlerin hepsine göre verilir.

## Yöntem

Dil tanımı ([`docs/spec/`](spec/README.md)) baştan, bölüm bölüm yazılıyor. Her bölüm amaca göre yazılır ve iki denetimden geçer: kararlar doğru mu (Gemini araştırır, DENETİM doğrular) ve metin doğru mu (DENETİM). Sonra kararları uygulanır. Bu yol haritası yalnız yeniden yazılmış bölümlerden gelen işleri tutar. Eski yol haritası arşivdedir ([`docs/archive/roadmap-2026-10-27.md`](archive/roadmap-2026-10-27.md)); oradaki bir iş, ilgili bölüm yeniden yazılınca o bölümün kararlarına göre buraya gelir.

## 1. bölüm: Program

Yazıldı ve iki denetimden geçti (2026-10-27). Uygulanacak kararlar; her birinin bugünkü durumu [`status/01_program.md`](status/01_program.md)'dedir, her biri testle kapanır:

1. **Statik tip ve tip çıkarımı (1.2).** Büyük tasarım: denetleyiciye tip çıkarımı; kitaplığın, örneklerin ve testlerin imzaları; "olabilir" tipi; tensörün tür ve şekil bilgisinin tipe ne kadar gireceği.
2. **Modülde ve test dosyasında yalnız tanım, ilk değer sabit ifade (1.1).**
3. **Çalışmadan önce bütün program denetlenir (1.3):** modül hatasında program hiç başlamaz, `TR_cmd_debug` da denetler, `TR_kw_import` yalnız dosyanın başında.
4. **Okunmayan argüman çalışmadan önce kullanım hatasıdır; ROS 2'nin `--ros-args` blokları programın değildir (1.4).**
5. **Durdurma (1.5, 1.7, 1.9):** durum değiştirmeyen iş ve her bekleme bırakılır, alt sürece istek iletilir; süreç isteğin sinyaliyle biter; `tr` aracının kullanım hatası 2.
6. **Durdurmayı üstlenme (1.6):** `TR_fn_handle_stop`, `TR_fn_stop_requested`; kontrol noktası adım sınırında kaydeder, bitişte kaydetmez.
7. **Programın bitişi (1.10–1.15):** `TR_fn_exit`, çıkış kodu aralığı, `tr test`'te çıkış, temizlik sırasındaki çıkış ve hata, bitişte gelen Ctrl+C.

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
