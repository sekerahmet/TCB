# 6. VS Code Editör Eklentisi Kısayolları (Snippets)
`.tr` uzantılı bir dosya açtığınızda aşağıdaki kısayolları yazıp `Tab` tuşuna basabilirsiniz:
- `fonk` / `TR_kw_func` $\rightarrow$ Fonksiyon bloğu oluşturur
- `TR_kw_if` $\rightarrow$ `eğer ... değilse` bloğu oluşturur
- `TR_kw_match` $\rightarrow$ `TR_kw_match` durum eşleştirme bloğu oluşturur
- `TR_mem_filter` / `TR_mem_map` $\rightarrow$ Okla `.TR_mem_filter((x) => ...)` / `.TR_mem_map((x) => ...)` yazar
- `TR_mem_reduce` $\rightarrow$ `.TR_mem_reduce(başlangıç, (birikim, x) => ...)` yazar
- `TR_kw_for` $\rightarrow$ `TR_kw_for i TR_kw_in 1..=10` bloğu oluşturur
- `liste_üret` $\rightarrow$ Tek satırda liste üreteci oluşturur
- `TR_kw_struct` $\rightarrow$ `TR_kw_struct` ve metot şablonu oluşturur
- `TR_kw_try` $\rightarrow$ `dene ... yakala` hata yakalama bloğu oluşturur
- `TR_type_Tensor` / `türev` $\rightarrow$ Tensörlerle küçük bir eğitim döngüsü oluşturur (`TR_type_Tensor(..., TR_arg_requires_grad: TR_kw_true)`, `TR_fn_AdamW`, `iyileştirici.TR_mem_zero_grad()`, `TR_fn_backward`, `iyileştirici.TR_mem_step()`)
