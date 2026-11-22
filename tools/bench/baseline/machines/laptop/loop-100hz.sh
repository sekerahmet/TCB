# 100 Hz döngü (S13/loop_100hz), dizüstü: 1 000 tur, 2 sn tensör yükü; kaçan tur ve en geç kalış. Satır:
# docs/measurements/tensor-and-runtime.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok), işlemcide; model yok.
# Ölçü sırasında makinede başka iş olmamalı (ölçü penceresi); kayıt koşu öncesi ve sonrası süreç listesini tutar.
#   bash tools/bench/baseline/machines/laptop/loop-100hz.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_makine loop-100hz
olcu_kos loop-100hz S13 --key S13/loop_100hz
