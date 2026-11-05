#!/usr/bin/env python3
"""İki commit'in Go kıyasını gerileme kapısı olarak karşılaştırır.

Aynı kod, fonksiyonun bellekteki hizası değişince yüzde 6-10 oynayabilir (BF16TrainStep, widenSpan:
docs/measurements.md). Bu yüzden iki taraf da -funcalign=64 ile derlenir: fark kodun farkı olur,
o commit'in yerleşiminin farkı olmaz. Bayrak yalnız kıyas içindir; kullanıcının derlemesini bağlamaz.

Üç kol nöbetleşe koşar: taban, tabanın kopyası (aynı ikili), dal. Fark ancak kopyanın farkının
dışındaysa ve iki kolun çeyrekleri ayrıksa gerçek sayılır. Gerçek yavaşlamada çıkış kodu 1.

  python tools/bench/compare_commits.py --base <commit> --head <commit> --bench BF16TrainStep
  python tools/bench/compare_commits.py --verdict taban.txt dal.txt kopya.txt

--plain hizasız kolları da ekler: kullanıcının ikilisinde görülen fark budur.
Hız ölçüsü boş makinede ya da Colab'da koşar, paylaşımlı bilgisayarda değil.
"""
import argparse
import collections
import os
import re
import shutil
import statistics
import subprocess
import sys
import tempfile

ALIGN_FLAG = '-ldflags=-funcalign=64'
LINE = re.compile(r'^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns/op')


def parse(lines):
    """Kıyas çıktısının satırlarından {kıyas: [ns, ...]} kurar."""
    out = collections.defaultdict(list)
    for line in lines:
        m = LINE.match(line)
        if m:
            out[m.group(1)].append(float(m.group(2)))
    return out


def quartiles(samples):
    return statistics.quantiles(samples, n=4)


def verdict(base, head, copy):
    """(fark yüzdesi, kopyanın farkı yüzdesi, gerçek mi): kapının tek kuralı."""
    qb, qh, qc = quartiles(base), quartiles(head), quartiles(copy)
    diff = 100 * (qh[1] / qb[1] - 1)
    noise = 100 * (qc[1] / qb[1] - 1)
    apart = qh[0] > qb[2] or qh[2] < qb[0]
    return diff, noise, abs(diff) > abs(noise) and apart


def report(name, base, head, copy):
    """Bir kıyasın kararını yazar; gerçek yavaşlamada True döner."""
    diff, noise, real = verdict(base, head, copy)
    state = 'gerçek' if real else 'gerçek değil'
    print('%s: fark %+.1f%%, kopya %+.1f%%, %s' % (name, diff, noise, state))
    return real and diff > 0


def show_arm(arm, samples, reference):
    q = quartiles(samples)
    print('  %-16s n %2d ortanca %.0f (Ç1 %.0f, Ç3 %.0f) %+.1f%%' % (
        arm, len(samples), q[1], q[0], q[2], 100 * (q[1] / reference - 1)))


def build(commit, flags, target, package, work):
    """commit'in kıyas ikilisini kurar; koşulacağı paket klasörünü döndürür."""
    tree = os.path.join(work, 'tree_' + commit)
    if not os.path.isdir(tree):
        subprocess.run(['git', 'worktree', 'add', '-q', '--detach', tree, commit], check=True)
    subprocess.run(['go', 'test', '-c', *flags, '-o', target, './' + package], cwd=tree, check=True)
    return os.path.join(tree, package)


def symbol_address(binary, symbol):
    out = subprocess.run(['go', 'tool', 'nm', '-n', binary], capture_output=True, text=True, check=True)
    for line in out.stdout.splitlines():
        if line.endswith('.' + symbol):
            return int(line.split()[0], 16)
    return None


def arms_of(args, work):
    """Kolları kurar: {kol: (ikili, klasör)}. Hizalı kollar kapının, hizasızlar teşhisin."""
    arms = {}
    kinds = [('@64', [ALIGN_FLAG])] + ([('', [])] if args.plain else [])
    for suffix, flags in kinds:
        for side, commit in (('base', args.base), ('head', args.head)):
            binary = os.path.join(work, side + suffix.replace('@', '_') + '.test')
            arms[side + suffix] = (binary, build(commit, flags, binary, args.package, work))
            if args.symbol:
                a = symbol_address(binary, args.symbol)
                print('%s%s %s: %s' % (side, suffix, args.symbol, 'yok' if a is None else '%x, 64\'e göre %d' % (a, a % 64)))
        copy = os.path.join(work, 'copy' + suffix.replace('@', '_') + '.test')
        shutil.copy(arms['base' + suffix][0], copy)
        arms['copy' + suffix] = (copy, arms['base' + suffix][1])
    return arms


def run_arms(args, arms):
    """Kolları nöbetleşe koşar: her turda sıra bir kayar."""
    names = list(arms)
    samples = {arm: collections.defaultdict(list) for arm in names}
    for rnd in range(args.rounds):
        for i in range(len(names)):
            arm = names[(i + rnd) % len(names)]
            binary, cwd = arms[arm]
            cmd = [binary, '-test.run', 'XXX', '-test.bench', args.bench, '-test.count', str(args.count)]
            out = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=args.timeout)
            if out.returncode:
                sys.exit('%s düştü (%d):\n%s' % (arm, out.returncode, out.stdout + out.stderr))
            with open(os.path.join(args.out, 'bench_%s_%d.txt' % (arm.replace('@', '_'), rnd)), 'w', encoding='utf-8', newline='') as f:
                f.write(out.stdout)
            for name, values in parse(out.stdout.splitlines()).items():
                samples[arm][name].extend(values)
            print('tur %d %s bitti' % (rnd, arm), flush=True)
    return samples


def compare(args):
    os.makedirs(args.out, exist_ok=True)
    work = tempfile.mkdtemp(prefix='compare_commits_')
    try:
        arms = arms_of(args, work)
        samples = run_arms(args, arms)
    finally:
        for name in os.listdir(work):
            if name.startswith('tree_'):
                subprocess.run(['git', 'worktree', 'remove', '--force', os.path.join(work, name)], check=False)
        shutil.rmtree(work, ignore_errors=True)
    slower = False
    for name in sorted(samples['base@64']):
        print(name)
        for suffix in ('@64', '') if args.plain else ('@64',):
            reference = statistics.median(samples['base' + suffix][name])
            for side in ('base', 'copy', 'head'):
                show_arm(side + suffix, samples[side + suffix][name], reference)
        if args.plain:
            report('hizasız', samples['base'][name], samples['head'][name], samples['copy'][name])
        slower |= report('hizalı', samples['base@64'][name], samples['head@64'][name], samples['copy@64'][name])
    return 1 if slower else 0


def verdict_of_files(paths):
    loaded = []
    for path in paths:
        with open(path, encoding='utf-8') as f:
            loaded.append(parse(f))
    slower = False
    for name in sorted(loaded[0]):
        slower |= report(name, loaded[0][name], loaded[1][name], loaded[2][name])
    return 1 if slower else 0


def main():
    sys.stdout.reconfigure(encoding='utf-8')  # -I, PYTHONIOENCODING'i görmez
    p = argparse.ArgumentParser(description='İki commit\'in Go kıyası: hizalı kollar, kopya kolu, nöbetleşe.')
    p.add_argument('--base', help='taban commit')
    p.add_argument('--head', help='dal commit')
    p.add_argument('--bench', help='-test.bench düzenli ifadesi')
    p.add_argument('--package', default='internal/compute/tensor', help='paket klasörü, depo köküne göre')
    p.add_argument('--symbol', help='adresi yazılacak fonksiyon (ör. widenSpan)')
    p.add_argument('--rounds', type=int, default=6, help='tur sayısı')
    p.add_argument('--count', type=int, default=5, help='turda tekrar')
    p.add_argument('--timeout', type=int, default=300, help='bir koşunun saniye sınırı')
    p.add_argument('--out', default='compare_commits_out', help='ham çıktı klasörü')
    p.add_argument('--plain', action='store_true', help='hizasız kolları da koş')
    p.add_argument('--verdict', nargs=3, metavar=('TABAN', 'DAL', 'KOPYA'), help='kayıtlı üç çıktıya karar ver')
    args = p.parse_args()
    if args.verdict:
        return verdict_of_files(args.verdict)
    if not (args.base and args.head and args.bench):
        p.error('--base, --head ve --bench gerekli')
    return compare(args)


if __name__ == '__main__':
    sys.exit(main())
