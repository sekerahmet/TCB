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
# Emsallerin en hızlı ayarı (Ş1) kayıt kökündeki en-hizli.json'dadır (en-hizli-ayar.sh); öbür betikler ölçüyü o ayarla koşar.
# Bir grup makine ya da emsal değişene kadar geçerlidir; ölçü yalnız kendi kalemlerinin gruplarını ister ve eksik olanları
# ölçüden önce kendisi tarar (H1e K1, K2, K4).
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO=$(cd "$HERE/../../../../.." && pwd)
SOURCES="$HERE/sources.json"
MODELLER=${OLCU_MODELLER:-$USERPROFILE/models}
IS=${OLCU_IS:-$USERPROFILE/TR_olcu/is}
KAYIT=${OLCU_KAYIT:-$USERPROFILE/TR_olcu/kayit}
LLAMA=${OLCU_LLAMA:-$USERPROFILE/tools/llama.cpp/b12124}
WHISPER=${OLCU_WHISPER:-$USERPROFILE/tools/whisper.cpp/b5454/Release}
AYAR="$KAYIT/en-hizli.json"
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
# klasörüne hazırlık (modellerin bağlandığı yollar, kimlikler, sürümler) ve pip list yazılır (Ş3). Emsallerin ayarı
# en-hizli.json'dan (Ş1); ölçü kipinde koşturucu yalnız seçilen kalemlerin gruplarını ister, her biri makine ve emsal
# değişene kadar geçerlidir (H1e K1, K2, K7).
olcu_kos() {
  local ad=$1 soru=$2; shift 2
  local r="$KAYIT/run-$ad"; mkdir -p "$r"
  olcu_dost_yok
  cp "$IS/hazirlik.txt" "$r/hazirlik.txt"
  "$PY" -m pip list --format=freeze > "$r/pip_list.txt" 2>/dev/null
  "$PY" --version > "$r/python_version.txt" 2>&1  # N4 (DENETİM 947)
  olcu_ayar_hazir "$ad" "$soru" "$@" || exit 2
  # K7 (H1e): --settings her zaman; koşturucu yalnız seçilen kalemlerin gruplarını ister, dosya yoksa boş sayılır. Grubun
  # kimliği bu koşunun makine kaydından ve sources.json'dan (K2).
  local ayar=(--settings "$AYAR" --sources "$SOURCES" --machine-file "$KAYIT/$ad-makine.txt")
  sleep 20  # derlemenin ve sha256 hesabının yükü insin: koşturucunun yük denetimi koşudan önce ölçer
  local kos="$PY -I tools/bench/baseline/run.py --work $IS --machine laptop --out $r --question $soru"
  local rc
  if [ "${OLCU_DENEME:-}" = 1 ]; then
    # Deneme (Ahmet 2026-11-20): her şey bir kez. Emsali olan kalem uyuşma geçişiyle (her taraf bir koşu), emsalsiz kalem
    # --trial ile (tek koşu, ısınmasız) sınanır: betik, model ve aygıt; sayı kullanılmaz.
    olcu "$ad: $soru (deneme: her kalem bir kez)"
    (cd "$REPO" && $kos "$@" "${ayar[@]}" --agreement && $kos "$@" "${ayar[@]}" --trial)
    rc=$?
  else
    olcu "$ad: $soru"
    (cd "$REPO" && "$IS/bin/record.exe" --out "$KAYIT" --name "h1-laptop-$ad" -- $kos "$@" "${ayar[@]}")
    rc=$?
  fi
  [ -f "$AYAR" ] && cp "$AYAR" "$r/en-hizli.json"  # koşunun aldığı ayar, benimsenen grubun kimliğiyle (K9) kaydına
  olcu "$ad çıkışı: $rc"
  (cd "$REPO" && "$PY" -I tools/bench/baseline/run.py --rows --out "$r")
  return $rc
}

# olcu_ayar_hazir <ad> <soru> [run.py'nin ek seçenekleri]: K4 (H1e). Ölçü kipinde koşturucuya seçilen kalemlerin eksik
# gruplarını sorar (run.py --settings-missing; geçersiz grup da eksiktir, eski dosyanın grubu kendi kaydıyla benimsenir)
# ve tarama betiğini yalnız o gruplarla, kendi iş klasöründe ($IS-tara) koşar: ölçünün tools.json'u ve hazırlığı
# değişmez. Koşturucu durursa (tek koşulu grup) ya da tarama düşerse ölçü yok. Deneme taramaz, dosyayı varsa kullanır.
TARA_GRUPLARI=("llama.cpp read" "llama.cpp gen" "llama.cpp S3" "faster-whisper float32" "faster-whisper int8" "whisper.cpp" "onnxruntime")
TARA_BETIK=${TARA_BETIK:-$HERE/en-hizli-ayar.sh}
olcu_ayar_hazir() {
  local ad=$1 soru=$2; shift 2
  [ "${OLCU_DENEME:-}" = 1 ] && return 0
  local r="$KAYIT/run-$ad" cikti rc satir eksik=()
  mkdir -p "$r"
  cikti=$(cd "$REPO" && "$PY" -I tools/bench/baseline/run.py --work "$IS" --machine laptop --out "$r" --question "$soru" "$@" \
    --settings "$AYAR" --sources "$SOURCES" --machine-file "$KAYIT/$ad-makine.txt" --settings-missing)
  rc=$?
  [ -n "$cikti" ] && echo "$cikti"
  [ "$rc" = 0 ] || { olcu "$ad: ayar dosyası bu ölçüye uymuyor (çıkış $rc), ölçü yok (K7)"; return 2; }
  while IFS= read -r satir; do
    satir=${satir%$'\r'}  # Windows'ta Python satırı \r\n ile biter
    [[ $satir == "MISSING "* ]] && eksik+=("${satir#MISSING }")
  done <<< "$cikti"
  [ ${#eksik[@]} = 0 ] && return 0
  olcu "$ad: eksik gruplar taranıyor: ${eksik[*]} (K4)"
  OLCU_IS="$IS-tara" bash "$TARA_BETIK" "${eksik[@]}" || { olcu "$ad: eksik grupların taraması düştü, ölçü yok (K4)"; return 2; }
}

# olcu_tara <grup>...: Ş1'in kısa taraması (run.py --scan): her grubun en hızlı ayarı en-hizli.json'a (var olana eklenir;
# gruplar ayrı komutlarla da koşabilir). Kural koşudan önce yazılı: varsayılan, 6 ve mantıksal sayı (S3'te -fa 0 ve 1),
# her ayar bir ısınma, sonra 3 koşu ayarlar arasında dönerek; sonraki ayar ancak bütün koşuları öncekinin bütün
# koşularından kısaysa seçilir (H1e K6); denemede her ayar bir kez, ısınmasız. Her grup bitince kimliğiyle (makine kaydı ve
# sources.json; H1e K2) yazılır; ölçü yalnız kimliği tutan grubu alır (run.py settings_gate). K5 (H1e): dosya silinmez,
# taranmayan grupların girdisi korunur; tarama yarıda düşerse biten gruplar yine yazılır. Deneme kendi klasörüne
# (run-en-hizli-ayar-deneme) yazar, kayıt kökünün dosyasına dokunmaz: tek koşulu grup ölçüyü durdururdu (K3).
olcu_tara() {
  local r="$KAYIT/run-en-hizli-ayar"
  [ "${OLCU_DENEME:-}" = 1 ] && r="$KAYIT/run-en-hizli-ayar-deneme"
  mkdir -p "$r"
  olcu_dost_yok
  cp "$IS/hazirlik.txt" "$r/hazirlik.txt"
  "$PY" -m pip list --format=freeze > "$r/pip_list.txt" 2>/dev/null
  "$PY" --version > "$r/python_version.txt" 2>&1
  if [ -f "$AYAR" ]; then cp "$AYAR" "$r/en-hizli.json"; else rm -f "$r/en-hizli.json"; fi  # eski kopya dirilmesin
  local g args=() deneme=()
  for g in "$@"; do args+=(--scan "$g"); done
  [ "${OLCU_DENEME:-}" = 1 ] && deneme=(--trial)
  sleep 20
  olcu "en-hizli-ayar: $* (${OLCU_DENEME:+deneme: her ayar bir kez})"
  local rc
  (cd "$REPO" && "$PY" -I tools/bench/baseline/run.py --work "$IS" --machine laptop --out "$r" "${args[@]}" "${deneme[@]}" \
    --sources "$SOURCES" --machine-file "$KAYIT/en-hizli-ayar-makine.txt")
  rc=$?
  [ "${OLCU_DENEME:-}" != 1 ] && [ -f "$r/en-hizli.json" ] && cp "$r/en-hizli.json" "$AYAR"
  [ "$rc" = 0 ] || { olcu "en-hizli-ayar: tarama düştü (çıkış $rc); biten gruplar yazıldı"; exit 2; }
  olcu "en hızlı ayar: $("$PY" -I -c 'import json,sys; print({k: v for k, v in json.load(open(sys.argv[1], encoding="utf-8")).items() if not k.startswith("_")})' "$r/en-hizli.json")"
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
