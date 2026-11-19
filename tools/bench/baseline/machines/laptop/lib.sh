# Dizüstü ölçü betiklerinin ortak başı (tools/bench/baseline/machines/laptop/). Her betik bunu okur, sonra tek ölçüyü koşar:
# trlang ve record bu commit'ten derlenir, makine ve sürümler kayda yazılır, ölçü koşturucuyla (run.py --machine laptop)
# tools/record altında koşar, satır taslağı (--rows) basılır. Yollar ortam değişkenlerinden; depoya yerel yol girmez.
#   OLCU_MODELLER  modellerin klasörü (öntanımlı $USERPROFILE/models)
#   OLCU_IS        iş klasörü (öntanımlı $USERPROFILE/TR_olcu/is): bin/, tools.json, models/
#   OLCU_KAYIT     kayıt kökü (öntanımlı $USERPROFILE/TR_olcu/kayit)
#   OLCU_SESLER    O-7'nin sabit seslerinin klasörü (öntanımlı $USERPROFILE/TR_olcu/sesler; Colab'daki A-11'in sounds/'u)
# Asıl ölçü ölçü penceresinde koşar (YAN_DAL bölüm 7); pencere dışındaki koşu denemedir, sayısı kullanılmaz.
set -u
REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)
MODELLER=${OLCU_MODELLER:-$USERPROFILE/models}
IS=${OLCU_IS:-$USERPROFILE/TR_olcu/is}
KAYIT=${OLCU_KAYIT:-$USERPROFILE/TR_olcu/kayit}
PY=${PY:-python}
olcu() { echo "== $(date '+%H:%M:%S %z') $*"; }

# olcu_kur: trlang, record ve Go karşılaştırma programı (S13 bütün kalemlerini kurarken ister) bu commit'ten;
# tools.json'a onlar ve verilen araçlar (ad=yol).
olcu_kur() {
  mkdir -p "$IS/bin" "$KAYIT"
  (cd "$REPO" && go build -o "$IS/bin/trlang.exe" ./cmd/trlang && go build -o "$IS/bin/record.exe" ./tools/record &&
    go build -o "$IS/bin/compare.exe" ./tools/bench/go) || { olcu "derleme düştü"; exit 2; }
  "$PY" -I -c "
import json, sys
d = {'trlang': sys.argv[1], 'go_compare': sys.argv[1].replace('trlang.exe', 'compare.exe')}
for kv in sys.argv[2:]:
    k, v = kv.split('=', 1); d[k] = v
json.dump(d, open(sys.argv[1].rsplit('/bin/', 1)[0] + '/tools.json', 'w'), indent=1)
" "$IS/bin/trlang.exe" "$@"
}

# olcu_makine <ad>: makinenin ve sürümlerin kaydı (kayıt kökünde <ad>-makine.txt): commit, Go, işlemci, güç planı.
olcu_makine() {
  local f="$KAYIT/$1-makine.txt"
  {
    echo "tarih: $(date '+%Y-%m-%d %H:%M:%S %z')"
    echo "commit: $(git -C "$REPO" rev-parse HEAD) $(git -C "$REPO" status --porcelain | grep -c . ) değişik dosya"
    echo "go: $(go version)"
    echo "işlemci: $(powershell -NoProfile -Command '(Get-CimInstance Win32_Processor).Name' 2>/dev/null | tr -d '\r')"
    echo "mantıksal işlemci: $NUMBER_OF_PROCESSORS"
    echo "güç planı: $(powercfg -getactivescheme 2>/dev/null | tr -d '\r')"
    echo "Vantage kipi: ölçü penceresinde TCB'nin açılış mesajından"
  } > "$f"
  cat "$f"
}

# olcu_kos <ad> <soru> [run.py'nin ek seçenekleri]: koşturucu tools/record altında; sonra satır taslağı.
olcu_kos() {
  local ad=$1 soru=$2; shift 2
  local r="$KAYIT/run-$ad"; mkdir -p "$r"
  sleep 20  # derlemenin ve sha256 hesabının yükü insin: koşturucunun yük denetimi koşudan önce ölçer
  local kos="$PY -I tools/bench/baseline/run.py --work $IS --machine laptop --out $r --question $soru"
  local rc
  if [ "${OLCU_DENEME:-}" = 1 ]; then
    # Deneme (Ahmet 2026-11-20): her şey bir kez. Emsali olan kalem uyuşma geçişiyle (her taraf bir koşu), emsalsiz kalem
    # --trial ile (tek koşu, ısınmasız) sınanır: betik, model ve aygıt; sayı kullanılmaz.
    olcu "$ad: $soru (deneme: her kalem bir kez)"
    (cd "$REPO" && $kos "$@" --agreement && $kos "$@" --trial)
    rc=$?
  else
    olcu "$ad: $soru"
    (cd "$REPO" && "$IS/bin/record.exe" --out "$KAYIT" --name "h1-laptop-$ad" -- $kos "$@")
    rc=$?
  fi
  olcu "$ad çıkışı: $rc"
  (cd "$REPO" && "$PY" -I tools/bench/baseline/run.py --rows --out "$r")
  return $rc
}

# olcu_model <ad>: questions.py MODELS'taki modelin dosyaları iş klasörüne (models/<ad>/), sha256 denetimiyle (model.py).
# Dizüstünde yalnız OLCU_MODELLER aranır (bütün modeller %USERPROFILE%\models altında). Bulunamazsa ya da sha256 tutmazsa ölçü yok.
olcu_model() {
  "$PY" -I "$(dirname "${BASH_SOURCE[0]}")/model.py" "$REPO" "$IS" "$MODELLER" "$1" ||
    { olcu "model $1 hazır değil"; exit 2; }
}

# olcu_ses <dosya=sha256>...: sabit sesler OLCU_SESLER'den iş klasörünün sounds/'una, sha256 denetimiyle; tutmazsa ölçü yok
# (sesler her makinede aynı baytlar; sha256'lar A-11'in sounds.json'undan).
olcu_ses() {
  local kaynak=${OLCU_SESLER:-$USERPROFILE/TR_olcu/sesler} kv dosya want got
  mkdir -p "$IS/sounds"
  for kv in "$@"; do
    dosya=${kv%%=*}; want=${kv#*=}
    [ -f "$kaynak/$dosya" ] || { olcu "ses yok: $kaynak/$dosya"; exit 2; }
    got=$(sha256sum < "$kaynak/$dosya" | cut -c1-64)
    [ "$got" = "$want" ] || { olcu "ses $dosya sha256 tutmadı: $got"; exit 2; }
    cp "$kaynak/$dosya" "$IS/sounds/$dosya"
    echo "ses $dosya: sha256 ${got:0:12} tuttu"
  done
}

# olcu_surum <paket>...: emsalin Python paketlerinin sürümü ve torch'un aygıtı kayda (<ad>-makine.txt'nin yanına).
olcu_surum() {
  "$PY" -I -c "
import importlib, sys
sys.stdout.reconfigure(encoding='utf-8')  # Windows'ta yönlendirilmiş çıktı cp1254'tü: 'aygıtı' bozuk basılıyordu
for m in sys.argv[1:]:
    try:
        mod = importlib.import_module(m)
        print(m, getattr(mod, '__version__', '?'))
    except Exception as e:
        print(m, 'YOK', type(e).__name__)
try:
    import torch; print('torch aygıtı: cpu (cuda', torch.cuda.is_available(), '), iş parçacığı', torch.get_num_threads())
except Exception:
    pass
" "$@" | tee -a "$KAYIT/surumler.txt"
}
