# GPU sınaması, mikro kıyaslar, model kıyası ve profil (Colab)

Bu klasör `bash tools/gpu/colab_hazirla.sh KLASOR` ile hazırlanır ve Drive'a
(`Drive'ım/TR_dili/`) kopyalanır. Hücreler tek oturumda, bir GPU çalışma zamanında (ör. G4)
sırayla çalıştırılır: sınamalar → mikro kıyaslar (TR ve PyTorch) → model kıyası → profil.
Her hücrenin geri bildirilecek çıktısı kısadır ve tablo biçimindedir.

## 1. Drive'ı bağla, dosyaları kopyala, GPU durumu

```python
from google.colab import drive
drive.mount('/content/drive')
```

```bash
%%bash
mkdir -p /content/tr
cp -r "/content/drive/MyDrive/TR_dili/." /content/tr/
chmod +x /content/tr/tr /content/tr/gpu_sinama
cat /content/tr/SURUM.txt
nvidia-smi --query-gpu=name,compute_cap,memory.total,driver_version --format=csv,noheader
# GPU durumu ve matris çarpımının yolu (cuBLASLt bulunduysa yolu yazılır):
cd /content/tr && ./gpu_sinama -test.run '^$' 2>&1 | head -1
```

İlk satır `matris çarpımı: cuBLASLt 12.8.x (/usr/local/cuda/lib64/libcublasLt.so.12)` gibi
olmalıdır (Blackwell için en az 12.8; eski sürümlü adaylar `atlanan:` diye yazılır). `kendi çekirdeği (cuBLASLt bulunamadı ...)` yazıyorsa kitaplığın yolu açıkça
verilebilir (PyTorch'un kurduğu kopya da olur):
`export TR_GPU_CUBLASLT=$(python -c "import nvidia.cublas, os; print(os.path.join(nvidia.cublas.__path__[0], 'lib', 'libcublasLt.so.12'))")`.

## 2. GPU arka ucunun sınamaları (her çekirdek CPU ile karşılaştırılır)

İlk koşu her çekirdekten sonra eşzamanlar (`TR_GPU_ESZAMANLI=1`): bir çekirdek hata
verirse hata tam olarak o çekirdekte bildirilir. Ardından normal (eşzamansız) koşu.

```bash
%%bash
cd /content/tr
TR_GPU_ESZAMANLI=1 ./gpu_sinama -test.count=1 2>&1 | tail -5
./gpu_sinama -test.v -test.count=1 2>&1 | grep -E '^(--- FAIL|FAIL|ok|PASS)|panic' | head -20
./gpu_sinama -test.v -test.count=1 -test.run 'CarpimKitapligi' 2>&1 | tail -3
```

PTX derleme hatasında mesaj PTX metninin yazıldığı geçici dosyanın yolunu verir; o dosyayı da
gönderin (`cat /tmp/tr_gpu_*.ptx | head -200`).

## 3. Dil tanımı testleri

```bash
%%bash
cd /content/tr
./tr test testler 2>&1 | tail -3
```

## 4. Mikro kıyaslar (TR ve PyTorch, işlem işlem)

Matris çarpımı TR'de iki yolla ölçülür (cuBLASLt ve kendi çekirdeğimiz). Tablo süre ve
GFLOPS/GB/s verir; son sütun TR'nin en iyi süresinin PyTorch'a oranıdır (< 1: TR daha hızlı).

```bash
%%bash
cd /content/tr
TR_GPU_MIKRO=1 ./gpu_sinama -test.run 'TestMikroKiyas$' -test.v > mikro_tr.txt 2>&1
python mikro_kiyas.py > mikro_py.txt 2>&1
python mikro_tablo.py mikro_tr.txt mikro_py.txt
```

## 5. Veri (isteğe bağlı)

Kendi `.npy` veri klasörünüz varsa yerel diske kopyalayın. Veri verilmezse kıyas sabit
sentetik toplu işlerle koşar (süre geçerli, kayıp anlamsız).

```bash
%%bash
mkdir -p /content/veri
cp "/content/drive/MyDrive/VERI_KLASORUNUZ/"*.npy /content/veri/ && ls -l /content/veri
```

## 6. Model kıyası: TR ve PyTorch

`"veri": ""` ve `--veri` verilmezse sentetik toplu işler kullanılır. GPU kullanımı ve bellek
200 ms'de bir örneklenir.

```bash
%%bash
cd /content/tr
izle() { nvidia-smi --query-gpu=utilization.gpu,memory.used --format=csv,noheader,nounits -lms 200 > kullanim.csv & echo $!; }
ozet() { awk -F', ' '{k+=$1; if ($2>m) m=$2; n++} END {printf "  GPU ort %%%.0f, en çok bellek %d MB\n", k/n, m}' kullanim.csv; }
for boyut in kucuk buyuk; do
  echo "{\"boyut\": \"$boyut\", \"veri\": \"/content/veri\", \"cihaz\": \"gpu\"}" > kiyas_ayar.json
  p=$(izle); ./tr kiyas_gpu.tr | tail -1; kill $p; ozet
  p=$(izle); python kiyas_gpu.py --boyut $boyut --veri /content/veri --derleme yok 2>&1 | tail -1; kill $p; ozet
done
```

Kendi matris çarpımı çekirdeğimizle karşılaştırmak için (isteğe bağlı) TR satırının önüne
`TR_GPU_KENDI_CARPIM=1` ekleyin.

## 7. Profil: büyük modelin çekirdek dökümü

`TR_GPU_PROFIL=1` her çekirdeği GPU olaylarıyla ölçer; program sonunda standart hataya özet
yazar: adım başına duvar/GPU/boşluk/konak süreleri, aile ve çekirdek tabloları (çağrı, toplam
ms, ortalama µs, %), bellek (ayrılmış, havuzda, canlı, tepeler).

```bash
%%bash
cd /content/tr
echo '{"boyut": "buyuk_kisa", "veri": "/content/veri", "cihaz": "gpu"}' > kiyas_ayar.json
TR_GPU_PROFIL=1 ./tr kiyas_gpu.tr 2>&1 | tail -70
```

## 8. Dış profilleyiciler (isteğe bağlı)

nsys, TR'nin sürücü çağrılarını CUPTI ile toplar; TR program sonunda aracın çıkış
işleyicilerini çalıştırır (Go programları bunları kendiliğinden çalıştırmaz; daha önce rapor
bu yüzden boş kalıyordu). Rapor yine boşsa çıktıyı aynen gönderin.

```bash
%%bash
cd /content/tr
echo '{"boyut": "buyuk_kisa", "veri": "/content/veri", "cihaz": "gpu"}' > kiyas_ayar.json
nsys profile -t cuda -o tr_buyuk --force-overwrite true ./tr kiyas_gpu.tr 2>&1 | tail -3
nsys stats --report cuda_gpu_kern_sum --format csv tr_buyuk.nsys-rep 2>/dev/null | head -25
```

Tek bir çekirdeğin donanım sayaçları (ör. kendi matris çarpımımız):

```bash
%%bash
cd /content/tr
TR_GPU_MIKRO=1 TR_GPU_KENDI_CARPIM=1 ncu --target-processes all -k regex:carpim --launch-count 2 \
  --section SpeedOfLight ./gpu_sinama -test.run 'TestMikroKiyas$' 2>&1 | grep -E 'carpim|Throughput|Duration' | head -20
```

## Geri bildirilecekler

1. 1. hücrenin son iki satırı (kart, matris çarpımı yolu).
2. 2. ve 3. hücrelerin çıktıları.
3. 4. hücrenin tablosu.
4. 6. hücrenin çıktısı (ms/adım, son kayıp, GPU kullanımı, bellek).
5. 7. hücrenin özeti; 8. hücre çalıştırıldıysa çıktısı.

## Ortam değişkenleri

- `TR_GPU_PROFIL=1`: çekirdek ölçümü ve program sonunda özet (bkz. 7).
- `TR_GPU_ESZAMANLI=1`: her çekirdekten sonra eşzamanlar; eşzamansız bir hata tam olarak onu
  üreten çekirdekte bildirilir. Yavaştır; yalnızca teşhis için.
- `TR_GPU_CUBLASLT=/tam/yol/libcublasLt.so.12`: matris çarpımı kitaplığının yolu.
- `TR_GPU_KENDI_CARPIM=1`: kitaplık bulunsa da kendi matris çarpımı çekirdeğimiz.
- `TR_GPU_DIKKAT=carpim`: dikkat birleşik çekirdek yerine matris çarpımı yolundan (karşılaştırma;
  mikro kıyas iki yolu da ölçer, `dikkat ... (matris yolu)` satırları).
- `TR_GPU_MIKRO=1`: gpu_sinama'da mikro kıyasları açar (`TR_GPU_MIKRO_BANT` kuramsal bant
  genişliği, GB/s; varsayılan 1800).
- `TR_GPU_BENZETIM=1`: gerçek GPU yokken PTX benzetimini `"gpu"` cihazı olarak kurar.
