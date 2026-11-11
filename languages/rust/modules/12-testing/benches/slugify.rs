//! `cargo bench -p m12-testing --bench slugify` (criterion; the numbers depend on your machine).

use std::hint::black_box;

use criterion::{Criterion, criterion_group, criterion_main};
use m12_testing::slugify;

fn bench_slugify(c: &mut Criterion) {
    let short = "Write the docs";
    let long = "A much longer task title, with punctuation! And more words... ".repeat(8);

    let mut group = c.benchmark_group("slugify");
    group.bench_function("short", |b| b.iter(|| slugify(black_box(short))));
    group.bench_function("long", |b| b.iter(|| slugify(black_box(&long))));
    group.finish();
}

criterion_group!(benches, bench_slugify);
criterion_main!(benches);
