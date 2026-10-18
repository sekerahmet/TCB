# GPU sınaması ve kıyası (Colab)

Bu klasör `bash bakim/gpu/colab_hazirla.sh KLASOR` ile hazırlanır ve Drive'a
(`Drive'ım/TR_dili/`) kopyalanır. Aşağıdaki hücreler sırayla, bir GPU çalışma zamanında
(ör. G4) çalıştırılır. Her hücrenin çıktısının tamamı geri bildirilir.

## 1. Drive'ı bağla, dosyaları kopyala

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
```

## 2. Kart ve sürücü

```bash
%%bash
nvidia-smi
nvidia-smi --query-gpu=name,compute_cap,memory.total,driver_version --format=csv
```

## 3. GPU arka ucunun sınamaları (her çekirdek CPU ile karşılaştırılır)

İlk koşu her çekirdekten sonra eşzamanlar (`TR_GPU_ESZAMANLI=1`): bir çekirdek hata
verirse hata tam olarak o çekirdekte bildirilir. İlk satır sınamaların gerçek GPU'da
koştuğunu yazmalıdır (`GPU sınamaları gerçek GPU'da koşuyor: 1 GPU; gpu: ...`).

```bash
%%bash
cd /content/tr
TR_GPU_ESZAMANLI=1 ./gpu_sinama -test.v -test.count=1 2>&1 | tail -80
```

Ardından normal (eşzamansız) koşu:

```bash
%%bash
cd /content/tr
./gpu_sinama -test.v -test.count=1 2>&1 | tail -80
```

Yalnızca bir sınamayı yeniden koşturmak için `-test.run 'SINAMANIN_ADI'` ekleyin.

PTX derleme hatasında mesaj sürücünün derleyici günlüğünü ve PTX metninin yazıldığı
geçici dosyanın yolunu verir; o dosyayı da gönderin:

```bash
%%bash
ls /tmp/tr_gpu_*.ptx 2>/dev/null && cat /tmp/tr_gpu_*.ptx | head -200
```

## 4. Dil tanımı testleri (GPU kuralları 15.26–15.30 dahil)

```bash
%%bash
cd /content/tr
./tr test testler/tensor_test.tr 2>&1 | tail -15
./tr test testler 2>&1 | tail -3
```

## 5. Veri (isteğe bağlı)

Kendi `.npy` veri klasörünüz varsa yerel diske kopyalayın (Drive'dan okumak yavaştır).
Veri verilmezse kıyas sabit sentetik toplu işlerle koşar (süre geçerli, kayıp anlamsız).

```bash
%%bash
mkdir -p /content/veri
cp "/content/drive/MyDrive/VERI_KLASORUNUZ/"*.npy /content/veri/ && ls -l /content/veri
```

## 6. Kıyas: TR

Ayar dosyası `kiyas_ayar.json` betikle aynı klasördedir. `"veri"` boş bırakılırsa sentetik
toplu işler kullanılır. Kıyas sürerken GPU kullanım oranı 200 ms'de bir örneklenir.

```bash
%%bash
cd /content/tr
for boyut in kucuk buyuk; do
  echo "{\"boyut\": \"$boyut\", \"veri\": \"/content/veri\", \"cihaz\": \"gpu\"}" > kiyas_ayar.json
  nvidia-smi --query-gpu=utilization.gpu,memory.used --format=csv,noheader,nounits -lms 200 > kullanim_tr_$boyut.csv &
  izleyici=$!
  ./tr kiyas_gpu.tr
  kill $izleyici
  awk -F', ' '{k+=$1; if ($2>m) m=$2; n++} END {printf "GPU kullanımı ortalama %%%.0f, en çok bellek %d MB (%d örnek)\n", k/n, m, n}' kullanim_tr_$boyut.csv
done
```

Veri klasörü yoksa `"veri": ""` yazın. Aynı modeli CPU'da görmek için `"cihaz": "cpu"`
(yalnızca küçük ayarla; büyük ayar CPU'da çok uzun sürer).

## 7. Kıyas: karşılaştırma tarafı (derlemesiz ve derlemeli)

```bash
%%bash
cd /content/tr
for boyut in kucuk buyuk; do
  nvidia-smi --query-gpu=utilization.gpu,memory.used --format=csv,noheader,nounits -lms 200 > kullanim_py_$boyut.csv &
  izleyici=$!
  python kiyas_gpu.py --boyut $boyut --veri /content/veri --derleme ikisi
  kill $izleyici
  awk -F', ' '{k+=$1; if ($2>m) m=$2; n++} END {printf "GPU kullanımı ortalama %%%.0f, en çok bellek %d MB (%d örnek)\n", k/n, m, n}' kullanim_py_$boyut.csv
done
```

Veri yoksa `--veri` argümanını çıkarın.

## Geri bildirilecekler

1. 2. hücrenin çıktısı (kart, sürücü).
2. 3. hücredeki iki koşunun son 80 satırı.
3. 4. hücrenin özetleri.
4. 6. ve 7. hücrelerin bütün çıktıları (ms/adım, son kayıp, GPU kullanımı, bellek).

## Teşhis ortam değişkenleri

- `TR_GPU_ESZAMANLI=1`: her çekirdekten sonra eşzamanlar; eşzamansız bir hata (ör.
  `CUDA_ERROR_ILLEGAL_ADDRESS`) tam olarak onu üreten çekirdekte bildirilir. Yavaştır;
  yalnızca teşhis için.
- `TR_GPU_BENZETIM=1`: gerçek GPU yokken PTX benzetimini `"gpu"` cihazı olarak kurar
  (geliştirme makinesinde GPU kodunu dil düzeyinde sınamak için; çok yavaştır).
