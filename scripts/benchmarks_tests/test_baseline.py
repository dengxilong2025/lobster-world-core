import unittest

from scripts.benchmarks_baseline import pick_baseline


class TestBaseline(unittest.TestCase):
    def test_pick_latest_not_current(self):
        files = [
            "docs/ops/benchmarks/2026-04-23_aaaaaaa.json",
            "docs/ops/benchmarks/2026-04-23_bbbbbbb.json",
            "docs/ops/benchmarks/2026-04-23_ccccccc.json",
        ]
        cur = "docs/ops/benchmarks/2026-04-23_ccccccc.json"
        self.assertEqual(
            pick_baseline(files, cur), "docs/ops/benchmarks/2026-04-23_bbbbbbb.json"
        )


if __name__ == "__main__":
    unittest.main()

