# 8. Kod Biçimlendirme (`tr biçimle`)

`TR_cmd_format` kodu tek bir standart biçime getirir:

- 4 boşluk girinti; `{` her zaman aynı satırda, `TR_else` / `TR_catch` kapanan `}` ile aynı satırda,
- ikili operatörlerin iki yanında ve virgülden sonra birer boşluk (aralıklar bitişik: `1..=10`; sözlükler `{"a": 1}`, sondaki bloklar `{ o > 2 }`),
- satır sonunda boşluk yok, art arda en fazla bir boş satır, dosya sonunda tek satır sonu.

**Yorumlar korunur**, metin sabitlerine bayt bayt dokunulmaz ve satır yapısı (hangi kodun hangi satırda olduğu) değişmez. Biçimlendirici sonucu yeniden ayrıştırıp özgün kodla karşılaştırır: anlam değişecek olursa dosyaya **dokunmaz** ve bunu bildirir. Sözdizimi hatası olan dosyalar biçimlenmez. Aynı dosyayı iki kez biçimlemek bir kez biçimlemekle aynıdır. Kodlu biçimdeki dosya (ilk satırı `//! tr kodlu 2 dil=<dil>`, bkz. `TR_cmd_encode`) kodlu biçimde biçimlenir: kodlar olduğu gibi kalır, anlam denetimi kodları kodlarıyla, kullanıcının adlarını metinleriyle karşılaştırır.

```text
> tr biçimle --kontrol .
biçimlenmeli: hesap.tr
3 dosyadan 1 tanesi biçimlenmeli. Düzeltmek için: tr biçimle <dosya|klasör>
> tr biçimle .
biçimlendi: hesap.tr
3 dosya incelendi, 1 dosya biçimlendi.
```
