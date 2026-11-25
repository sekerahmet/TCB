#!/usr/bin/env bash
# Metin üretimi, Qwen3-4B Q4_K_M (Qwen/Qwen3-4B-GGUF), Colab L4, GPU: bu commit taban commit'e (OLCU_TABAN) karşı, aynı kartta
# ve aynı koşuda (run.py --machine l4gpu --base-trlang; ısınma ve ABBA koşturucuda): S15 ısınmış okuma (512 token), S3 korunan
# önbellek 700 ve 4 100 derinlikte 40 token okuma, S1 boş önbellekte 512 token okuma ve 128 adım üretim.
# Satır: docs/measurements/text-generation.md, "Colab L4, GPU". Taraflar: iki TR ikilisi, ikisi GPU'da; emsal yok.
#   OLCU_TABAN    taban commit (zorunlu); çalışma ağacı iş klasöründe açılır, ikilisi oradan derlenir
#   OLCU_IS       iş klasörü (öntanımlı /content/olcu_is): bin/, taban/, models/, tools.json, hazirlik.txt
#   OLCU_KAYIT    kayıt kökü (öntanımlı /content/olcumler)
#   OLCU_DENEME=1 her kalem bir kez, ısınmasız (run.py --trial): sayı kullanılmaz
# Kurar: Go (go.mod'un araç zinciri, go.dev) ve model (run.py --fetch: deposu ve revision'ı questions.py'de).
# Değişik dosyalı ağaçta durur (denemede uyarır). Ek seçenek verilirse soruların yerine geçer.
#   OLCU_TABAN=<commit> bash tools/bench/baseline/machines/l4/text-generation-base-qwen3-4b-q4km.sh [--question S15 --key S15/]
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO=$(cd "$HERE/../../../../.." && pwd)
AD=text-generation-base-qwen3-4b-q4km
IS=${OLCU_IS:-/content/olcu_is}
KAYIT=${OLCU_KAYIT:-/content/olcumler}
PY=${PY:-python3}
MODEL=qwen3-4b-q4km
GGUF=$IS/models/$MODEL/Qwen3-4B-Q4_K_M.gguf
olcu() { echo "== $(date '+%H:%M:%S %z') $*"; }
dur() { olcu "$AD: $*, ölçü yok"; exit 2; }
[ -n "${OLCU_TABAN:-}" ] || dur "OLCU_TABAN verilmedi"
mkdir -p "$IS/bin" "$KAYIT"
cd "$REPO" || dur "depo yok"
# Kart Go'dan önce denetlenir: yanlış makinede hiçbir şey kurulmaz ve silinmez.
KART=$(nvidia-smi --query-gpu=name,driver_version,memory.total,power.limit --format=csv,noheader | head -1)
case $KART in *L4*) ;; *) dur "kart L4 değil ($KART)";; esac
export GOCACHE=${GOCACHE:-/root/.cache/go-build} GOPATH=${GOPATH:-/root/go} PYTHONIOENCODING=utf-8

GO=$(sed -n 's/^toolchain //p' go.mod | tr -d '\r')
export PATH=/usr/local/go/bin:$PATH
go version 2>/dev/null | grep -q "$GO " || {
  rm -rf /usr/local/go
  wget -q "https://go.dev/dl/$GO.linux-amd64.tar.gz" -O /tmp/go.tgz && tar -C /usr/local -xzf /tmp/go.tgz || dur "Go kurulmadı"
}
DEGISIK=$(git status --porcelain --untracked-files=no | grep -c .)
[ "$DEGISIK" = 0 ] || { [ "${OLCU_DENEME:-}" = 1 ] && olcu "UYARI: ağaçta $DEGISIK değişik dosya (deneme)"; } ||
  dur "ağaçta $DEGISIK değişik dosya: satırın Commit'i ölçülen kodu göstermez"

git worktree prune
[ -d "$IS/taban" ] && git worktree remove --force "$IS/taban"
git worktree add -q --detach "$IS/taban" "$OLCU_TABAN" || dur "taban $OLCU_TABAN açılmadı"
(cd "$IS/taban" && go build -trimpath -o "$IS/bin/trlang_taban" ./cmd/trlang) || dur "taban derlenmedi"
go build -trimpath -o "$IS/bin/trlang" ./cmd/trlang && go build -o "$IS/bin/compare" ./tools/bench/go &&
  go build -o "$IS/bin/record" ./tools/record || dur "derleme düştü"
"$PY" -I -c "import json, sys; w = sys.argv[1]; json.dump({'trlang': w + '/bin/trlang', 'go_compare': w + '/bin/compare', 'llama_bench': 'false', 'llama_completion': 'false'}, open(w + '/tools.json', 'w'), indent=1)" "$IS"
"$PY" -I tools/bench/baseline/run.py --work "$IS" --fetch --models "$MODEL" || dur "model inmedi"
{
  echo "tarih: $(date '+%Y-%m-%d %H:%M:%S %z')"
  echo "commit: $(git rev-parse HEAD) $DEGISIK değişik dosya"
  echo "taban: $(git -C "$IS/taban" rev-parse HEAD)"
  echo "go: $(go version)"
  echo "kart: $KART"
  echo "model: $(basename "$GGUF") sha256 $(sha256sum "$GGUF" | cut -c1-12)"
  echo "python: $("$PY" --version 2>&1)"
} | tee "$IS/hazirlik.txt"

[ $# = 0 ] && set -- --question S15 --question S3 --question S1 --key S15/$MODEL --key S3/$MODEL --key S1/$MODEL/
R=$KAYIT/run-$AD
rm -rf "$R"; mkdir -p "$R"
cp "$IS/hazirlik.txt" "$R/"
KOS=("$PY" -I tools/bench/baseline/run.py --work "$IS" --machine l4gpu --out "$R" --base-trlang "$IS/bin/trlang_taban" "$@")
sleep 20  # derlemenin ve sha256 hesabının yükü insin
olcu "$AD ${OLCU_DENEME:+(deneme: her kalem bir kez)}"
if [ "${OLCU_DENEME:-}" = 1 ]; then
  "${KOS[@]}" --trial
else
  "$IS/bin/record" --out "$KAYIT" --name "l4-$AD" -- "${KOS[@]}"
fi
rc=$?
olcu "$AD çıkışı: $rc"
"$PY" -I tools/bench/baseline/run.py --rows --out "$R"
exit $rc
