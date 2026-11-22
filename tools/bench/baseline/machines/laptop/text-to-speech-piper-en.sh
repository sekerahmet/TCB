# Yazıyı sese çevirme, Piper İngilizce (S7/en), dizüstü: texts.json'un üç cümlesi, english_phoneme_ids.json'un kimlikleriyle,
# gürültü 0; TR (piper.tr) ile onnxruntime (iş parçacığı en-hizli.json'dan, Ş1; ayar dosyasız denemede öntanımlı, varsayılan ve 6 iş parçacığı),
# ABBA 10 koşu. Satır: docs/measurements/text-to-speech.md, "Dizüstü i7-13700H". Taraflar: ikisi işlemcide (onnxruntime
# CPUExecutionProvider; sağlayıcı emsalin çıktısında). Model rhasspy/piper-voices en_US-lessac-medium (yeri ve kimliği
# sources.json'da), %USERPROFILE%\models\piper-en-lessac-medium\; S7 kurulurken Türkçe sesin kimlikleri de yazıldığı için
# piper-tr-dfki-medium da bağlanır. Deneme kipinde yalnız uyuşma geçişi (her taraf bir koşu).
#   bash tools/bench/baseline/machines/laptop/text-to-speech-piper-en.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model piper-tr-dfki-medium
olcu_model piper-en-lessac-medium
olcu_makine text-to-speech-piper-en
olcu_surum onnxruntime numpy
olcu_kos text-to-speech-piper-en S7 --key S7/en
