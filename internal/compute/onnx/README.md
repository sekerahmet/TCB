# onnx

ONNX modelinin (protobuf, `onnx.proto3`) tensörlerini okur: ağırlıklar (`GraphProto.initializer`) adlarıyla, `Constant` düğümlerinin değerleri çıkış adlarıyla (dil tanımı 15.47). `ml` bunu `TR_load_tensors`'ta kullanır; Piper sesleri bu biçimdedir.

## Standartlar

**Bağımlılık yok.** Protobuf'un tel biçimi elle çözülür (`wire.go`): varint, sabit 32/64 bit, boylu alan; grup ve bilinmeyen tel türü geçersizdir. Yeni Go modülü yoktur. Denetim: `TestExternalDependencies` (internal/layers_test.go).

**Dosya güvenilmeyen girdidir.** Her boy içinde bulunduğu iletinin kalanıyla denetlenir (H6743); kesik dosya, 10 bayttan uzun varint ve beklenmeyen tel türü H6742'dir; tensör sayısı `MaxTensors`, eksen sayısı `MaxDims` ile sınırlıdır (H6744, H6745); veri boyu şeklin istediğine eşittir (H6747); dış veri (`external_data`) ve desteklenmeyen tür hatadır (H6747, H6746); yinelenen ad H6748'dir. Ayırma dosyadaki baytla orantılıdır: ağırlık verisi bildirilen boyda, varint'li tam sayı alanları (int32_data, int64_data) en çok 8 katı [hesap: 1 baytlık varint → 8 baytlık öğe]. Dosya bir kez, baştan sona okunur; `Decode` `size`'ı aşmaz. Denetim: `TestLoadOnnxRejectsBroken` (internal/stdlib/ml), `TestDecodeEveryPrefix`, `TestDecodeHoldsToSize`, `FuzzLoadOnnx`.

**Adsız ağırlık kullananın adını alır.** Dışa vericinin `onnx::` ile adlandırdığı tensör, onu girdi olarak kullanan ilk düğümün adıyla ve girdi sırasıyla adlanır (`/flow/flows.0/enc/in_layers.0/Conv:1`); adsız düğümde ya da kullanılmayanda ad değişmez. Denetim: `TestLoadOnnxAnonymousNamedByNode` (internal/stdlib/ml).

**Hesap okunmaz.** Düğümlerden yalnız adlar ve `Constant` değerleri okunur; alt çizgeler (If, Loop), seyrek ağırlıklar ve işlevler okunmaz.

**Bellek çağıranındır.** Ağırlığın raw_data'sına yeri `Options.Alloc` verir (`ml` büyük tensörü öbek dışı bloğa alır); eleman sınırını `Options.Check` denetler.

**`onnxtest` yalnızca testlerindir.** Depoya model dosyası girmez; testler dosyalarını elle kurar.
