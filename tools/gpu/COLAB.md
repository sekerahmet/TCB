# GPU sınaması, mikro kıyaslar, model kıyası ve profil (Colab)

Kod GitHub'dan çekilir ve Colab'da derlenir; Drive yalnızca eğitim verisi ve sonuçlar
içindir (`Drive'ım/TR_dili/`). Hücreler tek oturumda, bir GPU çalışma zamanında (ör. G4)
sırayla çalıştırılır: sınamalar → mikro kıyaslar (TR ve PyTorch) → model kıyası → profil.
Her hücrenin geri bildirilecek çıktısı kısadır ve tablo biçimindedir.

## 1. Kodu çek ve derle, GPU durumu

```bash
%%bash
# Go (oturumda bir kez, ~1 dk); sürüm go.mod'daki araç zinciridir.
[ -x /usr/local/go/bin/go ] || { wget -q https://go.dev/dl/go1.25.13.linux-amd64.tar.gz -O /tmp/go.tgz && tar -C /usr/local -xzf /tmp/go.tgz; }
export PATH=/usr/local/go/bin:$PATH
rm -rf /content/TCB && git clone -q --depth 1 https://github.com/sekerahmet/TCB /content/TCB
cd /content/TCB && git log -1 --format='commit: %h (%cd)' --date=iso
mkdir -p /content/tr
CGO_ENABLED=0 go build -trimpath -o /content/tr/tr ./cmd/tr
CGO_ENABLED=0 go test -c -trimpath -o /content/tr/gpu_sinama ./internal/compute/gpu
cp tools/gpu/compare_gpu.tr tools/gpu/gpt_training.tr tools/gpu/compare_gpu.py tools/gpu/micro_compare.py tools/gpu/micro_table.py /content/tr/
cp -r tests /content/tr/
nvidia-smi --query-gpu=name,compute_cap,memory.total,driver_version --format=csv,noheader
# GPU durumu ve matris çarpımının yolu (cuBLASLt bulunduysa yolu yazılır):
cd /content/tr && ./gpu_sinama -test.run '^$' 2>&1 | head -1
```

GPU'ya dokunan bir iş ana dala gitmeden önce yan dalda sınanır (bkz. [`docs/development.md`](../../docs/development.md#gönderim-sırası-cpu-ve-gpu)): `git clone` satırına `-b gpu-dogrulama` ekleyin. Belirli bir commit'i sınamak için `git clone` satırından sonra `git fetch -q --depth 1 origin
COMMIT && git checkout -q COMMIT` ekleyin.

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
./gpu_sinama -test.v -test.count=1 -test.run 'GemmLibrary' 2>&1 | tail -3
```

PTX derleme hatasında mesaj PTX metninin yazıldığı geçici dosyanın yolunu verir; o dosyayı da
gönderin (`cat /tmp/tr_gpu_*.ptx | head -200`).

## 3. Dil tanımı testleri

```bash
%%bash
cd /content/tr
./tr test tests 2>&1 | tail -3
```

## 4. Mikro kıyaslar (TR ve PyTorch, işlem işlem)

Matris çarpımı TR'de iki yolla ölçülür (cuBLASLt ve kendi çekirdeğimiz). Tablo süre ve
GFLOPS/GB/s verir; son sütun TR'nin en iyi süresinin PyTorch'a oranıdır (< 1: TR daha hızlı).

```bash
%%bash
cd /content/tr
TR_GPU_MIKRO=1 ./gpu_sinama -test.run 'TestMicroBench$' -test.v > mikro_tr.txt 2>&1
python micro_compare.py > mikro_py.txt 2>&1
python micro_table.py mikro_tr.txt mikro_py.txt
```

## 5. Veri (isteğe bağlı)

Drive yalnızca burada ve 9. hücrede gerekir:

```python
from google.colab import drive
drive.mount('/content/drive')
```

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
  echo "{\"boyut\": \"$boyut\", \"veri\": \"/content/veri\", \"cihaz\": \"gpu\"}" > compare_config.json
  p=$(izle); ./tr compare_gpu.tr | tail -1; kill $p; ozet
  p=$(izle); python compare_gpu.py --boyut $boyut --veri /content/veri --derleme yok 2>&1 | tail -1; kill $p; ozet
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
echo '{"boyut": "buyuk_kisa", "veri": "/content/veri", "cihaz": "gpu"}' > compare_config.json
TR_GPU_PROFIL=1 ./tr compare_gpu.tr 2>&1 | tail -70
```

## 8. Dış profilleyiciler (isteğe bağlı)

nsys, TR'nin sürücü çağrılarını CUPTI ile toplar; TR program sonunda aracın çıkış
işleyicilerini çalıştırır (Go programları bunları kendiliğinden çalıştırmaz; daha önce rapor
bu yüzden boş kalıyordu). Rapor yine boşsa çıktıyı aynen gönderin.

```bash
%%bash
cd /content/tr
echo '{"boyut": "buyuk_kisa", "veri": "/content/veri", "cihaz": "gpu"}' > compare_config.json
nsys profile -t cuda -o tr_buyuk --force-overwrite true ./tr compare_gpu.tr 2>&1 | tail -3
nsys stats --report cuda_gpu_kern_sum --format csv tr_buyuk.nsys-rep 2>/dev/null | head -25
```

Tek bir çekirdeğin donanım sayaçları (ör. kendi matris çarpımımız):

```bash
%%bash
cd /content/tr
TR_GPU_MIKRO=1 TR_GPU_KENDI_CARPIM=1 ncu --target-processes all -k regex:carpim --launch-count 2 \
  --section SpeedOfLight ./gpu_sinama -test.run 'TestMicroBench$' 2>&1 | grep -E 'carpim|Throughput|Duration' | head -20
```

## 9. SimpleStories ile GPT eğitimi (dilin kitaplığıyla)

`gpt_training.tr` dilin kitaplığındaki GPT'yi (`aktar "tr/katman"`) gizli ağırlıklarda Muon,
gömme/norm ağırlıklarında AdamW ile (`aktar "tr/iyilestirici"`), ısınma–sabit–azalma (WSD)
öğrenme oranı planıyla eğitir. Veri klasörü: `tokenizer.json`, `train.npy`, `valid.npy`
(uint16 parça kimlikleri). Varsayılan model d512, 8 katman, 8 baş, bağlam 512, toplu 128.
Koşu klasörünü yerel diskte tutun, bitince Drive'a kopyalayın.

```bash
%%bash
mkdir -p /content/ss4096
cp "/content/drive/MyDrive/VERI/ss4096/"{tokenizer.json,train.npy,valid.npy} /content/ss4096/
cd /content/tr
# Önce birkaç dakikalık deneme: bellek ve hız (speed.json)
./tr gpt_training.tr --veri /content/ss4096 --koşu /content/runs/deneme --adım 60 --ısınma 10 \
  --değerlendir_her 30 --örnek_her 60 2>&1 | tail -4
cat /content/runs/deneme/speed.json
```

Bellek yetmezse toplu işi bölün (adım başına aynı veri): `--toplu 64 --birikim 2` (ya da
`--toplu 32 --birikim 4`). Karışık hassasiyet: `--hassasiyet bf16`. NorMuon: `--normuon`
(öğrenme oranı AdamW ölçeğindedir: `--muon_oranı 0.003`). Bütün ayarlar: `./tr gpt_training.tr --yardım`.

```bash
%%bash
cd /content/tr
./tr gpt_training.tr --veri /content/ss4096 --koşu /content/runs/gpt1 --adım 5000 2>&1 | tail -8
cp -r /content/runs/gpt1 "/content/drive/MyDrive/TR_dili/kosular/"
```

Oturum kesilirse aynı komut kaldığı yerden sürer (`kontrol_noktalari/`, 15 dakikada bir).
Koşu klasörü: `config.json`, `metrics.jsonl` (eğitim kaybı 10 adımda bir, `oran_çarpanı`,
`doğrulama_kaybı`), `samples.json` ("Once upon a time" ile örnekler), `speed.json`
(`ms_adım`, `parça_sn`), `model.trt`.

## Geri bildirilecekler

1. 1. hücrenin son iki satırı (kart, matris çarpımı yolu).
2. 2. ve 3. hücrelerin çıktıları.
3. 4. hücrenin tablosu.
4. 6. hücrenin çıktısı (ms/adım, son kayıp, GPU kullanımı, bellek).
5. 7. hücrenin özeti; 8. hücre çalıştırıldıysa çıktısı.
6. 9. hücre çalıştırıldıysa: speed.json, son doğrulama kaybı ve samples.json'daki son örnek.

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
- `TR_GPU_COP_ESIGI=MB`: GPU belleği için çöp toplama eşiği (varsayılan kart belleğinin 1/64'ü,
  256 MB–2 GB; `0` kapatır). Küçültmek tepe belleği düşürür, hız farkı küçüktür.
