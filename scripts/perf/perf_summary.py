#!/usr/bin/env python3
import argparse
import json
import re
from pathlib import Path


def metric_values(summary: dict, name: str) -> dict:
    metric = summary.get("metrics", {}).get(name, {})
    return metric.get("values", {})


def number(values: dict, key: str, default: float = 0.0) -> float:
    value = values.get(key, default)
    try:
        return float(value)
    except (TypeError, ValueError):
        return default


def integer(values: dict, key: str, default: int = 0) -> int:
    return int(number(values, key, default))


def format_ms(value: float) -> str:
    return f"{value:.2f} ms"


def format_rate(value: float) -> str:
    return f"{value:.2f}/s"


def format_percent(value: float) -> str:
    return f"{value * 100:.2f}%"


def append_summary(markdown: str, summary_file: str) -> None:
    if not summary_file:
        return
    path = Path(summary_file)
    with path.open("a", encoding="utf-8") as file:
        file.write(markdown)
        file.write("\n")


def k6_markdown(title: str, input_path: str) -> str:
    summary = json.loads(Path(input_path).read_text(encoding="utf-8"))
    duration = metric_values(summary, "http_req_duration")
    requests = metric_values(summary, "http_reqs")
    failures = metric_values(summary, "http_req_failed")
    checks = metric_values(summary, "checks")
    iterations = metric_values(summary, "iterations")

    rows = [
        ("Requests", str(integer(requests, "count"))),
        ("Request rate", format_rate(number(requests, "rate"))),
        ("Iterations", str(integer(iterations, "count"))),
        ("Iteration rate", format_rate(number(iterations, "rate"))),
        ("Failure rate", format_percent(number(failures, "rate"))),
        ("Check pass rate", format_percent(number(checks, "rate"))),
        ("Avg latency", format_ms(number(duration, "avg"))),
        ("Median latency", format_ms(number(duration, "med"))),
        ("P90 latency", format_ms(number(duration, "p(90)"))),
        ("P95 latency", format_ms(number(duration, "p(95)"))),
        ("Max latency", format_ms(number(duration, "max"))),
    ]
    table = "\n".join(f"| {name} | {value} |" for name, value in rows)
    return f"""## {title}

| Metric | Value |
|---|---:|
{table}
"""


BENCHMARK_RE = re.compile(
    r"^(Benchmark\S+)\s+(\d+)\s+([\d.]+)\s+ns/op\s+([\d.]+)\s+B/op\s+([\d.]+)\s+allocs/op"
)


def go_bench_markdown(title: str, input_path: str) -> str:
    rows = []
    for line in Path(input_path).read_text(encoding="utf-8").splitlines():
        match = BENCHMARK_RE.match(line.strip())
        if not match:
            continue
        name, iterations, ns_op, bytes_op, allocs_op = match.groups()
        rows.append(
            f"| `{name}` | {int(iterations):,} | {float(ns_op):,.0f} | {float(bytes_op):,.0f} | {float(allocs_op):,.0f} |"
        )

    if not rows:
        table = "| No benchmark rows found | - | - | - | - |"
    else:
        table = "\n".join(rows)

    return f"""## {title}

| Benchmark | Iterations | ns/op | B/op | allocs/op |
|---|---:|---:|---:|---:|
{table}
"""


def main() -> None:
    parser = argparse.ArgumentParser(description="Render performance results as Markdown.")
    subparsers = parser.add_subparsers(dest="command", required=True)

    k6_parser = subparsers.add_parser("k6")
    k6_parser.add_argument("--input", required=True)
    k6_parser.add_argument("--title", required=True)
    k6_parser.add_argument("--summary-file", default="")

    bench_parser = subparsers.add_parser("go-bench")
    bench_parser.add_argument("--input", required=True)
    bench_parser.add_argument("--title", required=True)
    bench_parser.add_argument("--summary-file", default="")

    args = parser.parse_args()
    if args.command == "k6":
        markdown = k6_markdown(args.title, args.input)
    else:
        markdown = go_bench_markdown(args.title, args.input)

    print(markdown)
    append_summary(markdown, args.summary_file)


if __name__ == "__main__":
    main()
