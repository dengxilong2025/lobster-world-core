import unittest
from pathlib import Path

from scripts.benchmarks_parse import parse_loadtest_output


class TestParse(unittest.TestCase):
    def test_parse_auth(self):
        txt = Path("scripts/benchmarks_samples/loadtest_auth_challenge.txt").read_text(
            encoding="utf-8"
        )
        out = parse_loadtest_output(txt)
        self.assertIn("qps", out)
        self.assertIn("avg_time_sec", out)
        self.assertIn("status_counts", out)

    def test_parse_intents_has_busy(self):
        txt = Path("scripts/benchmarks_samples/loadtest_intents.txt").read_text(
            encoding="utf-8"
        )
        out = parse_loadtest_output(txt)
        self.assertIn("busy_503", out)
        self.assertGreaterEqual(out["busy_503"], 0)

    def test_parse_export_has_avg_bytes(self):
        txt = Path("scripts/benchmarks_samples/loadtest_replay_export.txt").read_text(
            encoding="utf-8"
        )
        out = parse_loadtest_output(txt)
        self.assertIn("avg_bytes", out)


if __name__ == "__main__":
    unittest.main()

