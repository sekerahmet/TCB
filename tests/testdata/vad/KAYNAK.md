# Konuşma algılama başvuru dosyası

`tests/vad_test.tr` bu dosyayı okur (25.11).

- **`kucuk_olasilik.bin`:** 12 çerçevenin konuşma olasılığı, küçük endian `float32`. Ağırlıklar yayımlanmış bir modelden gelmez: `tests/vad_test.tr`'nin `değerler` formülüyle, `adlar` sırasıyla ve ölçeğiyle kuruldu; çerçeveler `formül_çerçevesi`'dir. Değerleri PyTorch 2.14.0'ın `Conv1d`, `LSTMCell` ve `F.pad(..., "reflect")`'i, Silero VAD v6.2.3'ün `tinygrad_model.py`'sindeki sırayla hesapladı. Kaydı üreten betik depoda değildir.

Silero'nun ağırlık dosyası depoda yoktur; kullanıcı <https://github.com/snakers4/silero-vad>'dan alır: `src/silero_vad/data/silero_vad_16k.safetensors`, v6.2.3.

**Lisans:** değerler bu deponun lisansındadır. Değerleri hesaplayan yazılım PyTorch'tur (<https://github.com/pytorch/pytorch>, BSD-3-Clause). `lib/vad.tr`'nin mimarisi Silero VAD'dan uyarlanmıştır (<https://github.com/snakers4/silero-vad>, v6.2.3):

```text
MIT License

Copyright (c) 2020-present Silero Team

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
