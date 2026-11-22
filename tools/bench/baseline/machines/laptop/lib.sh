# Dizüstü ölçü betiklerinin ortak başı (tools/bench/baseline/machines/laptop/). Her betik bunu okur, sonra tek ölçüyü koşar:
# trlang ve record bu commit'ten derlenir, makine ve sürümler kayda yazılır, ölçü koşturucuyla (run.py --machine laptop)
# tools/record altında koşar, satır taslağı (--rows) basılır. Yollar ortam değişkenlerinden; depoya yerel yol girmez.
#   OLCU_MODELLER  modellerin klasörü (öntanımlı $USERPROFILE/models); her dosyanın yeri ve kimliği sources.json'da
#   OLCU_IS        iş klasörü (öntanımlı $USERPROFILE/TR_olcu/is): bin/, tools.json, models/
#   OLCU_KAYIT     kayıt kökü (öntanımlı $USERPROFILE/TR_olcu/kayit)
#   OLCU_SESLER    O-7'nin sabit seslerinin klasörü (öntanımlı $USERPROFILE/TR_olcu/sesler; Colab'daki A-11'in sounds/'u)
#   OLCU_LLAMA     llama.cpp b12124'ün klasörü (öntanımlı $USERPROFILE/tools/llama.cpp/b12124)
#   OLCU_WHISPER   whisper.cpp b5454'ün klasörü (öntanımlı $USERPROFILE/tools/whisper.cpp/b5454/Release)
# Asıl ölçü ölçü penceresinde koşar (YAN_DAL bölüm 7); pencere dışındaki koşu denemedir (OLCU_DENEME=1), sayısı kullanılmaz.
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO=$(cd "$HERE/../../../../.." && pwd)
SOURCES="$HERE/sources.json"
MODELLER=${OLCU_MODELLER:-$USERPROFILE/models}
IS=${OLCU_IS:-$USERPROFILE/TR_olcu/is}
KAYIT=${OLCU_KAYIT:-$USERPROFILE/TR_olcu/kayit}
LLAMA=${OLCU_LLAMA:-$USERPROFILE/tools/llama.cpp/b12124}
WHISPER=${OLCU_WHISPER:-$USERPROFILE/tools/whisper.cpp/b5454/Release}
PY=${PY:-python}
olcu() { echo "== $(date '+%H:%M:%S %z') $*"; }
not_al() { tee -a "$IS/hazirlik.txt"; }  # hazırlığın her satırı koşunun kendi kaydına da gider (olcu_kos)

# olcu_kur: trlang, record ve Go karşılaştırma programı (S13 bütün kalemlerini kurarken ister) bu commit'ten;
# tools.json'a onlar ve verilen araçlar (ad=yol). Hazırlık kaydı (hazirlik.txt) burada başlar.
olcu_kur() {
  mkdir -p "$IS/bin" "$KAYIT"
  : > "$IS/hazirlik.txt"
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
# Ş4 (DENETİM 944): değişik dosyalı ağaçta satırın Commit'i ölçülen kodu göstermez: ölçü kipinde çıkış 2, denemede uyarı.
olcu_makine() {
  local f="$KAYIT/$1-makine.txt" degisik
  degisik=$(git -C "$REPO" status --porcelain | grep -c .)
  {
    echo "tarih: $(date '+%Y-%m-%d %H:%M:%S %z')"
    echo "commit: $(git -C "$REPO" rev-parse HEAD) $degisik değişik dosya"
    echo "go: $(go version)"
    echo "işlemci: $(powershell -NoProfile -Command '(Get-CimInstance Win32_Processor).Name' 2>/dev/null | tr -d '\r')"
    echo "mantıksal işlemci: ${NUMBER_OF_PROCESSORS:-?}"
    echo "güç planı: $(powercfg -getactivescheme 2>/dev/null | tr -d '\r')"
    echo "Vantage kipi: ölçü penceresinde TCB'nin açılış mesajından"
  } > "$f"
  not_al < "$f"
  if [ "$degisik" != 0 ]; then
    if [ "${OLCU_DENEME:-}" = 1 ]; then
      olcu "UYARI: ağaçta $degisik değişik dosya (deneme; ölçü kipinde betik durur)"
    else
      olcu "ağaçta $degisik değişik dosya: satırın Commit'i ölçülen kodu göstermez, ölçü yok (Ş4)"; exit 2
    fi
  fi
}

# olcu_dost_yok: N1 (DENETİM 944): Ahmet'in açık bir Dost'u (asistan.tr'yi koşan trlang ya da Dost uygulaması) işlemciyi,
# belleği ve kapıyı paylaşır; yük denetimi boştaki Dost'u yakalamaz. Açıksa betik durur.
olcu_dost_yok() {
  local acik
  acik=$(powershell -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object { \$_.CommandLine -match 'asistan\.tr' -or \$_.Name -match '^dost' } | ForEach-Object { \"\$(\$_.ProcessId) \$(\$_.Name)\" }" 2>/dev/null | tr -d '\r')
  if [ -n "$acik" ]; then
    olcu "Dost açık ($acik): ölçüden önce kapanmalı (N1)"; exit 2
  fi
}

# olcu_kos <ad> <soru> [run.py'nin ek seçenekleri]: koşturucu tools/record altında; sonra satır taslağı. Koşunun kayıt
# klasörüne hazırlık (modellerin bağlandığı yollar, kimlikler, sürümler) ve pip list yazılır (Ş3).
olcu_kos() {
  local ad=$1 soru=$2; shift 2
  local r="$KAYIT/run-$ad"; mkdir -p "$r"
  olcu_dost_yok
  cp "$IS/hazirlik.txt" "$r/hazirlik.txt"
  "$PY" -m pip list --format=freeze > "$r/pip_list.txt" 2>/dev/null
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

# olcu_model <ad>: modelin dosyaları iş klasörüne (models/<ad>/), sources.json'daki yer ve kimlikle (model.py; ağa çıkmaz).
# Bulunamazsa ya da kimlik tutmazsa ölçü yok.
olcu_model() {
  "$PY" -I "$HERE/model.py" "$SOURCES" "$IS" "$MODELLER" "$1" | not_al
  [ "${PIPESTATUS[0]}" = 0 ] || { olcu "model $1 hazır değil"; exit 2; }
}

# olcu_arac <llama.cpp|whisper.cpp>: emsal ikilisinin sha256'sı ve sürüm satırı sources.json'dakiyle aynı olmalı (Ş3).
olcu_arac() {
  local dir; case $1 in llama.cpp) dir=$LLAMA;; whisper.cpp) dir=$WHISPER;; esac
  "$PY" -I "$HERE/sources.py" "$SOURCES" tool "$1" "$dir" | not_al
  [ "${PIPESTATUS[0]}" = 0 ] || { olcu "$1 sources.json'daki değil"; exit 2; }
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
    echo "ses $dosya: sha256 ${got:0:12} tuttu" | not_al
  done
}

# olcu_surum <paket>...: emsalin Python paketlerinin sürümü sources.json'dakiyle aynı olmalı; paket yoksa ya da sürümü
# başkaysa ölçü yok (Ş3). torch'un aygıtı da yazılır.
olcu_surum() {
  "$PY" -I "$HERE/sources.py" "$SOURCES" python "$@" | not_al
  [ "${PIPESTATUS[0]}" = 0 ] || { olcu "Python paketleri sources.json'daki değil"; exit 2; }
}
