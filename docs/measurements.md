# Ölçümler taşındı

Hız ve bellek ölçüleri artık [`measurements/`](measurements/README.md) klasöründe, işine göre beş dosyada tek satırdır: biçim, makine ve oturum kodları, yol kümeleri ve tazelik komutu (`tools/measurefresh`) klasörün README'sinde. Bu belgede ölçü yoktur; burada ölçü yazılmaz.

Bu belgenin 2026-11-17'ye kadarki satırları, kalın bölümleri ve "Eski" listesi klasöre satır olarak taşındı; her satırın Kaynak'ı `taşındı: docs/measurements.md@e23b0113:<satır>` ile eski yerini anar. Eskiyi artık ayrı liste değil komut söyler.

Satır olmayanlar (git geçmişinde, `docs/measurements.md@e23b0113`):

- Commit'i yazılmamış iki söz: `:293` (küçük modelde yorumlayıcının payı, DENETİM 2026-11-02) ve `:295`'in log-Mel farkı (Whisper, `jfk` sesi). Klasöre commit'siz ölçü girmez. Üçüncüsü (`:296`, dizüstünde Whisper small) yerine geçen 2026-11-08 dizüstü satırının Sonuç'unda not olarak durur.
- "Sıradaki ölçümler": Xeon'da CPU kıyası ve transformers bf16 temel ölçü setinin S2'sinde; G4'te üretim ve eğitimin yeniden ölçümü setin S1'inde ve S11'inde; GPU üretiminin 2026-11-02 gerilemesinin suçlu commit'i `docs/status.md`'nin "Bölümü olmayan işler"inde.
