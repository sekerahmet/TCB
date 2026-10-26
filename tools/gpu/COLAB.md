# Colab: GPU sınaması, ölçüm, profil ve ağır CPU koşuları

Kod GitHub'dan çekilir ve Colab'da derlenir; Drive yalnızca eğitim verisi ve sonuçlar içindir
(`Drive'ım/TR_dili/`). Ağır CPU koşuları (tam denetim, zorlama, CPU ölçümü) geliştiricinin makinesinde
değil, Colab'ın CPU High-RAM çalışma zamanında yapılır ([`docs/development.md`](../../docs/development.md#ölçüm-disiplini)).

## Hücre standardı

Not defterinde yalnızca aşağıdaki **temel hücreler** kalıcıdır; ölçüm, deney ve denetimler **tek seferlik
iş hücreleridir** (`İ1`, `İ2` ...) ve sonuçları Drive'a yazıldıktan sonra silinir.

| No | Hücre | Ne zaman | Süre |
| :-- | :-- | :-- | :-- |
| `0` | ÇALIŞMA ZAMANI: kart, çekirdek, bellek | her oturumun başında | anında |
| `1` | DRIVE'I BAĞLA | sonuç yazan her iş için (onay ister) | anında |
| `2` | KUR: Go, kodu çek (DAL), derle, `is_baslat` yardımcısı | dal değişince | ~1 dk, arka planda |
| `3` | NABIZ: bütün işlerin durumu ve son özet satırları | iş başlatıldıktan sonra, tahmini süre dolunca | anında |
| `İn` | tek seferlik iş: ölçüm, deney, tam denetim | gerektiğinde | arka planda, kayıtlı |
| `9` | KAPAT | GPU işi bitince ya da GPU boş bekleyecekse | anında |

Kurallar:

- **Başlık:** `# <no> · <NE YAPAR> · <GPU|CPU> · <anında | ~N dk, arka planda>`.
- **Süre önceden tahmin edilir:** benzer koşuların süresi `TR_dili/olcumler/OZET.md`'de (`Seconds` sütunu).
  Yalnızca anında bitenler (birkaç saniye) ön planda koşar; klonlama, derleme ve veri kopyası dahil gerisi arka planda.
- **Arka plan kalıbı:** her iş `bash /content/is_baslat.sh <ad> "<ne, nasıl>" -- <komut>` ile başlar
  (hücre 2 yazar). Yardımcı komutu `setsid nohup ... < /dev/null &` ile ayrı oturumda başlatır (hücreyi
  kesmek koşuya ulaşmaz, hücre beklemez), `tools/record` ile sarar ve `/content/is/<ad>/` altına `log` ile
  `bitti` (çıkış kodu) yazar. Durum yalnızca hücre 3'le, tahmini süre dolunca bir kez okunur; art arda sorgulanmaz.
- **Kayıt:** her koşu `TR_dili/olcumler/<ad>/<zaman>.json` (Türkiye saati) ve `OZET.md`'ye tek satır
  ([`tools/record`](../record/README.md)). Not (`--note`) ne ölçüldüğünü ve nasıl ölçüldüğünü söyler.
  A/B betikleri `$TR_RECORD_DIR/metrics.json`'a iki commit kimliğini ve sayıları yazar.
- **Deneyden önce:** `OZET.md`'de ve aşağıdaki tariflerde aynı deneyin yapılıp yapılmadığına bakılır.
- **Çalışma zamanı:** CPU High-RAM ağır CPU işleri için; L4 ucuz GPU (CPU işi de olur); G4 (RTX PRO 6000)
  yalnızca GPU işi ve onun hazırlığı (kurulum, derleme, veri kopyası).

## Temel hücreler

```bash
%%bash
# 0 · ÇALIŞMA ZAMANI: KART, ÇEKİRDEK, BELLEK · GPU|CPU · anında
nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader 2>/dev/null || echo "GPU yok (CPU çalışma zamanı)"
echo "çekirdek $(nproc) · bellek $(free -g | awk '/Mem:/{print $2}') GB · Drive: $([ -d /content/drive/MyDrive ] && echo bağlı || echo bağlı değil)"
```

```python
# 1 · DRIVE'I BAĞLA · CPU · anında (onay ister)
from google.colab import drive
drive.mount('/content/drive')
```

```bash
%%bash
# 2 · KUR: GO, KODU ÇEK (DAL), DERLE, is_baslat · CPU · ~1 dk, arka planda
DAL=main
cat > /content/is_baslat.sh <<'EOF'
# Kullanım: bash /content/is_baslat.sh <ad> "<ne, nasıl>" -- <komut ...>   (depo kökünde koşar)
AD=$1; NOT=$2; shift 3
mkdir -p /content/is/$AD && rm -f /content/is/$AD/bitti
setsid nohup bash -c 'cd /content/TCB && export PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=local && go run ./tools/record --name "$0" --note "$1" --out /content/drive/MyDrive/TR_dili/olcumler -- "${@:2}" > /content/is/$0/log 2>&1; echo $? > /content/is/$0/bitti' "$AD" "$NOT" "$@" > /dev/null 2>&1 < /dev/null &
echo "$AD başladı (durum: hücre 3)"
EOF
mkdir -p /content/is/kur && rm -f /content/is/kur/bitti
setsid nohup bash -c "( set -e
  [ -x /usr/local/go/bin/go ] || { wget -q https://go.dev/dl/go1.26.10.linux-amd64.tar.gz -O /tmp/go.tgz && tar -C /usr/local -xzf /tmp/go.tgz; }
  export PATH=/usr/local/go/bin:\$PATH GOTOOLCHAIN=local
  rm -rf /content/TCB && git clone -q --depth 2 -b $DAL https://github.com/sekerahmet/TCB /content/TCB && cd /content/TCB
  git log -1 --format='commit: %h %s' | cut -c1-80
  mkdir -p /content/tr && CGO_ENABLED=0 go build -trimpath -o /content/tr/tr ./cmd/tr && echo 'tr derlendi' )
  echo \$? > /content/is/kur/bitti" > /content/is/kur/log 2>&1 < /dev/null &
echo "kurulum başladı: DAL=$DAL (durum: hücre 3)"
```

```bash
%%bash
# 3 · NABIZ: İŞLERİN DURUMU VE SON ÖZET · CPU · anında
for d in /content/is/*/; do
  ad=$(basename $d)
  if [ -f $d/bitti ]; then echo "■ $ad: BİTTİ (çıkış $(cat $d/bitti))"; else echo "▶ $ad: sürüyor"; fi
  grep -E '^(--- FAIL|FAIL|panic|!! KALDI|❌|✅)' $d/log 2>/dev/null | tail -3
  tail -c 300 $d/log 2>/dev/null | tail -2; echo
done
tail -3 /content/drive/MyDrive/TR_dili/olcumler/OZET.md 2>/dev/null || echo "(OZET.md henüz yok)"
exit 0  # %%bash sıfır dışı çıkışı hücre hatası sayar; nabız hiçbir zaman hata vermez
```

```python
# 9 · KAPAT · GPU|CPU · anında (arka plan koşuları da biter)
from google.colab import runtime
runtime.unassign()
```

## İş tarifleri

Aşağıdaki tarifler bir `İn` hücresine kopyalanır ve `is_baslat` ile başlatılır. Bölüm 1 ve sonrasındaki eski tarifler
bu standarda kullanıldıkça geçirilir; kendi durum hücreleri yerine hücre 3 kullanılır.

```bash
%%bash
# İ1 · TAM DENETİM (KAYITLI) · CPU High-RAM · ~15 dk, arka planda
cd /content/TCB && bash /content/is_baslat.sh tam-denetim "tam denetim: bash tools/full_check.sh (dagitim + go-esleri)" -- bash tools/full_check.sh
```

```bash
%%bash
# İ2 · CPU TEMEL ÖLÇÜMÜ (KAYITLI) · CPU High-RAM · ~11 dk, arka planda
# tools/bench/cpu_baseline.sh: karakter modeli TR ve PyTorch, dil kıyası, tensör kernelleri. PyTorch PyPI'daki son
# sürümüyle ölçülür (Colab'ın hazır sürümü geride kalabilir); TORCH güncellenir.
TORCH=2.14.1
python3 -c "import torch,sys; sys.exit(torch.__version__.split('+')[0] != '$TORCH')" || pip install -q torch==$TORCH --index-url https://download.pytorch.org/whl/cpu 2>&1 | tail -2
cd /content/TCB && bash /content/is_baslat.sh cpu-temel "CPU temel ölçümü: karakter modeli TR/PyTorch $TORCH, dil kıyası, tensör kernelleri; 5 tekrar" -- bash tools/bench/cpu_baseline.sh
```

## 1. Kodu çek ve derle, GPU durumu

```bash
%%bash
# Go (oturumda bir kez, ~1 dk); sürüm go.mod'daki araç zinciridir.
[ -x /usr/local/go/bin/go ] || { wget -q https://go.dev/dl/go1.26.10.linux-amd64.tar.gz -O /tmp/go.tgz && tar -C /usr/local -xzf /tmp/go.tgz; }
export PATH=/usr/local/go/bin:$PATH
rm -rf /content/TCB && git clone -q --depth 1 https://github.com/sekerahmet/TCB /content/TCB
cd /content/TCB && git log -1 --format='commit: %h (%cd)' --date=iso
mkdir -p /content/tr
CGO_ENABLED=0 go build -trimpath -o /content/tr/tr ./cmd/tr
CGO_ENABLED=0 go test -c -trimpath -o /content/tr/gpu_sinama ./internal/compute/gpu
cp tools/gpu/compare_gpu.tr tools/gpu/measurement_model.tr tools/gpu/gpt_training.tr tools/gpu/compare_gpu.py tools/gpu/micro_compare.py tools/gpu/micro_table.py \
   tools/gpu/phase3_tr.tr tools/gpu/phase3_torch.py tools/gpu/phase3_table.py tools/gpu/phase3_nsys.py tools/gpu/phase3_config.json /content/tr/
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

## 2. Kart sınaması (GPU testleri, dil tanımı testleri, hız)

Her GPU değişikliği yan dalda kart sınamasından geçer: tek kart yeter; iş olgunlaşınca ve gerekirse iki kart, L4 ve G4 ([`docs/development.md`](../../docs/development.md#gönderim-sırası-cpu-ve-gpu)).
[`card_check.sh`](card_check.sh) tek komutla kart bilgisini, derlemeyi, GPU testlerini (önce her kernelden sonra
eşzamanlayarak, `TR_GPU_ESZAMANLI=1`: hata tam o kernelde bildirilir; sonra eşzamansız), dil tanımı testlerini ve
hedef modelde 120 adımlık hızı (ortanca ms/adım, tepe bellek) yapar. Kayıt aracıyla sarılır: kart, sürücü ve commit
Drive'daki kayda girer (`TR_dili/olcumler/kart/`), kartlar ve zaman arasında oradan karşılaştırılır. GPU çalışma
zamanında koşulur; yalnızca Go kurulumu ister (hücre B'deki satır). Sınama ikilisi paketin klasöründen çalışır: bazı
sınamalar paketin kaynak dosyalarını okur (`TestEveryEmittedInstructionIsSimulated`).

```bash
%%bash
# KART SINAMASI, KAYITLI (ARKA PLANDA, ~5 DK) · GPU — DAL değiştirilir; durum: hücre C
DAL=gpu-dogrulama
[ -x /usr/local/go/bin/go ] || { wget -q https://go.dev/dl/go1.26.10.linux-amd64.tar.gz -O /tmp/go.tgz && tar -C /usr/local -xzf /tmp/go.tgz; }
export PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=local
rm -rf /content/TCB && git clone -q --depth 1 -b $DAL https://github.com/sekerahmet/TCB /content/TCB
cd /content/TCB && git log -1 --format='commit: %h %s' | cut -c1-80 && rm -f /content/kosu_bitti
setsid nohup bash -c "cd /content/TCB && export PATH=/usr/local/go/bin:\$PATH GOTOOLCHAIN=local && go run ./tools/record --name kart --out /content/drive/MyDrive/TR_dili/olcumler -- bash tools/gpu/card_check.sh > /content/kosu.log 2>&1; echo \$? > /content/kosu_bitti" > /dev/null 2>&1 < /dev/null &
echo "başladı (durum: hücre C)"
```

PTX derleme hatasında mesaj PTX metninin yazıldığı geçici dosyanın yolunu verir; o dosyayı da
gönderin (`cat /tmp/tr_gpu_*.ptx | head -200`).

## 4. Mikro kıyaslar (TR ve PyTorch, işlem işlem)

Matris çarpımı TR'de iki yolla ölçülür (cuBLASLt ve kendi çekirdeğimiz). Tablo süre ve
GFLOPS/GB/s verir; son sütun TR'nin en iyi süresinin PyTorch'a oranıdır (< 1: TR daha hızlı).

```bash
%%bash
cd /content/tr
TR_GPU_MICRO=1 ./gpu_sinama -test.run 'TestMicroBench$' -test.v > mikro_tr.txt 2>&1
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
`TR_GPU_OWN_GEMM=1` ekleyin.

## 7. Profil: büyük modelin kernel dökümü

`TR_GPU_PROFIL=1` her çekirdeği GPU olaylarıyla ölçer; program sonunda standart hataya özet
yazar: adım başına duvar/GPU/boşluk/konak süreleri, aile ve kernel tabloları (çağrı, toplam
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
TR_GPU_MICRO=1 TR_GPU_OWN_GEMM=1 ncu --target-processes all -k regex:carpim --launch-count 2 \
  --section SpeedOfLight ./gpu_sinama -test.run 'TestMicroBench$' 2>&1 | grep -E 'carpim|Throughput|Duration' | head -20
```

## 9. SimpleStories ile GPT eğitimi (dilin kitaplığıyla)

`gpt_training.tr` dilin kitaplığındaki GPT'yi (`aktar "tr/katman"`) gizli ağırlıklarda Muon,
gömme/norm ağırlıklarında AdamW ile (`aktar "tr/iyilestirici"`), ısınma–sabit–azalma (WSD)
öğrenme oranı planıyla eğitir. Veri klasörü: `tokenizer.json`, `train.npy`, `valid.npy`
(uint16 parça kimlikleri). Varsayılan model d512, 8 katman, 8 bakış, bağlam 512, toplu 128.
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

Uzun koşudan önce aynı komut `--prova` ile denenir: iki adım, geçici klasör, bütün dosyaların yazıldığı denetlenir
(`Prova geçti` satırı).

```bash
%%bash
cd /content/tr
./tr gpt_training.tr --veri /content/ss4096 --koşu /content/runs/gpt1 --adım 5000 --prova 2>&1 | tail -3
./tr gpt_training.tr --veri /content/ss4096 --koşu /content/runs/gpt1 --adım 5000 2>&1 | tail -8
cp -r /content/runs/gpt1 "/content/drive/MyDrive/TR_dili/kosular/"
```

**PyTorch'la adil kıyas** (araştırma deposunun `ss_d512_transformer` koşusu: GPT-2 parçalı SimpleStories, Drive
`simplestories/gpt2`; tarif onun `results.json`'ından). Veri önce yerel diske kopyalanır; koşu klasörü Drive'dadır,
kopunca aynı komut sürer. Ardından `ss_exam.tr` araştırma deposunun ölçüsüyle sınar ve `results.json` yazar.

```bash
%%bash
mkdir -p /content/ss_gpt2 && cp /content/drive/MyDrive/simplestories/gpt2/{tokenizer.json,train.npy,valid.npy} /content/ss_gpt2/
R=/content/drive/MyDrive/TR_dili/kosular/ss_d512_tf_tr
cd /content/TCB && /content/tr/tr tools/gpu/gpt_training.tr --veri /content/ss_gpt2 --koşu $R --hassasiyet bf16 \
  --boyut 512 --katman 8 --bakış_sayısı 8 --bağlam 512 --toplu 64 --birikim 2 --adım 9241 --ısınma 92 --azalma 0.2001 \
  --muon_oranı 0.002 --adamw_oranı 0.002 --ağırlık_çürümesi 0.1 --adamw_çürümesi 0.1 --muon_ölçeği rms --ns_türü bf16 \
  --kırp 1.0 --tohum 0 --dağarcık 50304 --ara_boyut 1344 --epsilon 0.000001 --qk_normu --birleşik_kv --birleşik_kapı --ileri_sapmalı \
  --değerlendir_her 1000 --değerlendir_toplu 128 --son_değerlendir_toplu 100000 --örnek_her 1000 --örnek_sayısı 1 --her_dakika 10 \
  --istemler "Once upon a time|The little dog|One day, a girl named|In a small village," > /content/kosu.log 2>&1
cp /content/kosu.log $R/stdout.log
/content/tr/tr tools/gpu/ss_exam.tr --koşu $R --veri /content/ss_gpt2 | tail -12
```

İlk sonuç (2026-10-23, RTX PRO 6000): TR kayıp 1,5134, PyTorch 1,5196; ayrıntı `docs/roadmap.md` Evre 4.

Oturum kesilirse aynı komut kaldığı yerden sürer (`checkpoints/`, 15 dakikada bir).
Koşu klasörü: `config.json`, `metrics.jsonl` (eğitim kaybı 10 adımda bir, `oran_çarpanı`,
`doğrulama_kaybı`), `samples.json` ("Once upon a time" ile örnekler), `speed.json`
(`ms_adım`, `parça_sn`), `model.trt`.

## 10. Evre 3 ölçümü: TR ve `torch.compile` (aynı kart, aynı yapılandırma)

Soru: aynı modelde TR, `torch.compile` (varsayılan ve `max-autotune`) karşısında nerede? Model
`phase3_config.json`'dadır (12 katman, D768, 12 bakış, FF2048, bağlam 512, toplu 32, bf16, AdamW,
sentetik veri); TR ve PyTorch aynı dosyayı okur, ikisi de aynı sabit sentetik toplu işleri ve aynı
kırpma/iyileştirici ayarını kullanır. Başka bir yapılandırma için dosyayı değiştirmek yeter.
Her adım sonunda kayıp okunur (GPU eşzamanlanır): iki tarafın da adım süresi budur; sıradaki
adımın başlatılması ile GPU'nun çalışması üst üste bindirilmediği için bu süre toplu koşudan
biraz uzundur. TR'nin `zaman_ms`'i milisaniye çözünürlüklüdür; adımlar onlarca ms olduğundan
yeter. Isınma adımları (`warmup`, ilki derleme dahil) ölçülenlerden (`measured`) ayrıdır. Tepe
bellek PyTorch'ta `max_memory_allocated`, TR'de `tr profil --json` raporundan okunur (ikisi de
tensörlerin canlı tepesidir; ayrılan havuz değil).

Kayıp eğrisi: ilk `loss_steps` adımın kaybı yazılır. Eager ve compile aynı tohum ve aynı
başlangıçla koşar, birbirine bf16 yuvarlama düzeyinde yakın olmalıdır (tabloda "eager'dan en
büyük kayıp farkı"). TR'nin başlangıç ağırlıkları farklı rastgele üreteçten gelir, bu yüzden
TR eğrisi birebir aynı değil, aynı dağılımdan ve aynı eğilimdedir: [0] ≈ ln(40000) ≈ 10,6 ve
benzer iniş beklenir.

### 10.1. Kart bilgisi ve `nsys`

```bash
%%bash
nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader
python -c "import torch; print('torch', torch.__version__, 'cuda', torch.version.cuda)"
# nsys: önce kuruluysa yolu, değilse kur (kurulum Colab'da doğrulanmadı; olmazsa çıktıyı gönderin):
which nsys || ls /usr/local/cuda*/bin/nsys /opt/nvidia/nsight-systems*/*/bin/nsys 2>/dev/null   || { apt-get update -qq && apt-get install -y -qq nsight-systems-cli; }
nsys --version
```

`nsys` bulunmadıysa ve kurulamadıysa 10.4 atlanır; 10.2–10.3 ondan bağımsızdır. `which nsys` bir
şey yazmıyor ama ikinci satır bir yol veriyorsa `export PATH=$PATH:<o klasör>` ekleyin.

### 10.2. Adım süreleri: PyTorch üç kip ve TR

Dört koşu, her biri ayrı süreçte (derleme önbelleği ve bellek kirlenmesin). `max-autotune`
derlemesi dakikalar sürebilir.

```bash
%%bash
cd /content/tr
for kip in eager compile max-autotune; do
  python phase3_torch.py --mode $kip --out p3_$kip.json > /dev/null 2> p3_$kip.err || tail -5 p3_$kip.err
done
./tr profil --json p3_tr_profil.json phase3_tr.tr --çıktı p3_tr.json > /dev/null 2> p3_tr.err || tail -5 p3_tr.err
python phase3_table.py p3_tr.json p3_eager.json p3_compile.json p3_max-autotune.json --tr-memory p3_tr_profil.json
```

Geri bildirilecek: bu hücrenin tablosu (kart satırı + dört satır + oran satırı). Bir koşu
hata verdiyse `.err` dosyasının son satırları.

### 10.3. Kayıp eğrisi (doğruluk karşılaştırması)

```bash
%%bash
cd /content/tr
python - <<'PY'
import json
a = {k: json.load(open(f"p3_{k}.json"))["loss_curve"] for k in ("eager", "compile", "max-autotune", "tr")}
print("| adım | eager | compile | max-autotune | TR |")
print("|---|---|---|---|---|")
for i in sorted({0, 1, 2, 5, 10, 20, 30, len(a["eager"]) - 1} & set(range(len(a["eager"])))):
    print(f"| {i} | " + " | ".join(f"{a[k][i]:.4f}" for k in ("eager", "compile", "max-autotune", "tr")) + " |")
PY
```

### 10.4. Nsight Systems: TR adımının zaman çizelgesi (ve karşılaştırma için PyTorch compile)

Kısa koşu (`phase3_short.json`: 10 ısınma, 12 ölçülen adım) `nsys` altında çalışır. Çözümleme
penceresi, son çekirdeğin bittiği andan geriye `--window-ms`'dir: program ölçülen adımların hemen
ardından bittiği için pencere yalnızca ölçülen adımlardır (başlangıç, derleme ve bellek ayırma
boşlukları dışarıda kalır). Pencere, 10.2'de ölçülen TR medyan adım süresinin 10 katı alınır;
PyTorch için aynı pencere ve `--adim 10` kullanılır (karşılaştırma aynı süre aralığında).

```bash
%%bash
cd /content/tr
python - <<'PY'
import json
c = json.load(open("phase3_config.json")); c.update(warmup=10, measured=12, loss_steps=4)
json.dump(c, open("phase3_short.json", "w"))
PY
PEN=$(python -c "import json; print(10 * json.load(open('p3_tr.json'))['measured']['median_ms'])")
# TR
nsys profile -t cuda,nvtx --force-overwrite true -o p3_nsys_tr ./tr phase3_tr.tr --ayar phase3_short.json > /dev/null 2>&1
nsys export --type sqlite --force-overwrite true -o p3_nsys_tr.sqlite p3_nsys_tr.nsys-rep > /dev/null 2>&1
python phase3_nsys.py p3_nsys_tr.sqlite --window-ms $PEN --steps 10 --name "TR"
# PyTorch torch.compile (varsayılan kip)
nsys profile -t cuda,nvtx --force-overwrite true -o p3_nsys_pt python phase3_torch.py --mode compile --config phase3_short.json > /dev/null 2>&1
nsys export --type sqlite --force-overwrite true -o p3_nsys_pt.sqlite p3_nsys_pt.nsys-rep > /dev/null 2>&1
python phase3_nsys.py p3_nsys_pt.sqlite --window-ms $PEN --steps 10 --name "PyTorch compile"
```

Geri bildirilecek: iki tablo (kernel sayısı, kernel/adım, toplam kernel süresi, GPU boşta
oranı, kerneller arası boşluk, en pahalı 10 kernel). Boşluk oranı kernellerin birleşimine
göredir (bellek kopyaları sayılmaz); `nsys stats --report cuda_gpu_kern_sum` da aynı
kernel toplamlarını verir ve çapraz denetim için çalıştırılabilir. Rapor boşsa
(`pencerede kernel yok`) 8. hücredeki notu uygulayıp çıktıyı aynen gönderin.

## 11. Hedef model defteri (hücre 1–10, 15–17)

Hedef modelin (`tools/gpu/rgn512.py` ↔ `tools/gpu/rgn512.tr`) hız, bellek, eşleşme ve gerçek veride
kalite ölçümleri için kullanılan defter. Colab defteri kalıcı olmadığından hücreler buradadır; defter
değişince bu bölüm de güncellenir.

Kurallar:

- Her hücre `# N · NE YAPAR · GPU|CPU` başlığıyla başlar (büyük harf; GPU'yu kullanıp kullanmadığı).
  Kabuk hücreleri Colab'da `%%bash` satırıyla başlar (aşağıdaki bloklarda yazılmamıştır).
- İzleme ve durum hücreleri anında biter (`grep`, `tail`); `sleep` döngüsüyle bekleyen hücre tek
  çekirdeği kilitler.
- Uzun koşular `setsid nohup bash betik.sh > /dev/null 2>&1 < /dev/null &` ile başlar: hücreyi
  kesmek (durdurma düğmesi, `Ctrl+M I`) aynı süreç grubundaki `nohup`'lu eğitimi de durdurur.
- Çalışan bir hücrenin metni değiştirilmez (ekran yeni metni, kernel eskisini çalıştırır).
- Tepe bellek nvidia-smi'den MiB/1024 = GiB olarak yazılır (hücre 2 ve 4 aynı ölçü).
- İş bitince hücre 10 ile çalışma zamanı kapatılır.

### Hücre 1 · KURULUM: KODU ÇEK VE TR'Yİ DERLE · CPU

```bash
# 1 · KURULUM: KODU ÇEK VE TR'Yİ DERLE · CPU
# DAL: main (GPU işi sınanırken gpu-dogrulama)
DAL=main
[ -x /usr/local/go/bin/go ] || { wget -q https://go.dev/dl/go1.26.10.linux-amd64.tar.gz -O /tmp/go.tgz && tar -C /usr/local -xzf /tmp/go.tgz; }
export PATH=/usr/local/go/bin:$PATH
rm -rf /content/TCB && git clone -q --depth 1 -b $DAL https://github.com/sekerahmet/TCB /content/TCB
cd /content/TCB && git log -1 --format='commit: %h %s'
mkdir -p /content/tr
CGO_ENABLED=0 go build -trimpath -o /content/tr/tr ./cmd/tr && echo "derleme tamam"
```

### Hücre 2 · PYTORCH TEMEL ÇİZGİSİ: HIZ VE BELLEK · GPU

```bash
# 2 · PYTORCH TEMEL ÇİZGİSİ: HIZ VE BELLEK (COMPILE + EAGER, SENTETİK VERİ, 120 ADIM) · GPU
# Kayıp anlamsızdır (rastgele veri); hız ve tepe bellek (nvidia-smi, GiB) karşılaştırma içindir.
mkdir -p /content/rgn && cd /content/rgn
[ -f train.npy ] || python -c "
import numpy as np
r = np.random.default_rng(0)
np.save('train.npy', r.integers(1, 40000, size=40_000_000, dtype=np.uint16))
np.save('valid.npy', r.integers(1, 40000, size=2_000_000, dtype=np.uint16))"
for kip in compile eager; do
  ek=""; [ $kip = eager ] && ek="--no_compile"
  nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits -lms 100 > mem_$kip.log & P=$!
  python /content/TCB/tools/gpu/rgn512.py train --data train.npy --valid valid.npy --out kosu_$kip --steps 120 $ek > rgn_$kip.out 2>&1
  kill $P; sleep 1
  printf "%-8s %s | tepe %.2f GiB\n" $kip "$(grep -o '[0-9]* ms/adım' rgn_$kip.out | tail -1)" $(echo "$(sort -n mem_$kip.log | tail -1)/1024" | bc -l)
done
```

### Hücre 3 · TR PROFİLİ · GPU

```bash
# 3 · TR PROFİLİ: GPU KERNEL AİLELERİ VE TEPE BELLEK DÖKÜMÜ (tr profil, 20 ADIM) · GPU
mkdir -p /content/rgn/trveri && cd /content/rgn
[ -f trveri/train.npy ] || python -c "import numpy as np; np.save('trveri/train.npy', np.random.default_rng(0).integers(1, 40000, size=40_000_000, dtype=np.uint16))"
/content/tr/tr profil /content/TCB/tools/gpu/rgn512.tr --veri trveri --adım 20 --günlük 20 > profil.txt 2>&1
grep -A12 "^aile" profil.txt | head -14; grep -A14 "ayıran satıra göre" profil.txt
```

### Hücre 4 · TR HIZ VE BELLEK ÖLÇÜMÜ · GPU

```bash
# 4 · TR HIZ VE BELLEK ÖLÇÜMÜ (SENTETİK VERİ, 120 ADIM) · GPU
# Tepe bellek nvidia-smi (GiB = MiB/1024, hücre 2 ile aynı ölçü); tepe canlı/ayrılmış TR_GPU_PROFIL'den.
mkdir -p /content/rgn/trveri
[ -f /content/rgn/trveri/train.npy ] || python -c "import numpy as np; np.save('/content/rgn/trveri/train.npy', np.random.default_rng(0).integers(1, 40000, size=40_000_000, dtype=np.uint16))"
cd /content/rgn
nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits -lms 100 > mem.log & P=$!
/content/tr/tr /content/TCB/tools/gpu/rgn512.tr --veri trveri --adım 120 --çıktı hiz.json 2>&1 | tail -1
kill $P; sleep 1
printf "tepe (nvidia-smi): %.2f GiB\n" $(echo "$(sort -n mem.log | tail -1)/1024" | bc -l); cat hiz.json; echo
TR_GPU_PROFIL=1 /content/tr/tr /content/TCB/tools/gpu/rgn512.tr --veri trveri --adım 6 --günlük 6 2>&1 | grep "^bellek"
```

### Hücre 5 · GERÇEK VERİ: DRIVE'I BAĞLA VE KOPYALA · CPU

Python hücresi. Drive salt okunur kullanılır; bağlarken Google izin penceresi açılır.

```python
# 5 · GERÇEK VERİ: DRIVE'I BAĞLA, EĞİTİM PARÇASI VE DOĞRULAMAYI YEREL DİSKE KOPYALA · CPU
from google.colab import drive
drive.mount('/content/drive')
import os, shutil
src = '/content/drive/MyDrive/v2/fineweb2_tr/tok40k'
os.makedirs('/content/fw/train', exist_ok=True)
for a, b in [(f'{src}/train/fw2_28_01.npy', '/content/fw/train/fw2_28_01.npy'), (f'{src}/valid.npy', '/content/fw/valid.npy')]:
    if not os.path.exists(b):
        shutil.copy(a, b)
import numpy as np
for f in ['/content/fw/train/fw2_28_01.npy', '/content/fw/valid.npy']:
    a = np.load(f, mmap_mode='r')
    print(f, a.dtype, len(a), 'parça; en büyük kimlik', int(a[:5_000_000].max()))
```

### Hücre 6 · KALİTE KOŞUSU BAŞLAT · GPU

```bash
# 6 · KALİTE KOŞUSU BAŞLAT: PYTORCH SONRA TR, GERÇEK VERİ, ARKA PLANDA · GPU
# Hücre hemen döner; koşu ayrı oturumda (setsid: hücre kesmeleri eğitime ulaşmaz). Durum: hücre 7.
# Önce hücre 1 ve 5. ADIM değiştirilebilir (2000 adım ≈ PyTorch 5 dk + TR 7 dk).
ADIM=2000
cd /content && rm -rf kalite && mkdir -p kalite && cd kalite
cat > kos.sh <<EOF
python /content/TCB/tools/gpu/rgn512.py train --data /content/fw/train --valid /content/fw/valid.npy --out k_pt --steps $ADIM > pt.log 2>&1
/content/tr/tr /content/TCB/tools/gpu/rgn512.tr --veri /content/fw/train --doğrulama /content/fw/valid.npy --adım $ADIM --günlük 50 --değerlendirme 1000 --kayıplar tr_kayip.json > tr.log 2>&1
echo bitti > bitti
EOF
setsid nohup bash kos.sh > /dev/null 2>&1 < /dev/null &
echo "başladı: $ADIM adım (durum için hücre 7)"
```

### Hücre 7 · KALİTE KOŞUSU DURUMU · CPU

```bash
# 7 · KALİTE KOŞUSU DURUMU (ANINDA BİTER, BEKLEMEZ) · CPU
cd /content/kalite 2>/dev/null || { echo "koşu yok (hücre 6)"; exit 0; }
echo "== PyTorch"; grep -E "parametre|^adım (50|250|500|750|1000|1250|1500|1750|2000) /|DEĞERLENDİRME|BİTTİ" pt.log | sed 's/ | kalan.*//'
echo "== TR"; [ -f tr.log ] && grep -E "parametre|^adım (50|250|500|750|1000|1250|1500|1750|2000) /|DEĞERLENDİRME" tr.log
echo "-- son satır: $(tail -1 tr.log 2>/dev/null || tail -1 pt.log)"
[ -f bitti ] && echo "BİTTİ (karşılaştırma: hücre 8)" || echo "sürüyor"
```

### Hücre 8 · KALİTE VE HIZ KARŞILAŞTIRMASI · CPU

Python hücresi. Tek adımın kaybı gürültülüdür (farklı rastgele toplu işler); karşılaştırma 500 adımlık
dilimlerin ortalaması ve aynı doğrulama penceresindeki kayıpladır. Hız, iki günlüğün 100. adımdan
sonraki ms/adım değerlerinin ortancasıdır (gerçek veride okuma da içindedir).

```python
# 8 · KALİTE VE HIZ KARŞILAŞTIRMASI: EĞİTİM (500 ADIMLIK ORTALAMA), DOĞRULAMA KAYBI, ORTANCA MS/ADIM (ANINDA BİTER) · CPU
import re, statistics
def oku(yol):
    egitim, dogr, hiz = {}, {}, []
    for s in open(yol, encoding='utf-8'):
        m = re.match(r'adım (\d+) / \d+ .*?kayıp ([\d.]+)', s)
        if m:
            egitim[int(m[1])] = float(m[2])
            h = re.search(r'([\d.]+) ms/adım', s)
            if h and int(m[1]) > 100: hiz.append(float(h[1]))
        m = re.search(r'DEĞERLENDİRME adım (\d+): kayıp ([\d.]+)', s)
        if m: dogr[int(m[1])] = float(m[2])
    return egitim, dogr, hiz
pe, pd, ph = oku('/content/kalite/pt.log'); te, td, th = oku('/content/kalite/tr.log')
son = max(pe) if pe else 0
print('| adımlar | PyTorch eğitim (ort.) | TR eğitim (ort.) | fark |')
print('|---|---|---|---|')
for a in range(0, son, 500):
    p = [v for k, v in pe.items() if a < k <= a + 500 and k % 50 == 0]
    t = [v for k, v in te.items() if a < k <= a + 500 and k % 50 == 0]
    if p and t:
        print(f'| {a+1}–{a+500} | {sum(p)/len(p):.4f} | {sum(t)/len(t):.4f} | {sum(t)/len(t)-sum(p)/len(p):+.4f} |')
print('
| adım | PyTorch doğrulama | TR doğrulama | fark |
|---|---|---|---|')
for k in sorted(pd):
    if k in td: print(f'| {k} | {pd[k]:.4f} | {td[k]:.4f} | {td[k]-pd[k]:+.4f} |')
if ph and th:
    print(f'
hız (100. adımdan sonra, ortanca): PyTorch {statistics.median(ph):.0f} ms/adım, TR {statistics.median(th):.0f} ms/adım')
```

### Hücre 9 · EŞLEŞME TESTİ · GPU

```bash
# 9 · EŞLEŞME TESTİ: PYTORCH ↔ TR AYNI BAŞLANGIÇ VE AYNI TOPLU İŞLER, BF16, 100 ADIM · GPU
cd /content && rm -rf par && mkdir par
cd /content/TCB/tools/gpu
SECONDS=0
python rgn_reference.py --out /content/par --steps 100 --micro 32 --accum 2 --seq 1024 --device cuda --bf16 --ns-bf16 > /content/par/py.log 2>&1 || tail -5 /content/par/py.log
echo "PyTorch: ${SECONDS} sn"; SECONDS=0
cd /content/par
/content/tr/tr /content/TCB/tools/gpu/rgn512.tr --veri data --ağırlık init.trt --sıralı --adım 100 --kayıplar tr.json --günlük 100 2>&1 | tail -2
echo "TR: ${SECONDS} sn"
python - <<'PY'
import json
r = json.load(open("reference.json"))["steps"]; t = json.load(open("tr.json"))["steps"]
print("| adım | PyTorch kayıp | TR kayıp | fark | PyTorch grad | TR grad |")
print("|---|---|---|---|---|---|")
for i in [0, 1, 2, 4, 9, 19, 29, 49, 74, 99]:
    a, b = r[i], t[i]
    print(f"| {i+1} | {a['loss']:.4f} | {b['loss']:.4f} | {b['loss']-a['loss']:+.4f} | {a['grad_norm']:.3f} | {b['grad_norm']:.3f} |")
PY
```

### Hücre 15 · BELİRLENİMCİ BIRAKMA SINAMASI · GPU

Dil tanımı 15.43'ün GPU sınaması: GPU arka ucunun testleri, hedef modelde bırakma açık ve kapalı
(`TR_RELEASE=0`) hız ve tepe bellek, PyTorch ile eşleşme (hücre 9 ile aynı) ve bellek dökümlü profil.
Hücre 1'in GPU sınama ikilisini de derlemesi gerekir (`go test -c ... -o /content/tr/gpu_sinama
./internal/compute/gpu`, bölüm 1).

```bash
# 15 · BELİRLENİMCİ BIRAKMA SINAMASI: GPU TESTLERİ, HIZ+BELLEK (AÇIK/KAPALI), EŞLEŞME, PROFİL (ARKA PLANDA) · GPU
# Hücre hemen döner; koşu ayrı oturumda (setsid). Durum: hücre 16. Önce hücre 1 (DAL=gpu-dogrulama).
mkdir -p /content/rgn/trveri /content/rgn/bir && cd /content/rgn
[ -f trveri/train.npy ] || python -c "import numpy as np; np.save('trveri/train.npy', np.random.default_rng(0).integers(1, 40000, size=40_000_000, dtype=np.uint16))"
rm -f bir/*
cat > bir/kos.sh <<'EOF'
cd /content/TCB/internal/compute/gpu && /content/tr/gpu_sinama -test.count=1 > /content/rgn/bir/gpu_test.log 2>&1; echo "gpu testleri çıkış $?" >> /content/rgn/bir/sonuc.txt
cd /content/rgn
for B in 1 0; do
  nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits -lms 100 > bir/mem_$B.log & P=$!
  TR_RELEASE=$B /content/tr/tr /content/TCB/tools/gpu/rgn512.tr --veri trveri --adım 120 --çıktı bir/hiz_$B.json > bir/tr_$B.log 2>&1
  kill $P; sleep 1
  printf "bırakma %s | tepe %.2f GiB | ortanca %s ms/adım | son kayıp %s\n" $([ $B = 1 ] && echo açık || echo kapalı) $(echo "$(sort -n bir/mem_$B.log | tail -1)/1024" | bc -l) $(python3 -c "import json;print(round(json.load(open('bir/hiz_$B.json'))['measured']['median_ms'],1))") "$(grep -o 'kayıp [0-9.]*' bir/tr_$B.log | tail -1)" >> bir/sonuc.txt
done
rm -rf /content/par && mkdir /content/par && cd /content/TCB/tools/gpu
python rgn_reference.py --out /content/par --steps 100 --micro 32 --accum 2 --seq 1024 --device cuda --bf16 --ns-bf16 > /content/par/py.log 2>&1
cd /content/par && /content/tr/tr /content/TCB/tools/gpu/rgn512.tr --veri data --ağırlık init.trt --sıralı --adım 100 --kayıplar tr.json --günlük 100 > tr.log 2>&1
python - >> /content/rgn/bir/sonuc.txt <<'PY'
import json
r = json.load(open("reference.json"))["steps"]; t = json.load(open("tr.json"))["steps"]
d = max(abs(a["loss"] - b["loss"]) for a, b in zip(r, t)); g = max(abs(a["grad_norm"] - b["grad_norm"]) for a, b in zip(r, t))
print(f"eşleşme (100 adım, bf16): en büyük kayıp farkı {d:.4f}, grad farkı {g:.4f}; son kayıp PT {r[-1]['loss']:.4f} TR {t[-1]['loss']:.4f}")
PY
cd /content/rgn && /content/tr/tr profil /content/TCB/tools/gpu/rgn512.tr --veri trveri --adım 20 --günlük 20 > bir/profil.txt 2>&1
echo bitti > bir/bitti
EOF
setsid nohup bash bir/kos.sh > /dev/null 2>&1 < /dev/null &
echo "başladı (durum: hücre 16)"
```

### Hücre 16 · BIRAKMA SINAMASI DURUMU · CPU

```bash
# 16 · BIRAKMA SINAMASI DURUMU (ANINDA BİTER, BEKLEMEZ) · CPU
cd /content/rgn/bir 2>/dev/null || { echo "koşu yok (hücre 15)"; exit 0; }
cat sonuc.txt 2>/dev/null
grep -E "^(--- FAIL|FAIL|ok|PASS)" gpu_test.log 2>/dev/null | head -5
if [ -f bitti ]; then
  echo "== PROFİL"; grep -E "^bellek|Çöp toplama|Bellek:" profil.txt; grep -A16 "ayıran satıra göre" profil.txt
else L=$(ls -t *.log /content/par/*.log 2>/dev/null | head -1); echo "sürüyor: $L → $(tail -1 $L 2>/dev/null | cut -c1-150)"; fi
```

### Hücre 17 · EŞLEŞMEYİ BIRAKMA KAPALI TEKRARLA · GPU

Hücre 15'in PyTorch referansıyla TR'yi `TR_RELEASE=0` ile yeniden koşar: bırakma sonucu değiştirmez
(iki TR koşusunun kayıpları aynı olmalıdır); PyTorch'a karşı fark bf16'nın koşudan koşuya
belirlenimsizliğidir.

```bash
# 17 · EŞLEŞMEYİ BIRAKMA KAPALI TEKRARLA (AYNI PYTORCH REFERANSI, ~2 DK) · GPU
cd /content/par
TR_RELEASE=0 /content/tr/tr /content/TCB/tools/gpu/rgn512.tr --veri data --ağırlık init.trt --sıralı --adım 100 --kayıplar tr0.json --günlük 100 > tr0.log 2>&1
python - <<'PY'
import json
r = json.load(open("reference.json"))["steps"]
for ad in ["tr.json", "tr0.json"]:
    t = json.load(open(ad))["steps"]
    d = [abs(a["loss"] - b["loss"]) for a, b in zip(r, t)]
    i = max(range(len(d)), key=d.__getitem__)
    print(f"{ad}: en büyük kayıp farkı {d[i]:.4f} (adım {i+1}), ortalama {sum(d)/len(d):.4f}")
a = json.load(open("tr.json"))["steps"]; b = json.load(open("tr0.json"))["steps"]
print("bırakma açık / kapalı TR kayıpları en çok", max(abs(x["loss"] - y["loss"]) for x, y in zip(a, b)), "farklı")
PY
```

### Hücre 18 · PROFİLİN KERNEL AİLELERİ · CPU

Hücre 15'in profilinden adım başına süreyi kernel ailelerine (çarpım, eleman bazlı, indirge,
kopya, kaynaşık …) ve en uzun 30 çekirdeğe göre gösterir.

```bash
# 18 · PROFİLİN KERNEL AİLELERİ VE İLK 30 KERNEL (ANINDA BİTER) · CPU
cd /content/rgn/bir && grep -A30 "^aile" profil.txt | head -28; grep -A32 "^kernel (ilk" profil.txt | cut -c1-110
```

### Hücre 19 · HIZLI ÖLÇÜM · GPU

Bir hız değişikliğinin kısa sınaması (~4 dk): ilgili GPU testleri (kaynaşık kerneller, parçalı
doğrusal+CE, normlar), hedef modelde 120 adımın ortanca süresi ve tepe belleği, kernel profili.
Eşleşme için hücre 15'in tamamı koşulur.

```bash
# 19 · HIZLI ÖLÇÜM: KAYNAŞIK GPU TESTLERİ, HIZ+BELLEK, PROFİL (ARKA PLANDA, ~4 DK) · GPU
# Hücre hemen döner; durum: hücre 20. Önce hücre 1 (DAL=gpu-dogrulama).
mkdir -p /content/rgn/trveri /content/rgn/hz && cd /content/rgn
[ -f trveri/train.npy ] || python -c "import numpy as np; np.save('trveri/train.npy', np.random.default_rng(0).integers(1, 40000, size=40_000_000, dtype=np.uint16))"
rm -f hz/*
cat > hz/kos.sh <<'EOF'
cd /content/TCB/internal/compute/gpu && /content/tr/gpu_sinama -test.run 'Fused|LinearCrossEntropy|Norms' -test.count=1 > /content/rgn/hz/gpu_test.log 2>&1; echo "gpu testleri çıkış $?" >> /content/rgn/hz/sonuc.txt
cd /content/rgn
nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits -lms 100 > hz/mem.log & P=$!
/content/tr/tr /content/TCB/tools/gpu/rgn512.tr --veri trveri --adım 120 --çıktı hz/hiz.json > hz/tr.log 2>&1
kill $P; sleep 1
printf "tepe %.2f GiB | ortanca %s ms/adım | %s\n" $(echo "$(sort -n hz/mem.log | tail -1)/1024" | bc -l) $(python3 -c "import json;print(round(json.load(open('hz/hiz.json'))['measured']['median_ms'],1))") "$(grep -o 'kayıp [0-9.]*' hz/tr.log | tail -1)" >> hz/sonuc.txt
/content/tr/tr profil /content/TCB/tools/gpu/rgn512.tr --veri trveri --adım 20 --günlük 20 > hz/profil.txt 2>&1
echo bitti > hz/bitti
EOF
setsid nohup bash hz/kos.sh > /dev/null 2>&1 < /dev/null &
echo "başladı (durum: hücre 20)"
```

### Hücre 20 · HIZLI ÖLÇÜM DURUMU · CPU

```bash
# 20 · HIZLI ÖLÇÜM DURUMU VE PROFİL (ANINDA BİTER, BEKLEMEZ) · CPU
cd /content/rgn/hz 2>/dev/null || { echo "koşu yok (hücre 19)"; exit 0; }
cat sonuc.txt 2>/dev/null
if [ -f bitti ]; then
  grep -E "^bellek" profil.txt; grep -A22 "^aile" profil.txt; grep -A26 "^kernel (ilk" profil.txt | cut -c1-100
else echo "sürüyor: $(tail -c 160 tr.log 2>/dev/null)"; fi
```

### Hücre 21 · KİTAPLIK DENEYİ (A/B) · GPU

Bir kitaplık değişikliğini dile ve depoya almadan önce ölçer: `/content/TCB`'nin kopyasında
`lib/katman.tr` yamalanır (kitaplık ikiliye gömülüdür), `tr_deney` derlenir; iki ikili aynı
sentetik veride sırayla ikişer kez 120 adım koşar. Örnekteki yama, f32 girdili normların
(`norm1`, `norm2`, `son_norm`) çıktısını doğrudan bf16 yazar (`tür:`): bu modelde onları yalnızca
çarpımlar okur, çarpımın kendi dönüşümü gereksizleşir. Yamayı deneye göre değiştirin
(`assert`, her aranan metnin kitaplıkta bir kez geçtiğini denetler).

```bash
# 21 · DENEY A/B: NORM ÇIKTISI bf16 (tür:) — KİTAPLIK YAMALI İKİLİ vs OLDUĞU GİBİ (ARKA PLANDA, ~4 DK) · GPU
# Hücre hemen döner; durum: hücre 22. Önce hücre 1 (DAL=gpu-dogrulama).
export PATH=/usr/local/go/bin:$PATH
rm -rf /content/TCB_b && cp -r /content/TCB /content/TCB_b && cd /content/TCB_b
python3 - <<'PY'
p = "lib/katman.tr"; s = open(p, encoding="utf-8").read()
r = [("kendi.norm1.ileri(x)", 'rms_normu(x, kendi.norm1.ağırlık, epsilon: kendi.norm1.epsilon, tür: "bf16")'),
     ("kendi.norm2.ileri(h)", 'rms_normu(h, kendi.norm2.ağırlık, epsilon: kendi.norm2.epsilon, tür: "bf16")'),
     ("sonuç kendi.son_norm.ileri(x)", 'sonuç rms_normu(x, kendi.son_norm.ağırlık, epsilon: kendi.son_norm.epsilon, tür: "bf16")')]
for a, b in r:
    assert s.count(a) == 1, a
    s = s.replace(a, b)
open(p, "w", encoding="utf-8").write(s)
print("kitaplık yamalandı")
PY
CGO_ENABLED=0 go build -trimpath -o /content/tr/tr_deney ./cmd/tr && echo "tr_deney derlendi"
mkdir -p /content/rgn/trveri /content/rgn/ab && cd /content/rgn
[ -f trveri/train.npy ] || python -c "import numpy as np; np.save('trveri/train.npy', np.random.default_rng(0).integers(1, 40000, size=40_000_000, dtype=np.uint16))"
rm -f ab/*
cat > ab/kos.sh <<'EOF'
cd /content/rgn
for i in 1 2; do for ad in tr tr_deney; do
  /content/tr/$ad /content/TCB/tools/gpu/rgn512.tr --veri trveri --adım 120 --çıktı ab/${ad}_$i.json > ab/${ad}_$i.log 2>&1
  printf "%-9s tur %s | ortanca %s ms/adım | %s\n" $ad $i $(python3 -c "import json;print(round(json.load(open('ab/${ad}_$i.json'))['measured']['median_ms'],1))") "$(grep -o 'kayıp [0-9.]*' ab/${ad}_$i.log | tail -1)" >> ab/sonuc.txt
done; done
echo bitti > ab/bitti
EOF
setsid nohup bash ab/kos.sh > /dev/null 2>&1 < /dev/null &
echo "başladı (durum: hücre 22)"
```

### Hücre 22 · KİTAPLIK DENEYİ DURUMU · CPU

```bash
# 22 · DENEY A/B DURUMU (ANINDA BİTER, BEKLEMEZ) · CPU
cd /content/rgn/ab 2>/dev/null || { echo "koşu yok (hücre 21)"; exit 0; }
cat sonuc.txt 2>/dev/null
[ -f bitti ] && echo "bitti" || echo "sürüyor: $(ls -t *.log 2>/dev/null | head -1) → $(tail -c 120 $(ls -t *.log 2>/dev/null | head -1) 2>/dev/null)"
```

### Hücre 10 · ÇALIŞMA ZAMANINI KAPAT

Python hücresi. GPU serbest bırakılır; arka plan koşuları da biter.

```python
# 10 · ÇALIŞMA ZAMANINI KAPAT (GPU SERBEST BIRAKILIR; ARKA PLAN KOŞULARI DA BİTER)
from google.colab import runtime; runtime.unassign()
```

## 12. Üretim hızı yarışı: TR, transformers ve llama.cpp

Hazır modellerle (SmolLM2-135M, Qwen3-0.6B) 100 parçalık açgözlü üretim; TR (float32 ve bf16 ağırlık),
transformers (eager ve `cache_implementation="static"` ile derlenmiş), llama.cpp (f32 ve f16 GGUF); GPU ve
CPU. Betikler `tools/gpu/generation_race.sh` (llama.cpp'yi CUDA'yla derler, modelleri indirip GGUF'a
çevirir), `generation_race.tr`, `generation_race_hf.py`. Sonuçlar yol haritasında (Evre 5, "Üretim hızı").
~40 dakika; arka planda koşar.

```bash
%%bash
# 21 · ÜRETİM HIZI YARIŞI (ARKA PLANDA, ~40 DK) · GPU
setsid nohup bash /content/TCB/tools/gpu/generation_race.sh /content/tr/tr /content/yaris > /dev/null 2>&1 < /dev/null &
echo "başladı (durum: hücre 22)"
```

```bash
%%bash
# 22 · ÜRETİM HIZI YARIŞI DURUMU (ANINDA BİTER, BEKLEMEZ) · CPU
cat /content/yaris/sonuc.txt 2>/dev/null; [ -f /content/yaris/bitti ] && echo BİTTİ || echo sürüyor
```

## Geri bildirilecekler

1. 1. hücrenin son iki satırı (kart, matris çarpımı yolu).
2. 2. ve 3. hücrelerin çıktıları.
3. 4. hücrenin tablosu.
4. 6. hücrenin çıktısı (ms/adım, son kayıp, GPU kullanımı, bellek).
5. 7. hücrenin özeti; 8. hücre çalıştırıldıysa çıktısı.
6. 10. hücre (Evre 3 ölçümü): 10.2'nin tablosu, 10.3'ün tablosu, 10.4'ün iki tablosu.
7. 9. hücre çalıştırıldıysa: speed.json, son doğrulama kaybı ve samples.json'daki son örnek.
8. 11. bölüm (hedef model defteri): hücre 2 ve 4'ün satırları, hücre 8'in iki tablosu, hücre 9'un tablosu.
9. 12. bölüm (üretim yarışı): `/content/yaris/sonuc.txt`.

## Ortam değişkenleri

- `TR_GPU_PROFIL=1`: kernel ölçümü ve program sonunda özet (bkz. 7).
- `TR_GPU_ESZAMANLI=1`: her kernelden sonra eşzamanlar; eşzamansız bir hata tam olarak onu
  üreten kernelde bildirilir. Yavaştır; yalnızca teşhis için.
- `TR_GPU_CUBLASLT=/tam/yol/libcublasLt.so.12`: matris çarpımı kitaplığının yolu.
- `TR_GPU_OWN_GEMM=1`: kitaplık bulunsa da kendi matris çarpımı çekirdeğimiz.
- `TR_GPU_GEMV=0`: tek satırlık çarpımlar (M = 1) gemv çekirdeği yerine kitaplığa ya da döşemeli
  çekirdeğe gider (karşılaştırma; mikro kıyas üç yolu da ölçer, `gemv ...` satırları).
- `TR_GPU_ATTENTION=carpim`: dikkat birleşik kernel yerine matris çarpımı yolundan (karşılaştırma;
  mikro kıyas iki yolu da ölçer, `dikkat ... (matris yolu)` satırları).
- `TR_GPU_MICRO=1`: gpu_sinama'da mikro kıyasları açar (`TR_GPU_MICRO_BAND` kuramsal bant
  genişliği, GB/s; varsayılan 1800).
- `TR_GPU_SIM=1`: gerçek GPU yokken PTX benzetimini `"gpu"` cihazı olarak kurar.
- `TR_GPU_GC_THRESHOLD=MB`: GPU belleği için çöp toplama eşiği (varsayılan kart belleğinin 1/64'ü,
  256 MB–2 GB; `0` kapatır). Küçültmek tepe belleği düşürür, hız farkı küçüktür.
