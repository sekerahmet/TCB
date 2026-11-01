// compare, hız kıyas setinin (tools/bench) Rust sürümüdür: TR, Python ve Go programlarının yaptığı işleri
// aynı sonuç değerleriyle yapar ve sürelerini yazar ("ad değer ms"). Yalnız standart kitaplık; tools/bench/run.py
// `rustc -C opt-level=3` ile derleyip çalıştırır (rustc yoksa Rust sütunu atlanır).
//
// Layer: tools (tools); kullanıcıya gitmez.
use std::collections::HashMap;
use std::env;
use std::hint::black_box;
use std::time::Instant;

// Geçen süre mikrosaniye çözünürlükte, milisaniye olarak (Go sürümü gibi): derlenmiş sürümde süre çoğu kez 1 ms'nin altında.
fn ms(t: Instant) -> String {
    format!("{:.3}", t.elapsed().as_micros() as f64 / 1000.0)
}

fn fib(n: i64) -> i64 {
    if n < 2 {
        n
    } else {
        fib(n - 1) + fib(n - 2)
    }
}

fn main() {
    let ad = env::args().nth(1).unwrap_or_default();
    match ad.as_str() {
        "dongu" => {
            let t = Instant::now();
            let mut toplam: i64 = 0;
            // Her adımın değeri black_box'tan geçer: LLVM sınır bilinmese de 0+1+…+(n−1) toplamını n(n−1)/2'ye
            // indirebiliyor (2026-11-01: yalnız sınır gizlenince yine 0,0 ms çıktı); döngü gerçekten koşsun.
            for i in 0..black_box(3_000_000i64) {
                toplam += black_box(i);
            }
            println!("dongu {} {}", black_box(toplam), ms(t));
        }
        "fib" => {
            let t = Instant::now();
            let f = fib(black_box(27));
            println!("fib {} {}", f, ms(t));
        }
        "sirala" => {
            let t = Instant::now();
            let mut x: i64 = 42;
            let mut l: Vec<i64> = Vec::new();
            for _ in 0..300_000 {
                x = (x * 1103515245 + 12345) % 2147483648;
                l.push(x);
            }
            l.sort();
            println!("sirala {} {}", l[0] + l[150000] + l[299999], ms(t));
        }
        "metin" => {
            let t = Instant::now();
            let mut toplam: usize = 0;
            for i in 0..200_000 {
                let s = format!("kelime{}", i);
                toplam += s.len();
            }
            println!("metin {} {}", toplam, ms(t));
        }
        "sozluk" => {
            let t = Instant::now();
            let mut d: HashMap<String, i64> = HashMap::new();
            for i in 0..200_000i64 {
                d.insert(format!("k{}", i), i);
            }
            let mut toplam: i64 = 0;
            for i in 0..200_000 {
                toplam += d[&format!("k{}", i)];
            }
            println!("sozluk {} {}", toplam, ms(t));
        }
        "matris_dongu" => {
            let n = 120usize;
            let mut a = vec![vec![0i64; n]; n];
            let mut b = vec![vec![0i64; n]; n];
            for i in 0..n {
                for j in 0..n {
                    a[i][j] = ((i * j) % 7) as i64;
                    b[i][j] = ((i + j) % 5) as i64;
                }
            }
            let t = Instant::now();
            let mut iz: i64 = 0;
            for i in 0..n {
                for j in 0..n {
                    let mut s = 0i64;
                    for k in 0..n {
                        s += a[i][k] * b[k][j];
                    }
                    if i == j {
                        iz += s;
                    }
                }
            }
            println!("matris_dongu {} {}", iz, ms(t));
        }
        "matris_yerlesik" => {
            // Go sürümü gibi elle yazılmış üçlü döngü; rastgele değerler basit bir doğrusal üreteçten.
            let n = 512usize;
            let mut x: u64 = 42;
            let mut sonraki = || {
                x = x.wrapping_mul(6364136223846793005).wrapping_add(1442695040888963407);
                (x >> 11) as f64 / (1u64 << 53) as f64
            };
            let a: Vec<f64> = (0..n * n).map(|_| sonraki()).collect();
            let b: Vec<f64> = (0..n * n).map(|_| sonraki()).collect();
            let mul = |x: &[f64], y: &[f64]| -> Vec<f64> {
                let mut c = vec![0.0f64; n * n];
                for i in 0..n {
                    for k in 0..n {
                        let v = x[i * n + k];
                        for j in 0..n {
                            c[i * n + j] += v * y[k * n + j];
                        }
                    }
                }
                c
            };
            let t = Instant::now();
            let c = mul(&a, &b);
            let d = mul(&c, &b);
            black_box(d);
            println!("matris_yerlesik - {}", ms(t));
        }
        _ => {
            eprintln!("bilinmeyen test: {}", ad);
            std::process::exit(2);
        }
    }
}
