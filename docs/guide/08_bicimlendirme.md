# 8. Kod Biçimlendirme (`trlang format`)

`TR_cmd_format` kodu tek bir standart biçime getirir:

- 4 boşluk girinti; `{` her zaman aynı satırda, `TR_else` / `TR_catch` kapanan `}` ile aynı satırda,
- ikili operatörlerin iki yanında ve virgülden sonra birer boşluk (aralıklar bitişik: `1..=10`; sözlükler `{"a": 1}`, sondaki bloklar `{ X @ w }`),
- satır sonunda boşluk yok, art arda en fazla bir boş satır, dosya sonunda tek satır sonu.

**Yorumlar korunur**, metin sabitlerine bayt bayt dokunulmaz ve satır yapısı (hangi kodun hangi satırda olduğu) değişmez. Biçimlendirici sonucu yeniden ayrıştırıp özgün kodla karşılaştırır: anlam değişecek olursa dosyaya **dokunmaz** ve bunu bildirir. Sözdizimi hatası olan dosyalar biçimlenmez. Aynı dosyayı iki kez biçimlemek bir kez biçimlemekle aynıdır. Dosya (ilk satırı `# TR 4`) kimlikleriyle biçimlenir: kimlikler olduğu gibi kalır, anlam denetimi kodları kodlarıyla, kullanıcının adlarını metinleriyle karşılaştırır.

```text
> trlang format --check .
biçimlenmeli: hesap.tr
3 dosyadan 1 tanesi biçimlenmeli. Düzeltmek için: trlang format <dosya|klasör>
> trlang format .
biçimlendi: hesap.tr
3 dosya incelendi, 1 dosya biçimlendi.
```
