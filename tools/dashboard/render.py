#!/usr/bin/env python3
"""Render template.html + history.json + milestones.json into a single
publishable HTML file for the Artifact tool.

Usage: python3 tools/dashboard/render.py [output_path]
Defaults to tools/dashboard/dist.html.
"""
import json
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent


def main():
    out_path = Path(sys.argv[1]) if len(sys.argv) > 1 else HERE / "dist.html"

    template = (HERE / "template.html").read_text()
    history = (HERE / "history.json").read_text() if (HERE / "history.json").exists() else "[]"
    milestones = (HERE / "milestones.json").read_text()

    # Validate JSON before embedding so a bad run doesn't publish broken HTML.
    json.loads(history)
    json.loads(milestones)

    html = template.replace("__RUN_HISTORY_JSON__", history).replace(
        "__MILESTONES_JSON__", milestones
    )
    out_path.write_text(html)
    print(f"wrote {out_path}")


if __name__ == "__main__":
    main()
