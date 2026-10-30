# 6. VS Code Editör Eklentisi Kısayolları (Snippets)
`.tr` uzantılı bir dosya açtığınızda yazım dilinizdeki (`tr.writingLanguage`: Türkçe ya da İngilizce) kısayolu yazıp öneri listesinden şablonu seçerek `Tab` tuşuna basabilirsiniz; şablon her dilde dosyaya **kimlik yazımıyla** (`TR_func`, `TR_if` ...) kod ekler (ayrıntılar: [`editors/vscode/README.md`](../../editors/vscode/README.md#kod-şablonları)):
- `"fonksiyon"` (İngilizce: `"func"`) $\rightarrow$ `TR_func` fonksiyon bloğu oluşturur
- `"eğer"` (`"if"`) $\rightarrow$ `TR_if ... TR_else` bloğu oluşturur
- `"eşle"` (`"match"`) $\rightarrow$ `TR_match` durum eşleştirme bloğu oluşturur
- `"filtrele"` / `".filtrele"`, `"haritala"` / `".haritala"` (`"filter"`, `"map"`) $\rightarrow$ Okla `.TR_filter((x) => ...)` / `.TR_map((x) => ...)` yazar
- `"indirge"` / `".indirge"` (`"reduce"`) $\rightarrow$ `.TR_reduce(başlangıç, (birikim, x) => ...)` yazar
- `"döngü"` (`"for"`) $\rightarrow$ `TR_for i TR_in 1..=10` bloğu oluşturur
- `"liste_üret"` (`"list_comp"`) $\rightarrow$ Tek satırda `[x * x TR_for x TR_in 1..=10 TR_if x % 2 == 0]` liste üreteci oluşturur
- `"yapı"` (`"struct"`) $\rightarrow$ `TR_struct` ve metot şablonu oluşturur
- `"dene"` (`"try"`) $\rightarrow$ `TR_try ... TR_catch` hata yakalama bloğu oluşturur
- `"Tensör"` / `"türev"` (`"Tensor"` / `"grad"`) $\rightarrow$ Tensörlerle küçük bir eğitim döngüsü oluşturur (`TR_Tensor(..., TR_requires_grad: TR_true)`, `TR_AdamW`, `iyileştirici.TR_zero_grad()`, `TR_backward`, `iyileştirici.TR_step()`)
