import argparse
from pathlib import Path
from typing import List, Optional


def pick_baseline(files: List[str], current: str) -> Optional[str]:
    """
    Pick the latest baseline file path from a list of candidate json files,
    excluding the current file itself.

    Rule: sort by filename (stable) and pick the last one != current.
    """
    cur = str(current)
    candidates = sorted([str(f) for f in files])
    for p in reversed(candidates):
        if p != cur:
            return p
    return None


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--out-dir", required=True)
    ap.add_argument("--current", required=True)
    args = ap.parse_args()

    out_dir = Path(args.out_dir)
    files = [str(p) for p in out_dir.glob("*.json")]
    baseline = pick_baseline(files, args.current)
    if baseline:
        print(baseline)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

