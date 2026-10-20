# cmd/tr/e2e

`tr` ikilisini derleyip gerçek süreç olarak çalıştıran uçtan uca testler: komutlar, çıkış kodları, sinyaller, belgeler ve VS Code eklentisi. Bu klasörde üretim kodu yoktur; bütün dosyalar davranış testidir, ortak yardımcılar `helpers_test.go`'dadır.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`); bu klasörde üretim kodu olmadığından hepsi davranış testidir. Birden çok dosyayı kesen davranış testleri:

- `behavior_arguments_test.go`: betiğe ve derlenmiş uygulamaya komut satırı argümanlarının iletilmesi
- `behavior_checkpoint_resume_test.go`: kontrol noktasından devam eden eğitimin kesintisizle bit düzeyinde aynı sonucu vermesi
- `behavior_coded_files_test.go`: kodlu dosyaların Türkçe dosyalarla aynı çıktıyı vermesi; kodla/göster komutları ve İngilizce yazım dili
- `behavior_colab_cells_test.go`: `tools/gpu/COLAB.md` hücrelerindeki satır içi Python'un ve GPU betiklerinin sözdizimi (Colab'a gitmeden önce)
- `behavior_trt_python_test.go`: TR tensör dosyasının (.trt) Python'da yazılıp TR'de okunması ve tersi (`tools/gpu/trt_io.py`)
- `behavior_target_model_parity_test.go`: hedef modelin TR karşılığının PyTorch sürümüyle aynı başlangıçtan float32'de adım adım aynı kaybı ve gradyan normunu vermesi (PyTorch varsa)
- `behavior_commands_test.go`: komutların (çalıştır, kontrol, test, biçimle, derle, DAP, REPL) uçtan uca davranışı, hata iletileri ve çıkış kodları
- `behavior_doc_code_blocks_test.go`: belgelerdeki TR kod bloklarının ve eklenti şablonlarının derlenmesi, kaldırılmış adların belgede kalmaması
- `behavior_doc_links_test.go`: belgelerdeki bağlantıların ve başlık çapalarının geçerliliği
- `behavior_file_formats_language_test.go`: dosya biçimlerinin (kontrol noktası, günlük) yazım diline göre değişmemesi: TR_DIL=en ile aynı dosya adları ve anahtarlar
- `behavior_extension_test.go`: VS Code eklentisinin metinlerinin, ayarlarının ve JS testlerinin çeviri dosyasıyla tutarlılığı
- `behavior_interrupt_test.go`: kesme sinyalinde nazik çıkış, beklemeyi uyandırma ve ikinci kesmede hemen çıkış
- `behavior_interrupt_unix_test.go`: kesme testlerinin Unix'te süreç hazırlığı ve sinyal gönderimi
- `behavior_interrupt_windows_test.go`: kesme testlerinin Windows'ta süreç hazırlığı ve sinyal gönderimi
- `behavior_library_test.go`: kitaplık modüllerinin yüklenmesi ve GPT eğitim betiğinin duman testi
- `behavior_process_logging_test.go`: komut satırı seçenekleri, gizli değerlerin çıktıda maskelenmesi ve ilerleme çıktısı
- `behavior_profile_watch_test.go`: profil ve izle komutları, profil dosyası ve GPU raporu
- `behavior_temp_dir_test.go`: komutların geçici klasör bırakmaması
