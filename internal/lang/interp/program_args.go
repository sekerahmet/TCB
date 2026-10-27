package interp

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Betiğe komut satırında verilen argümanlar ('tr betik.tr --adım 5 veri' → ["--adım", "5",
// "veri"]). 'tr betik.tr', 'tr çalıştır', 'tr hata_ayıkla', 'tr derle' ile üretilen uygulama
// ve düzenleyicinin başlatma ayarındaki "args" aynı yoldan buraya gelir (1.4). Program
// argümanları okuyan yerleşik (komut_satırı, internal/stdlib/process) Args ile alır. Argüman
// verilip programın en dışından ulaşılan bir komut_satırı çağrısı yoksa program çalışmaz
// (bkz. command_line_reach.go).

// SetArgs records the program's arguments (once, before the program starts). Launchers' own
// arguments are left out (programArguments).
func (v *VM) SetArgs(arguments []string) {
	v.arguments = programArguments(arguments)
}

// Args returns a copy of the program's arguments.
func (v *VM) Args() []string {
	return append([]string(nil), v.arguments...)
}

// ProgramCommand is how the command that started the program is written (KomutAdi; when empty
// "tr <SourceFile>").
func (v *VM) ProgramCommand() string {
	if v.CommandName != "" {
		return v.CommandName
	}
	if v.SourceFile == "" {
		return "tr"
	}
	return "tr " + v.SourceFile
}

// ArgumentVerdict argüman verilmiş bir programın komut satırını okuyup okumadığına verilen
// karardır (1.4).
type ArgumentVerdict int

const (
	// ArgumentsUsable: argüman verilmedi ya da program komut satırını okuyabilir.
	ArgumentsUsable ArgumentVerdict = iota
	// ArgumentsNoOptions: program komut satırını okumuyor ve yalnız -h verildi: programın seçeneği
	// olmadığı yazılır, çıkış 0.
	ArgumentsNoOptions
	// ArgumentsUnread: argüman verilip program komut satırını okumuyor: kullanım hatası, çıkış 2.
	ArgumentsUnread
)

// CheckArguments argüman verilip programın en dışından ulaşılan bir komut_satırı çağrısı olup
// olmadığına bakar (1.4). cmd/tr'nin bütün çalıştırma yolları ve hata ayıklayıcının başlatma yolu
// (düzenleyicinin "args" ayarı) aynı kararı bununla verir; metin ArgumentsMessage'dadır.
func (v *VM) CheckArguments(prog *ast.Program) ArgumentVerdict {
	if len(v.arguments) == 0 || v.ReadsCommandLine(prog) {
		return ArgumentsUsable
	}
	if len(v.arguments) == 1 && v.arguments[0] == "-h" {
		return ArgumentsNoOptions
	}
	return ArgumentsUnread
}

// ArgumentsMessage CheckArguments'in kararının kullanıcıya yazılacak metnidir. Kullanım hatası
// verilen seçeneklerin adlarını ve programın başına eklenecek çağrıyı söyler; seçeneklerin değerleri
// (gizli olabilir) yazılmaz.
func (v *VM) ArgumentsMessage(verdict ArgumentVerdict) string {
	if verdict == ArgumentsNoOptions {
		return messages.Text(messages.H8665, v.ProgramCommand())
	}
	var options []string
	for _, a := range v.arguments {
		if name, ok := strings.CutPrefix(a, "--"); ok && name != "" {
			name, _, _ = strings.Cut(name, "=")
			options = append(options, "--"+name)
		}
	}
	example := "ad"
	if len(options) > 0 {
		example = strings.TrimPrefix(options[0], "--")
	}
	return messages.Text(messages.H8664, len(v.arguments), strings.Join(options, ", "), example,
		codes.NameRef(codes.KwVar), codes.NameRef(codes.FnCommandLine))
}

// programArguments başlatıcıların argümanlarını programın argümanlarından ayırır (1.4): ROS 2'nin
// --ros-args ile başlayıp -- ile ya da sonda biten blokları ve dağıtık eğitim başlatıcılarının
// verdiği --local-rank=N, --local_rank=N (aynı bilgi LOCAL_RANK ortam değişkeninde).
func programArguments(arguments []string) []string {
	var own []string
	inROSBlock := false
	for _, a := range arguments {
		switch {
		case inROSBlock:
			inROSBlock = a != "--"
		case a == rosArguments:
			inROSBlock = true
		case isLocalRank(a):
		default:
			own = append(own, a)
		}
	}
	return own
}

// rosArguments ROS 2'nin kendi argümanlarını başlatan işarettir.
const rosArguments = "--ros-args"

// isLocalRank argümanın --local-rank=N ya da --local_rank=N olduğunu söyler.
func isLocalRank(a string) bool {
	for _, prefix := range []string{"--local-rank=", "--local_rank="} {
		if rank, ok := strings.CutPrefix(a, prefix); ok && rank != "" && strings.Trim(rank, "0123456789") == "" {
			return true
		}
	}
	return false
}
