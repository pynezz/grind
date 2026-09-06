#!/usr/bin/env python3
"""Run the grind test suite and benchmarks, append one run record to
history.json. Pure stdlib; no network code, matching the rest of M0.

Usage: python3 tools/dashboard/collect.py
Run from the repo root (or anywhere; paths are relative to this file).
"""
import json
import re
import subprocess
import sys
import time
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
HISTORY_PATH = Path(__file__).resolve().parent / "history.json"
MAX_RUNS = 50

BENCH_DIFFICULTIES = ["4", "6", "8", "10"]
BENCH_RE = re.compile(r"^(BenchmarkSolve/d=(\d+))-\d+\s+\d+\s+(\d+)\s+ns/op")


def run_tests():
    start = time.monotonic()
    proc = subprocess.run(
        ["go", "test", "./...", "-json"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    duration_ms = round((time.monotonic() - start) * 1000)

    packages = {}
    for line in proc.stdout.splitlines():
        if not line.strip():
            continue
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            continue
        pkg = ev.get("Package", "")
        short = pkg.replace("github.com/pynezz/grind/", "") or "grind"
        test = ev.get("Test")
        action = ev.get("Action")
        entry = packages.setdefault(
            short, {"name": short, "tests": {}, "elapsedS": 0.0}
        )
        if test:
            # Only count top-level tests, not t.Run subtests, in the
            # pass/fail tally (subtests still appear under detail).
            t = entry["tests"].setdefault(test, {"status": None, "elapsedS": 0.0})
            if action in ("pass", "fail", "skip"):
                t["status"] = action
                t["elapsedS"] = ev.get("Elapsed", 0.0)
        elif action == "pass":
            entry["elapsedS"] = ev.get("Elapsed", 0.0)

    pkg_summaries = []
    overall = "pass"
    for name, entry in sorted(packages.items()):
        # Subtests (name contains "/") roll up into their parent's status;
        # only tally top-level tests so counts match `go test` package output.
        top_level = {n: t for n, t in entry["tests"].items() if "/" not in n}
        total = len(top_level)
        passed = sum(1 for t in top_level.values() if t["status"] == "pass")
        failed = sum(1 for t in top_level.values() if t["status"] == "fail")
        if failed > 0:
            overall = "fail"
        pkg_summaries.append(
            {
                "name": name,
                "total": total,
                "passed": passed,
                "failed": failed,
                "elapsedS": entry["elapsedS"],
                "tests": [
                    {"name": n, "status": t["status"], "elapsedS": t["elapsedS"]}
                    for n, t in sorted(top_level.items())
                ],
            }
        )
    if proc.returncode != 0 and overall == "pass":
        # A build failure or panic with no failed test action still means
        # the run did not pass.
        overall = "fail"
    return pkg_summaries, overall, duration_ms


def run_benchmarks():
    proc = subprocess.run(
        [
            "go",
            "test",
            "./puzzle/",
            "-bench",
            "BenchmarkSolve",
            "-benchtime=3x",
            "-run",
            "^$",
        ],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    benchmarks = []
    for line in proc.stdout.splitlines():
        m = BENCH_RE.match(line)
        if m:
            benchmarks.append({"name": f"d={m.group(2)}", "nsPerOp": int(m.group(3))})
    return benchmarks


def main():
    packages, overall, duration_ms = run_tests()
    benchmarks = run_benchmarks()

    run = {
        "ts": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "overall": overall,
        "durationMs": duration_ms,
        "packages": packages,
        "benchmarks": benchmarks,
    }

    history = []
    if HISTORY_PATH.exists():
        history = json.loads(HISTORY_PATH.read_text())
    history.append(run)
    history = history[-MAX_RUNS:]
    HISTORY_PATH.write_text(json.dumps(history, indent=2) + "\n")

    print(f"recorded run: overall={overall} duration={duration_ms}ms "
          f"packages={len(packages)} benchmarks={len(benchmarks)}")
    return 0 if overall == "pass" else 1


if __name__ == "__main__":
    sys.exit(main())
