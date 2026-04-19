#!/usr/bin/env python3
"""
Generate assets/production/manifest.json for the /ui/assets preview page.

Constraints:
- No external dependencies.
- Manifest shape must match uiAssetsPageHTML expectations:
    manifest.ui.icons / frames / badges
    manifest.tiles.base / props / props_final_1024
    manifest.scenes
- tiles.props must NOT include files under tiles/base or tiles/props_final_1024
  to avoid duplication.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Iterable

ROOT = Path(__file__).resolve().parents[1]
PROD = ROOT / "assets" / "production"
OUT = PROD / "manifest.json"


def rel_to_prod(p: Path) -> str:
    return p.relative_to(PROD).as_posix()


def _iter_png_files(root: Path, exclude_dirs: Iterable[Path] = ()) -> list[Path]:
    """
    Recursively list .png files under root, pruning excluded directories.
    Returns absolute Paths.
    """
    if not root.exists():
        return []

    exclude_abs = {d.resolve() for d in exclude_dirs}
    out: list[Path] = []

    for dirpath, dirnames, filenames in os.walk(root):
        dp = Path(dirpath).resolve()

        # Prune excluded and hidden directories to avoid needless traversal.
        kept_dirnames = []
        for d in dirnames:
            if d.startswith("."):
                continue
            cand = (dp / d).resolve()
            if cand in exclude_abs:
                continue
            kept_dirnames.append(d)
        dirnames[:] = kept_dirnames

        for fn in filenames:
            if fn.startswith("."):
                continue
            if not fn.lower().endswith(".png"):
                continue
            out.append(Path(dirpath) / fn)

    out.sort(key=lambda p: p.as_posix())
    return out


def list_png_rel(dirpath: Path, exclude_dirs: Iterable[Path] = ()) -> list[str]:
    return [rel_to_prod(p) for p in _iter_png_files(dirpath, exclude_dirs=exclude_dirs)]


def main() -> None:
    tiles_dir = PROD / "tiles"
    tiles_base_dir = tiles_dir / "base"
    tiles_props_final_1024_dir = tiles_dir / "props_final_1024"

    data = {
        "ui": {
            "icons": list_png_rel(PROD / "ui" / "icons"),
            "frames": list_png_rel(PROD / "ui" / "frames"),
            "badges": list_png_rel(PROD / "ui" / "badges"),
        },
        "tiles": {
            "base": list_png_rel(tiles_base_dir),
            # NOTE: props are "everything else under tiles/", excluding the two
            # categories that are displayed separately.
            "props": list_png_rel(
                tiles_dir, exclude_dirs=(tiles_base_dir, tiles_props_final_1024_dir)
            ),
            "props_final_1024": list_png_rel(tiles_props_final_1024_dir),
        },
        "scenes": list_png_rel(PROD / "scenes"),
    }

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(
        json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(f"wrote {OUT} ({OUT.stat().st_size} bytes)")


if __name__ == "__main__":
    main()

