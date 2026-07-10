import json
import tempfile
import unittest
from pathlib import Path

import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "perf"))

import perf_summary


class PerformanceSummaryTests(unittest.TestCase):
    def test_k6_markdown_renders_expected_metrics(self):
        payload = {
            "metrics": {
                "http_req_duration": {
                    "values": {"avg": 1.25, "med": 1, "p(90)": 2, "p(95)": 3, "max": 4}
                },
                "http_reqs": {"values": {"count": 20, "rate": 10}},
                "http_req_failed": {"values": {"rate": 0.05}},
                "checks": {"values": {"rate": 0.95}},
                "iterations": {"values": {"count": 20, "rate": 10}},
            }
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "summary.json"
            path.write_text(json.dumps(payload), encoding="utf-8")
            markdown = perf_summary.k6_markdown("Load", str(path))

        self.assertIn("| Requests | 20 |", markdown)
        self.assertIn("| Failure rate | 5.00% |", markdown)
        self.assertIn("| P95 latency | 3.00 ms |", markdown)

    def test_go_bench_markdown_parses_standard_benchmark_output(self):
        output = "BenchmarkRouter-8  1000  1234 ns/op  512 B/op  7 allocs/op\n"
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "bench.txt"
            path.write_text(output, encoding="utf-8")
            markdown = perf_summary.go_bench_markdown("Bench", str(path))

        self.assertIn("`BenchmarkRouter-8`", markdown)
        self.assertIn("| 1,000 | 1,234 | 512 | 7 |", markdown)

    def test_append_summary_creates_and_appends_file(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "summary.md"
            perf_summary.append_summary("first", str(path))
            perf_summary.append_summary("second", str(path))
            self.assertEqual(path.read_text(encoding="utf-8"), "first\nsecond\n")


if __name__ == "__main__":
    unittest.main()
