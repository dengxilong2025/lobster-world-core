import json
import unittest
from pathlib import Path

from scripts.benchmarks_diff import diff_summary


class TestDiff(unittest.TestCase):
    def test_regression_threshold(self):
        base = json.loads(
            Path("scripts/benchmarks_samples/bench_baseline.json").read_text(encoding="utf-8")
        )
        cur = json.loads(
            Path("scripts/benchmarks_samples/bench_current.json").read_text(encoding="utf-8")
        )
        md = diff_summary(cur, base, threshold_pct=10)
        self.assertIn("| metric | baseline | current | delta | verdict |", md)
        self.assertIn("| qps |", md)
        self.assertIn("tick_overrun_total_sum:", md)
        self.assertIn("bench_world_pending_queue_len:", md)
        self.assertIn("## agent_batch", md)
        self.assertIn("| duration_sec |", md)
        self.assertIn("| fail_total |", md)
        self.assertIn("REGRESSION", md)


if __name__ == "__main__":
    unittest.main()
