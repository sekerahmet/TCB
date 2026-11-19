# Karakter modeli TR ile PyTorch (S13/char_model), dizüstü: 300 adımlı ve 0 adımlı programın duvar saati farkının adım
# başına payı (tools/bench/run.py'nin modeli; PyTorch tools/bench/py/char_model.py). Satır:
# docs/measurements/tensor-and-runtime.md, "Dizüstü i7-13700H". Taraflar: TR ve PyTorch, ikisi işlemcide.
# Aygıt ve sürüm: torch'un sürümü ve aygıtı (cpu) koşudan önce kayda yazılır.
#   bash tools/bench/baseline/machines/laptop/char-model.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_makine char-model
"$PY" -I -c "import torch; print('torch', torch.__version__, 'cuda', torch.cuda.is_available(), 'iş parçacığı', torch.get_num_threads())" |
  tee -a "$KAYIT/char-model-makine.txt"
olcu_kos char-model S13 --key S13/char_model
