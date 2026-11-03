# cmd/tr/e2e

`tr` ikilisini derleyip gerçek süreç olarak çalıştıran uçtan uca testler: komutlar, çıkış kodları, sinyaller, belgeler ve VS Code eklentisi. Bu klasörde üretim kodu yoktur; bütün dosyalar davranış testidir, ortak yardımcılar `helpers_test.go`'dadır.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`); bu klasörde üretim kodu olmadığından hepsi davranış testidir. Birden çok dosyayı kesen davranış testleri:

- `behavior_arguments_test.go`: betiğe ve derlenmiş uygulamaya komut satırı argümanlarının iletilmesi; argüman verilip okunmayan programın kullanım hatası (çıkış 2), yalnız `-h`, başlatıcı argümanlarının (`--ros-args`, `--local-rank`) ayrılması
- `behavior_exit_test.go`: `TR_exit` ve çıkış kodları (ertele, yakala, görev, aralık, ertelenen işin ve görevin hatası), `tr`nin kullanım hatasının 2 olması, çalışmadan önce modüllerin tek yoldan denetlenmesi
- `behavior_stop_test.go`: durdurma isteğinin süreçteki yolu: üstlenen programın kendi sınırında bitmesi ve çıkış kodu, biterken gelen ilk isteğin ertelenen işleri kesmemesi, ertelenen işlerdeki beklemelerin bırakılmaması
- `behavior_checkpoint_resume_test.go`: kontrol noktasından devam eden eğitimin kesintisizle bit düzeyinde aynı sonucu vermesi
- `behavior_coded_files_test.go`: kodlu dosyalar ve görünüm (17.7): `TR_cmd_show` komutu (dosya ve standart giriş), görünüm dosyasının çalışmaması, İngilizce görünüm, başlıksız dosyanın ve kişinin dilinde yazılan kabuk satırının çalışmaması, kullanıcı adlarının kişinin diline bağlı olmaması
- `behavior_colab_cells_test.go`: `tools/gpu/COLAB.md` hücrelerindeki satır içi Python'un ve GPU betiklerinin sözdizimi (Colab'a gitmeden önce)
- `behavior_trt_python_test.go`: TR tensör dosyasının (.trt) Python'da yazılıp TR'de okunması ve tersi (`tools/gpu/trt_io.py`)
- `behavior_ready_model_test.go`: `tr/pretrained` ile yüklenen Hugging Face biçimindeki küçük Llama ve Qwen3 modellerinin transformers'la aynı logitleri vermesi (transformers varsa)
- `behavior_target_model_stability_test.go`: hedef modelin eğitiminin bir NaN'de son kontrol noktasına dönüp toplu işleri atlayarak sürmesi, kontrol noktası yoksa açık hatayla durması (Evre 3 bitiş ölçütü)
- `behavior_target_model_outputs_test.go`: hedef modelin eğitiminin tek adım sayısında da hız dosyasını, kayıp kaydını ve son ağırlıkları yazması; kaydedilen ağırlıkların `--step 0` ile değerlendirilince son doğrulama kaybını vermesi
- `behavior_target_model_parity_test.go`: hedef modelin TR karşılığının PyTorch sürümüyle aynı başlangıçtan float32'de adım adım aynı kaybı ve gradyan normunu vermesi (PyTorch varsa)
- `behavior_check_json_test.go`: `tr check --json` (dil tanımı 2.13): her alanın varlığı ve biçimi, konumun kaynakta tam o metni göstermesi (`end` dışlayıcı, sütun karakter), iki dilde aynı kod/konum/düzeltme ve farklı ileti, `applicable` düzeltmenin dosyayı hatasız bırakması, ilgili konum, aktarılan modülün dosyası, kullanım hataları (çıkış 2) ve dosyadan sonra yazılan `--json`'ın hatası
- `behavior_check_json_gate_test.go`: tipsiz parametre, alan ve dönüşün (6.1, 7.1) `tr check --json` düzeltmeleri: varsayılandan ve dönüşlerden çıkan tip uygulanabilir, hepsi uygulanınca dosya hatasız geçer; çağrılardan tahmin uygulanamaz
- `behavior_commands_test.go`: komutların (`TR_cmd_run`, `TR_cmd_check`, `TR_cmd_test`, `TR_cmd_format`, `TR_cmd_build`, `TR_cmd_dap`, REPL) uçtan uca davranışı, hata iletileri ve çıkış kodları
- `behavior_doc_code_blocks_test.go`: belgelerdeki TR kod bloklarının ve eklenti şablonlarının derlenmesi
- `behavior_doc_links_test.go`: belgelerdeki bağlantıların ve başlık çapalarının geçerliliği
- `behavior_file_formats_language_test.go`: dosya biçimlerinin (kontrol noktası, günlük) yazım diline göre değişmemesi: TR_LANG=en ile aynı dosya adları ve anahtarlar
- `behavior_index_test.go`: `TR_cmd_index` ve alt komutları (`TR_sub_definition`, `TR_sub_references`, `TR_sub_tested_by`, `TR_sub_impact`, `TR_sub_rename`): sonuçlar, kimlik adıyla alt komutlar, üst düzeyde komut olmaması, çıkış kodları 3/4, `--json`, proje kökü bulma ve önbellek
- `behavior_example_errors_test.go`: hata örneklerinin (`examples/tr/hatalar`) beklenen ileti kodunu üretmesi, her gösterim dilinde çıkış kodu 1 ve denetimde düşen örnekte hiçbir satırın çalışmaması (dil tanımı 1.3)
- `behavior_program_test.go`: programın tek dosya olması: en dıştaki deyimlerin yukarıdan aşağı çalışması, `main` adlı fonksiyonun kendiliğinden çağrılmaması (dil tanımı 1.1)
- `behavior_library_coverage_test.go`: `lib/*.tr`'nin kullanıcıya açık adlarının (üst düzey ve metot) bir örnekte geçmesi; geçmeyenler gerekçeli, yalnızca küçülen listede
- `behavior_examples_coverage_test.go`: dilin her canlı adının bir örnekte geçmesi (kodlarla) ve komut satırı adlarının `examples/README.md`'de bulunması
- `behavior_examples_test.go`: öğrenme yolundaki örneklerin (`examples/tr`) numaralı ve kodlu olması, her örneğin iki gösterim dilinde (`TR_LANG=tr`, `TR_LANG=en`) uyarısız çalışıp aynı sayıları vermesi ve kök README'de listelenmesi
- `behavior_extension_test.go`: VS Code eklentisinin metinlerinin, ayarlarının ve JS testlerinin çeviri dosyasıyla tutarlılığı
- `behavior_interrupt_test.go`: kesme sinyalinde nazik çıkış, beklemeyi uyandırma ve ikinci kesmede hemen çıkış
- `behavior_interrupt_unix_test.go`: kesme testlerinin Unix'te süreç hazırlığı ve sinyal gönderimi
- `behavior_interrupt_windows_test.go`: kesme testlerinin Windows'ta süreç hazırlığı ve sinyal gönderimi; konsol olayı gönderilemeyip atlanan testlerin sayısı `TestMain` sonunda bir `UYARI` satırıyla yazılır
- `behavior_library_test.go`: kitaplık modüllerinin yüklenmesi ve GPT eğitim betiğinin duman testi
- `behavior_process_logging_test.go`: komut satırı seçenekleri, gizli değerlerin çıktıda maskelenmesi ve ilerleme çıktısı
- `behavior_profile_watch_test.go`: `TR_cmd_profile` ve `TR_cmd_watch` komutları, profil dosyası ve GPU raporu
- `behavior_speed_test.go`: hız tabanı (madde 15): kodlu dosyada `TR_cmd_check` ve çalıştırmanın süresi (`BenchmarkCheckAndRun`)
- `behavior_temp_dir_test.go`: komutların geçici klasör bırakmaması
