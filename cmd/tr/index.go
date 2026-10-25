package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/lib"
)

// 'tr dizin' komutu ve alt komutları (tanım, kullananlar, sınayanlar, etki): TR kodunun dizinini
// (internal/devtools/codeindex) kurar ve ona sorar. Burada yalnızca argümanlar, çıktı biçimi ve
// çıkış kodu vardır.

// defaultIndexLimit listelerdeki varsayılan en çok satırdır.
const defaultIndexLimit = 12

// indexOptions komutun seçenekleridir.
type indexOptions struct {
	json    bool   // K103 (--json)
	limit   int    // K107 (--sınır)
	all     bool   // K108 (--tümü)
	root    string // K109 (--kök)
	kind    string // K110 (--tür)
	noIndex bool   // K111 (--dizin-yok)
	status  bool   // K112 (--durum)
	clear   bool   // K113 (--temizle)
	noText  bool   // K114 (--metin-yok)
	term    bool   // K115 (--terim)

	limitSet bool // --sınır açıkça verildi
}

// indexValueOptions değer isteyen seçeneklerdir.
var indexValueOptions = []codes.Code{codes.K107, codes.K109, codes.K110}

// indexCommand 'tr dizin [alt komut] [ad] [seçenekler]' komutudur.
func indexCommand(args []string) {
	sub := codes.None
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		k, ok := codes.FindSubcommand(args[0])
		if !ok {
			exitWithError(messages.Text(messages.H8821, codes.NameRef(codes.K015), args[0], subcommandNames()))
		}
		sub, args = k, args[1:]
	}
	opts, names := parseIndexArguments(sub, args)
	root := opts.root
	if root == "" {
		root = codeindex.FindRoot(".")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		exitWithError(messages.Text(messages.H8826, err))
	}
	if sub == codes.None {
		indexOverview(root, opts)
		return
	}
	idx, _ := loadIndex(root, opts)
	runIndexQuery(codeindex.NewReader(idx), sub, names, root, opts)
}

// indexBuildOptions dizinin kurulum seçenekleridir (gömülü kitaplık dahil).
func indexBuildOptions() codeindex.Options { return codeindex.Options{Library: lib.Files} }

// loadIndex dizini önbellekle getirir: --dizin-yok varsa kullanılabilir önbellek olduğu gibi okunur
// (yoksa kurulur); yoksa değişen dosyalar tazelenir.
func loadIndex(root string, o indexOptions) (*codeindex.Index, codeindex.Stats) {
	if o.noIndex {
		if idx, ok := codeindex.LoadCached(root, indexBuildOptions()); ok {
			return idx, codeindex.Stats{Files: len(idx.Files), FromCache: true}
		}
	}
	idx, stats, err := codeindex.BuildCached(root, indexBuildOptions())
	if err != nil {
		exitWithError(messages.Text(messages.H8826, err))
	}
	return idx, stats
}

// subcommandNames etkin dildeki alt komut adlarıdır (hata iletisi için).
func subcommandNames() string {
	var names []string
	for _, k := range []codes.Code{codes.K016, codes.K017, codes.K018, codes.K019, codes.K020} {
		names = append(names, codes.DisplayName(k))
	}
	return strings.Join(names, ", ")
}

// parseIndexArguments seçenekleri ve (alt komutlarda) sorgu adını ayırır; hata varsa yazıp 1 ile çıkar.
func parseIndexArguments(sub codes.Code, args []string) (indexOptions, []string) {
	o := indexOptions{limit: defaultIndexLimit}
	var names []string
	allowed := []codes.Code{codes.K103, codes.K107, codes.K108, codes.K109, codes.K110, codes.K111}
	switch sub {
	case codes.None:
		allowed = []codes.Code{codes.K109, codes.K112, codes.K113}
	case codes.K020:
		allowed = []codes.Code{codes.K103, codes.K107, codes.K108, codes.K109, codes.K111, codes.K114, codes.K115}
	}
	command := codes.DisplayName(codes.K015)
	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		if !strings.HasPrefix(arg, "--") {
			names = append(names, arg)
			continue
		}
		var k codes.Code
		var value string
		k, value, args = parseLongOption(command, arg, args, allowed, func(k codes.Code) bool { return slices.Contains(indexValueOptions, k) })
		applyIndexOption(&o, k, value)
	}
	if o.all {
		o.limit = 0
	}
	switch {
	case sub == codes.K020 && len(names) != 2:
		exitWithError(messages.Text(messages.H8864, command, codes.NameRef(sub), "head view"))
	case sub != codes.None && sub != codes.K020 && len(names) != 1:
		exitWithError(messages.Text(messages.H8820, command, codes.NameRef(sub), "Muon"))
	}
	return o, names
}

func applyIndexOption(o *indexOptions, k codes.Code, value string) {
	switch k {
	case codes.K103:
		o.json = true
	case codes.K107:
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			exitWithError(messages.Text(messages.H8822, codes.OptionFlag(k), value))
		}
		o.limit, o.limitSet = n, true
	case codes.K108:
		o.all = true
	case codes.K109:
		o.root = value
	case codes.K110:
		o.kind = value
	case codes.K111:
		o.noIndex = true
	case codes.K112:
		o.status = true
	case codes.K113:
		o.clear = true
	case codes.K114:
		o.noText = true
	case codes.K115:
		o.term = true
	}
}

// indexOverview alt komutsuz 'tr dizin': dizini kurar ve özetler; --durum önbelleğin durumunu, --temizle
// önbelleği siler.
func indexOverview(root string, o indexOptions) {
	fmt.Println(messages.Text(messages.H8824, root))
	switch {
	case o.clear:
		removed, err := codeindex.ClearCache(root)
		if err != nil {
			exitWithError(messages.Text(messages.H8826, err))
		}
		if removed {
			fmt.Println(messages.Text(messages.H8828))
		} else {
			fmt.Println(messages.Text(messages.H8829))
		}
		return
	case o.status:
		st, err := codeindex.Status(root, indexBuildOptions())
		if err != nil {
			exitWithError(messages.Text(messages.H8826, err))
		}
		if !st.Exists {
			fmt.Println(messages.Text(messages.H8827))
			return
		}
		fmt.Println(messages.Text(messages.H8832, st.Files, len(st.Changed), len(st.Added), len(st.Removed)))
		for _, group := range [][]string{st.Changed, st.Added, st.Removed} {
			for _, f := range group {
				fmt.Println("  " + f)
			}
		}
		return
	}
	idx, stats := loadIndex(root, o)
	if stats.FromCache {
		fmt.Println(messages.Text(messages.H8834, stats.Files))
	} else {
		fmt.Println(messages.Text(messages.H8833, stats.Parsed, stats.Files))
	}
	fmt.Println(messages.Text(messages.H8823, len(idx.Files), len(idx.Nodes), len(idx.Edges), len(idx.Occurrences), len(idx.Unresolved), len(idx.Skipped)))
	for i, s := range idx.Skipped {
		if o.limit > 0 && i >= o.limit {
			break
		}
		fmt.Println("  " + messages.Text(messages.H8825, s.File, s.Reason))
	}
	for i, u := range idx.Unresolved {
		if o.limit > 0 && i >= o.limit {
			break
		}
		fmt.Println("  " + messages.Text(messages.H8831, u.Name, u.Pos.String()))
	}
}

// runIndexQuery bir alt komutu çalıştırır ve sonucu yazar; çıkış kodu sonucun durumudur
// (0 tamam, 3 belirsiz ad, 4 bulunamadı).
func runIndexQuery(r *codeindex.Reader, sub codes.Code, names []string, root string, o indexOptions) {
	query := names[0]
	qo := codeindex.QueryOptions{Limit: o.limit, Kind: o.kind}
	var res *codeindex.Result
	switch sub {
	case codes.K016:
		res = r.Definition(query, qo)
	case codes.K017:
		res = r.References(query, qo)
	case codes.K018:
		res = r.TestedBy(query, qo)
	case codes.K019:
		res = r.Impact(query, qo)
	case codes.K020:
		ro := codeindex.RenameOptions{Root: root, NoText: o.noText, Term: o.term}
		if o.limitSet {
			ro.Limit = o.limit
		}
		res = r.RenameImpact(query, names[1], ro)
	}
	if o.json {
		data, err := codeindex.EncodeJSON(res)
		if err != nil {
			exitWithError(messages.Text(messages.H8826, err))
		}
		os.Stdout.Write(data)
	} else {
		if res.Command == codeindex.CommandRename {
			fmt.Print(codeindex.RenameText(res))
		} else {
			fmt.Print(codeindex.Text(res))
		}
	}
	if code := codeindex.ExitCode(res.Status); code != codeindex.ExitOK {
		os.Exit(code)
	}
}
