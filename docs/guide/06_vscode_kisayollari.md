# 6. VS Code Editör Eklentisi Kısayolları (Snippets)
`.tr` uzantılı bir dosya açtığınızda aşağıdaki kısayolları yazıp `Tab` tuşuna basabilirsiniz:
- `fonk` / `TR_func` $\rightarrow$ Fonksiyon bloğu oluşturur
- `TR_if` $\rightarrow$ `TR_if ... TR_else` bloğu oluşturur
- `TR_match` $\rightarrow$ `TR_match` durum eşleştirme bloğu oluşturur
- `.TR_filter` / `.TR_map` $\rightarrow$ Okla `.TR_filter((x) => ...)` / `.TR_map((x) => ...)` yazar
- `.TR_reduce` $\rightarrow$ `.TR_reduce(başlangıç, (birikim, x) => ...)` yazar
- `TR_for` $\rightarrow$ `TR_for i TR_in 1..=10` bloğu oluşturur
- `liste_üret` $\rightarrow$ Tek satırda liste üreteci oluşturur
- `TR_struct` $\rightarrow$ `TR_struct` ve metot şablonu oluşturur
- `TR_try` $\rightarrow$ `TR_try ... TR_catch` hata yakalama bloğu oluşturur
- `TR_Tensor` / `türev` $\rightarrow$ Tensörlerle küçük bir eğitim döngüsü oluşturur (`TR_Tensor(..., TR_requires_grad: TR_true)`, `TR_AdamW`, `iyileştirici.TR_zero_grad()`, `TR_backward`, `iyileştirici.TR_step()`)
