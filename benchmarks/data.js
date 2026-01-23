window.BENCHMARK_DATA = {
  "lastUpdate": 1769135307494,
  "repoUrl": "https://github.com/stuckj/mkvdup",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "stuckj@gmail.com",
            "name": "Jonathan Stucklen",
            "username": "stuckj"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "efa5b29031908184aa6959484dc181a94246c209",
          "message": "Add performance regression testing in CI (#46) (#47)\n\n* Add performance regression testing in CI (#46)\n\n- Add benchmark tests for reader operations (entry access, ReadAt, initialization)\n- Add CI workflow using github-action-benchmark with 15% regression threshold\n- Add local benchmark comparison script (scripts/benchmark-compare.sh)\n- Update CONTRIBUTING.md with benchmark instructions\n\nBenchmarks track:\n- Sequential/random entry access (cache effectiveness)\n- Sequential/random ReadAt (simulates video playback)\n- Small reads (container parsing)\n- Reader initialization overhead\n\nResults visualized at: https://stuckj.github.io/mkvdup/benchmarks/\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Ignore benchmark dashboard URL in link checker (not yet deployed)\n\n* Address Copilot review feedback\n\n- Quote variable in trap command for shell safety\n- Add f.Sync() check to catch write errors in test helper\n- Fix potential panic in BenchmarkReadAt_Small when fileSize <= chunkSize\n- Fix potential panic in BenchmarkFindEntriesForRange when fileSize < 1000\n\n* Add benchstat for statistically significant regression detection\n\n- Use benchstat for statistical comparison (p-values, significance)\n- Add check-benchstat.sh script to detect >10% significant regressions\n- Add initial baseline.txt generated with count=10\n- Keep github-action-benchmark for visualization/trending\n- Baseline auto-updates on main branch merges\n- Update CONTRIBUTING.md with new workflow\n\n* Use count=5 for benchmarks and regenerate clean baseline\n\n- count=10 caused timeouts; count=5 is sufficient for benchstat\n- Regenerated baseline.txt without stack trace corruption\n- Updated workflow, script, and docs to use count=5\n\n* Address Copilot review feedback (round 2)\n\n- Add error handling for benchstat parse failures in CI workflow\n- Update to math/rand/v2 with NewPCG for modern Go patterns\n- PR description already updated to reflect 10% threshold\n- Scripts already have +x permissions (verified)\n\n* Address Copilot review feedback (round 3)\n\n- Fix memory allocation in benchmarks: use on-demand RNG instead of\n  pre-allocating b.N random values to avoid OOM with large iterations\n- Replace bc with awk for floating-point comparison in check-benchstat.sh\n  (awk is more universally available in CI environments)\n- Tighten regex pattern from '~.*\\(p=' to '~\\(p=' for more precise matching\n- Add fetch-depth: 0 to checkout step to avoid issues with concurrent\n  pushes to main when updating baseline\n- Use explicit 'bash scripts/...' instead of relying on execute permissions\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Clarify benchmark baseline is auto-updated by CI\n\n- Remove 'save' command from docs (not needed for normal workflow)\n- Add note that users should not commit baseline changes manually\n- Make local comparison section clearly optional\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n---------\n\nCo-authored-by: Claude Opus 4.5 <noreply@anthropic.com>",
          "timestamp": "2026-01-21T14:09:12-05:00",
          "tree_id": "d9d4e2092b2bbdef83ec4764c86bba2decc60b7f",
          "url": "https://github.com/stuckj/mkvdup/commit/efa5b29031908184aa6959484dc181a94246c209"
        },
        "date": 1769022699814,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.67,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32548860 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.67,
            "unit": "ns/op",
            "extra": "32548860 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32548860 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32548860 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.74,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32355388 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.74,
            "unit": "ns/op",
            "extra": "32355388 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32355388 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32355388 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.78,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32463182 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.78,
            "unit": "ns/op",
            "extra": "32463182 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32463182 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32463182 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.75,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32511434 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.75,
            "unit": "ns/op",
            "extra": "32511434 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32511434 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32511434 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.74,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32443053 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.74,
            "unit": "ns/op",
            "extra": "32443053 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32443053 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32443053 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.31,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29166529 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.31,
            "unit": "ns/op",
            "extra": "29166529 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29166529 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29166529 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29135535 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.3,
            "unit": "ns/op",
            "extra": "29135535 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29135535 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29135535 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29555108 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.24,
            "unit": "ns/op",
            "extra": "29555108 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29555108 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29555108 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.19,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29403074 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.19,
            "unit": "ns/op",
            "extra": "29403074 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29403074 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29403074 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29265776 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.24,
            "unit": "ns/op",
            "extra": "29265776 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29265776 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29265776 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.47,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.47,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.53,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "99155304 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.53,
            "unit": "ns/op",
            "extra": "99155304 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "99155304 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "99155304 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.43,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.43,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.64,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.64,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.59,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.59,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44916,
            "unit": "ns/op\t1459.09 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26547 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44916,
            "unit": "ns/op",
            "extra": "26547 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1459.09,
            "unit": "MB/s",
            "extra": "26547 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26547 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26547 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44497,
            "unit": "ns/op\t1472.82 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26494 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44497,
            "unit": "ns/op",
            "extra": "26494 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1472.82,
            "unit": "MB/s",
            "extra": "26494 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26494 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26494 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44396,
            "unit": "ns/op\t1476.18 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "27159 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44396,
            "unit": "ns/op",
            "extra": "27159 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1476.18,
            "unit": "MB/s",
            "extra": "27159 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "27159 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "27159 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44107,
            "unit": "ns/op\t1485.83 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "27031 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44107,
            "unit": "ns/op",
            "extra": "27031 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1485.83,
            "unit": "MB/s",
            "extra": "27031 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "27031 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "27031 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 43870,
            "unit": "ns/op\t1493.88 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "27222 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 43870,
            "unit": "ns/op",
            "extra": "27222 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1493.88,
            "unit": "MB/s",
            "extra": "27222 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "27222 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "27222 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45876,
            "unit": "ns/op\t1428.55 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26298 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45876,
            "unit": "ns/op",
            "extra": "26298 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1428.55,
            "unit": "MB/s",
            "extra": "26298 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26298 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26298 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46367,
            "unit": "ns/op\t1413.42 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25831 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46367,
            "unit": "ns/op",
            "extra": "25831 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1413.42,
            "unit": "MB/s",
            "extra": "25831 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25831 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25831 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 43836,
            "unit": "ns/op\t1495.04 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "27237 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 43836,
            "unit": "ns/op",
            "extra": "27237 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1495.04,
            "unit": "MB/s",
            "extra": "27237 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "27237 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "27237 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 43782,
            "unit": "ns/op\t1496.87 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25860 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 43782,
            "unit": "ns/op",
            "extra": "25860 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1496.87,
            "unit": "MB/s",
            "extra": "25860 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25860 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25860 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 44078,
            "unit": "ns/op\t1486.81 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "27226 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 44078,
            "unit": "ns/op",
            "extra": "27226 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1486.81,
            "unit": "MB/s",
            "extra": "27226 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "27226 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "27226 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 454.6,
            "unit": "ns/op\t 563.18 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2641070 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 454.6,
            "unit": "ns/op",
            "extra": "2641070 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 563.18,
            "unit": "MB/s",
            "extra": "2641070 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2641070 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2641070 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 452.9,
            "unit": "ns/op\t 565.25 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2639871 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 452.9,
            "unit": "ns/op",
            "extra": "2639871 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 565.25,
            "unit": "MB/s",
            "extra": "2639871 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2639871 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2639871 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 450.9,
            "unit": "ns/op\t 567.77 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2634897 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 450.9,
            "unit": "ns/op",
            "extra": "2634897 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 567.77,
            "unit": "MB/s",
            "extra": "2634897 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2634897 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2634897 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 452.7,
            "unit": "ns/op\t 565.51 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2649907 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 452.7,
            "unit": "ns/op",
            "extra": "2649907 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 565.51,
            "unit": "MB/s",
            "extra": "2649907 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2649907 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2649907 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 450.1,
            "unit": "ns/op\t 568.74 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2642174 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 450.1,
            "unit": "ns/op",
            "extra": "2642174 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 568.74,
            "unit": "MB/s",
            "extra": "2642174 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2642174 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2642174 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 911.1,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1315892 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 911.1,
            "unit": "ns/op",
            "extra": "1315892 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1315892 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1315892 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 913.9,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1312171 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 913.9,
            "unit": "ns/op",
            "extra": "1312171 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1312171 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1312171 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 917.4,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1306372 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 917.4,
            "unit": "ns/op",
            "extra": "1306372 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1306372 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1306372 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 910.4,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1315197 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 910.4,
            "unit": "ns/op",
            "extra": "1315197 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1315197 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1315197 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 923.2,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1313515 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 923.2,
            "unit": "ns/op",
            "extra": "1313515 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1313515 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1313515 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30028,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39625 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30028,
            "unit": "ns/op",
            "extra": "39625 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39625 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39625 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30702,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39136 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30702,
            "unit": "ns/op",
            "extra": "39136 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39136 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39136 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30218,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "36702 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30218,
            "unit": "ns/op",
            "extra": "36702 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "36702 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "36702 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30583,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39884 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30583,
            "unit": "ns/op",
            "extra": "39884 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39884 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39884 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30486,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38929 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30486,
            "unit": "ns/op",
            "extra": "38929 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38929 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38929 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30140,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39136 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30140,
            "unit": "ns/op",
            "extra": "39136 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39136 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39136 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30346,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39206 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30346,
            "unit": "ns/op",
            "extra": "39206 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39206 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39206 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30615,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38721 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30615,
            "unit": "ns/op",
            "extra": "38721 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38721 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38721 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30394,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39715 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30394,
            "unit": "ns/op",
            "extra": "39715 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39715 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39715 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30516,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39154 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30516,
            "unit": "ns/op",
            "extra": "39154 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39154 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39154 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "stuckj@gmail.com",
            "name": "Jonathan Stucklen",
            "username": "stuckj"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "ec4f23ce5877738f353ff6ab78465a851f7fba4b",
          "message": "Use PAT for benchmark baseline and gh-pages updates (#50)\n\n* Use PAT for benchmark baseline and gh-pages updates\n\nThe GITHUB_TOKEN cannot bypass branch protection rules. Use the\nBENCHMARK_PAT secret to authenticate pushes to main (baseline) and\ngh-pages (visualization).\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Remove benchmark URL from markdown link check ignore list\n\nThe gh-pages benchmark dashboard is now live, so we no longer need\nto ignore this URL in link checking.\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Add performance benchmarks link to README\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Address Copilot review feedback on benchmark workflow\n\n- Use BENCHMARK_PAT in checkout step for consistent credentials\n- Add validation to skip baseline update if PAT not configured\n- Add git pull --rebase before push to handle concurrent commits\n- Remove URL token interpolation (checkout handles credentials)\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n---------\n\nCo-authored-by: Claude Opus 4.5 <noreply@anthropic.com>",
          "timestamp": "2026-01-21T15:01:40-05:00",
          "tree_id": "d65a074eae05ec0f1833f09fe2398499e663c2da",
          "url": "https://github.com/stuckj/mkvdup/commit/ec4f23ce5877738f353ff6ab78465a851f7fba4b"
        },
        "date": 1769025856579,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32600860 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.7,
            "unit": "ns/op",
            "extra": "32600860 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32600860 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32600860 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32596946 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.8,
            "unit": "ns/op",
            "extra": "32596946 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32596946 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32596946 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.78,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32420758 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.78,
            "unit": "ns/op",
            "extra": "32420758 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32420758 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32420758 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.67,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32170509 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.67,
            "unit": "ns/op",
            "extra": "32170509 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32170509 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32170509 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.68,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32560869 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.68,
            "unit": "ns/op",
            "extra": "32560869 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32560869 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32560869 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.23,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28966464 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.23,
            "unit": "ns/op",
            "extra": "28966464 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28966464 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28966464 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.29,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28750664 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.29,
            "unit": "ns/op",
            "extra": "28750664 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28750664 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28750664 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28904172 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.24,
            "unit": "ns/op",
            "extra": "28904172 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28904172 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28904172 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.29,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28641837 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.29,
            "unit": "ns/op",
            "extra": "28641837 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28641837 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28641837 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.27,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28334942 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.27,
            "unit": "ns/op",
            "extra": "28334942 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28334942 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28334942 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.67,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.67,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.49,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.49,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.37,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "98398232 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.37,
            "unit": "ns/op",
            "extra": "98398232 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "98398232 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "98398232 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.69,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.69,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.5,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46401,
            "unit": "ns/op\t1412.38 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25254 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46401,
            "unit": "ns/op",
            "extra": "25254 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1412.38,
            "unit": "MB/s",
            "extra": "25254 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25254 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25254 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46531,
            "unit": "ns/op\t1408.44 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46531,
            "unit": "ns/op",
            "extra": "25808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1408.44,
            "unit": "MB/s",
            "extra": "25808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 45885,
            "unit": "ns/op\t1428.26 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25912 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 45885,
            "unit": "ns/op",
            "extra": "25912 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1428.26,
            "unit": "MB/s",
            "extra": "25912 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25912 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25912 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44547,
            "unit": "ns/op\t1471.16 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26943 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44547,
            "unit": "ns/op",
            "extra": "26943 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1471.16,
            "unit": "MB/s",
            "extra": "26943 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26943 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26943 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46222,
            "unit": "ns/op\t1417.85 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46222,
            "unit": "ns/op",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1417.85,
            "unit": "MB/s",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46146,
            "unit": "ns/op\t1420.19 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26217 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46146,
            "unit": "ns/op",
            "extra": "26217 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1420.19,
            "unit": "MB/s",
            "extra": "26217 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26217 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26217 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46100,
            "unit": "ns/op\t1421.61 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26134 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46100,
            "unit": "ns/op",
            "extra": "26134 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1421.61,
            "unit": "MB/s",
            "extra": "26134 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26134 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26134 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46004,
            "unit": "ns/op\t1424.57 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25819 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46004,
            "unit": "ns/op",
            "extra": "25819 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1424.57,
            "unit": "MB/s",
            "extra": "25819 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25819 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25819 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45828,
            "unit": "ns/op\t1430.03 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25764 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45828,
            "unit": "ns/op",
            "extra": "25764 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1430.03,
            "unit": "MB/s",
            "extra": "25764 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25764 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25764 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45837,
            "unit": "ns/op\t1429.77 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45837,
            "unit": "ns/op",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1429.77,
            "unit": "MB/s",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.7,
            "unit": "ns/op\t 564.28 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2655984 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.7,
            "unit": "ns/op",
            "extra": "2655984 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 564.28,
            "unit": "MB/s",
            "extra": "2655984 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2655984 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2655984 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 452.2,
            "unit": "ns/op\t 566.15 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2642593 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 452.2,
            "unit": "ns/op",
            "extra": "2642593 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 566.15,
            "unit": "MB/s",
            "extra": "2642593 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2642593 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2642593 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 451.9,
            "unit": "ns/op\t 566.46 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2659014 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 451.9,
            "unit": "ns/op",
            "extra": "2659014 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 566.46,
            "unit": "MB/s",
            "extra": "2659014 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2659014 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2659014 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 451.2,
            "unit": "ns/op\t 567.41 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2633002 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 451.2,
            "unit": "ns/op",
            "extra": "2633002 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 567.41,
            "unit": "MB/s",
            "extra": "2633002 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2633002 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2633002 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 452.6,
            "unit": "ns/op\t 565.59 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2645205 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 452.6,
            "unit": "ns/op",
            "extra": "2645205 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 565.59,
            "unit": "MB/s",
            "extra": "2645205 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2645205 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2645205 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 921.3,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1305778 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 921.3,
            "unit": "ns/op",
            "extra": "1305778 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1305778 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1305778 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 919.1,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1305642 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 919.1,
            "unit": "ns/op",
            "extra": "1305642 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1305642 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1305642 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 915.5,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1312971 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 915.5,
            "unit": "ns/op",
            "extra": "1312971 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1312971 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1312971 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 919.9,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1317124 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 919.9,
            "unit": "ns/op",
            "extra": "1317124 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1317124 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1317124 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 914.6,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1297552 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 914.6,
            "unit": "ns/op",
            "extra": "1297552 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1297552 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1297552 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30745,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38038 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30745,
            "unit": "ns/op",
            "extra": "38038 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38038 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38038 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30813,
            "unit": "ns/op\t    1032 B/op\t      27 allocs/op",
            "extra": "38527 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30813,
            "unit": "ns/op",
            "extra": "38527 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1032,
            "unit": "B/op",
            "extra": "38527 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38527 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30967,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38581 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30967,
            "unit": "ns/op",
            "extra": "38581 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38581 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38581 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 31081,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38521 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 31081,
            "unit": "ns/op",
            "extra": "38521 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38521 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38521 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30853,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38653 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30853,
            "unit": "ns/op",
            "extra": "38653 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38653 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38653 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30992,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38576 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30992,
            "unit": "ns/op",
            "extra": "38576 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38576 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38576 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30889,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38506 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30889,
            "unit": "ns/op",
            "extra": "38506 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38506 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38506 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30949,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38670 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30949,
            "unit": "ns/op",
            "extra": "38670 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38670 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38670 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 31018,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38288 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 31018,
            "unit": "ns/op",
            "extra": "38288 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38288 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38288 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30857,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38612 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30857,
            "unit": "ns/op",
            "extra": "38612 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38612 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38612 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "stuckj@gmail.com",
            "name": "Jonathan Stucklen",
            "username": "stuckj"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "cbf6254de2721503c3983677f2d0915842317c03",
          "message": "Add test coverage infrastructure and quick-win tests (Phase 1) (#51)\n\n* Add test coverage infrastructure and quick-win tests (Phase 1)\n\nThis is the first phase of comprehensive test coverage improvements (#45).\n\n## Coverage Infrastructure\n- Add coverage reporting to CI workflow with artifacts\n- Create coverage.yml workflow for gh-pages visualization\n- Add CI and coverage badges to README\n\n## New Tests\n- internal/dedup/config_test.go: 100% coverage for config parsing\n- internal/mmap/mmap_test.go: 93.5% coverage for memory-mapped files\n- Expand daemon_test.go with pipe-based NotifyReady/NotifyError tests\n\n## Coverage Improvements\n- dedup: 62.9% → 70.2%\n- mmap: 0% → 93.5%\n- daemon: 17.4% → 30.4%\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Add test coverage documentation to CONTRIBUTING.md\n\nDocument how to run tests with coverage locally and where to\nfind the coverage reports on GitHub Pages.\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Add coverage URL to markdown link check ignore list\n\nThe coverage page doesn't exist until first merge to main.\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Fix coverage workflow PR comment permissions\n\n- Add pull-requests: write permission\n- Use GITHUB_TOKEN for PR comments (BENCHMARK_PAT is for gh-pages)\n- Add continue-on-error to make comment step non-fatal\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Address Copilot review feedback on coverage workflow\n\n- Move step ID to before run block for clarity in coverage.yml\n- Use errors.Is(err, io.EOF) instead of string comparison in daemon_test.go\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n---------\n\nCo-authored-by: Claude Opus 4.5 <noreply@anthropic.com>",
          "timestamp": "2026-01-21T16:00:51-05:00",
          "tree_id": "8734edd63f981c06594278df259eb342f0f3f61d",
          "url": "https://github.com/stuckj/mkvdup/commit/cbf6254de2721503c3983677f2d0915842317c03"
        },
        "date": 1769029410875,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.73,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32486262 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.73,
            "unit": "ns/op",
            "extra": "32486262 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32486262 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32486262 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.67,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32293896 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.67,
            "unit": "ns/op",
            "extra": "32293896 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32293896 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32293896 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.65,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32376016 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.65,
            "unit": "ns/op",
            "extra": "32376016 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32376016 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32376016 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.83,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32500147 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.83,
            "unit": "ns/op",
            "extra": "32500147 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32500147 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32500147 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32311516 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.72,
            "unit": "ns/op",
            "extra": "32311516 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32311516 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32311516 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.19,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28431585 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.19,
            "unit": "ns/op",
            "extra": "28431585 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28431585 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28431585 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.71,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28343965 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.71,
            "unit": "ns/op",
            "extra": "28343965 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28343965 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28343965 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 48.23,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28344099 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 48.23,
            "unit": "ns/op",
            "extra": "28344099 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28344099 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28344099 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.37,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27930504 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.37,
            "unit": "ns/op",
            "extra": "27930504 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "27930504 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "27930504 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.32,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28347554 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.32,
            "unit": "ns/op",
            "extra": "28347554 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28347554 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28347554 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.45,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "98688032 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.45,
            "unit": "ns/op",
            "extra": "98688032 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "98688032 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "98688032 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.62,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.62,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.61,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "94991493 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.61,
            "unit": "ns/op",
            "extra": "94991493 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "94991493 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "94991493 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.58,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "114624552 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.58,
            "unit": "ns/op",
            "extra": "114624552 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "114624552 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "114624552 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.57,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "113541146 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.57,
            "unit": "ns/op",
            "extra": "113541146 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "113541146 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "113541146 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 45335,
            "unit": "ns/op\t1445.60 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26418 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 45335,
            "unit": "ns/op",
            "extra": "26418 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1445.6,
            "unit": "MB/s",
            "extra": "26418 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26418 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26418 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 45314,
            "unit": "ns/op\t1446.25 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 45314,
            "unit": "ns/op",
            "extra": "26264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1446.25,
            "unit": "MB/s",
            "extra": "26264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 47633,
            "unit": "ns/op\t1375.85 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26188 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 47633,
            "unit": "ns/op",
            "extra": "26188 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1375.85,
            "unit": "MB/s",
            "extra": "26188 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26188 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26188 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 47200,
            "unit": "ns/op\t1388.47 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25244 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 47200,
            "unit": "ns/op",
            "extra": "25244 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1388.47,
            "unit": "MB/s",
            "extra": "25244 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25244 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25244 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 47007,
            "unit": "ns/op\t1394.18 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25574 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 47007,
            "unit": "ns/op",
            "extra": "25574 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1394.18,
            "unit": "MB/s",
            "extra": "25574 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25574 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25574 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46851,
            "unit": "ns/op\t1398.81 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25375 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46851,
            "unit": "ns/op",
            "extra": "25375 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1398.81,
            "unit": "MB/s",
            "extra": "25375 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25375 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25375 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 47279,
            "unit": "ns/op\t1386.15 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25588 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 47279,
            "unit": "ns/op",
            "extra": "25588 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1386.15,
            "unit": "MB/s",
            "extra": "25588 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25588 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25588 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 44670,
            "unit": "ns/op\t1467.12 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26964 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 44670,
            "unit": "ns/op",
            "extra": "26964 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1467.12,
            "unit": "MB/s",
            "extra": "26964 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26964 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26964 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 44826,
            "unit": "ns/op\t1462.01 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26638 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 44826,
            "unit": "ns/op",
            "extra": "26638 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1462.01,
            "unit": "MB/s",
            "extra": "26638 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26638 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26638 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 44732,
            "unit": "ns/op\t1465.07 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26748 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 44732,
            "unit": "ns/op",
            "extra": "26748 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1465.07,
            "unit": "MB/s",
            "extra": "26748 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26748 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26748 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 457.7,
            "unit": "ns/op\t 559.30 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2616016 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 457.7,
            "unit": "ns/op",
            "extra": "2616016 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 559.3,
            "unit": "MB/s",
            "extra": "2616016 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2616016 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2616016 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 458.1,
            "unit": "ns/op\t 558.89 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2607344 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 458.1,
            "unit": "ns/op",
            "extra": "2607344 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 558.89,
            "unit": "MB/s",
            "extra": "2607344 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2607344 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2607344 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 463,
            "unit": "ns/op\t 552.88 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2595354 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 463,
            "unit": "ns/op",
            "extra": "2595354 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 552.88,
            "unit": "MB/s",
            "extra": "2595354 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2595354 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2595354 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 458.3,
            "unit": "ns/op\t 558.55 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2609893 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 458.3,
            "unit": "ns/op",
            "extra": "2609893 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 558.55,
            "unit": "MB/s",
            "extra": "2609893 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2609893 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2609893 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 456.5,
            "unit": "ns/op\t 560.82 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2620344 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 456.5,
            "unit": "ns/op",
            "extra": "2620344 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 560.82,
            "unit": "MB/s",
            "extra": "2620344 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2620344 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2620344 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 942.7,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1285310 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 942.7,
            "unit": "ns/op",
            "extra": "1285310 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1285310 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1285310 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 955.8,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1281987 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 955.8,
            "unit": "ns/op",
            "extra": "1281987 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1281987 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1281987 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 950.8,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1272657 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 950.8,
            "unit": "ns/op",
            "extra": "1272657 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1272657 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1272657 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 930.7,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1280036 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 930.7,
            "unit": "ns/op",
            "extra": "1280036 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1280036 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1280036 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 932.2,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1279798 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 932.2,
            "unit": "ns/op",
            "extra": "1279798 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1279798 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1279798 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30904,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38554 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30904,
            "unit": "ns/op",
            "extra": "38554 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38554 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38554 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 31026,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38190 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 31026,
            "unit": "ns/op",
            "extra": "38190 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38190 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38190 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 31134,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38252 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 31134,
            "unit": "ns/op",
            "extra": "38252 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38252 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38252 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 31232,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38467 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 31232,
            "unit": "ns/op",
            "extra": "38467 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38467 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38467 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 31081,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38138 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 31081,
            "unit": "ns/op",
            "extra": "38138 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38138 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38138 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30956,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38433 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30956,
            "unit": "ns/op",
            "extra": "38433 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38433 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38433 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 31024,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38360 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 31024,
            "unit": "ns/op",
            "extra": "38360 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38360 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38360 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 31022,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38470 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 31022,
            "unit": "ns/op",
            "extra": "38470 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38470 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38470 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30978,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38118 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30978,
            "unit": "ns/op",
            "extra": "38118 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38118 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38118 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30789,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38432 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30789,
            "unit": "ns/op",
            "extra": "38432 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38432 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38432 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "stuckj@gmail.com",
            "name": "Jonathan Stucklen",
            "username": "stuckj"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f0704adb9d144c16083ea650c1cbd23f7dddce8a",
          "message": "Add core package test coverage (Phase 2) (#52)\n\n* Add core package test coverage (Phase 2)\n\nExpand test coverage for core packages:\n- dedup: 70.2% -> 79.6% (reader error paths, verification, edge cases)\n- mkv: 28.9% -> 45.5% (parser tests, VINT edge cases, error handling)\n- source: 22.7% -> 25.9% (indexer tests, error conditions)\n- matcher: 6.2% -> 9.5% (NewMatcher, SetNumWorkers, struct tests)\n\nPart of #45 - Comprehensive test coverage\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Address Copilot review feedback on reader tests\n\n- Use strings.Contains instead of undefined contains helper\n- Compute xxhash of empty data dynamically instead of hardcoding\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Address Copilot review feedback on test code quality\n\n- Refactor TestComputeHash_Empty to test consistency rather than assuming\n  xxhash won't produce zero for empty data\n- Split TestParser_EmptyFile into two tests: one for NewParser behavior\n  and one (TestParser_EmptyContent) for Parse() on minimal content\n- Consolidate test helper functions by introducing testDedupFileOptions\n  struct and createTestDedupFileWithOptions, eliminating duplication\n  between createTestDedupFile and createTestDedupFileZeroEntries\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n---------\n\nCo-authored-by: Claude Opus 4.5 <noreply@anthropic.com>",
          "timestamp": "2026-01-21T22:23:28-05:00",
          "tree_id": "1c653fa671c5abb7f6436f201ac99c0922621185",
          "url": "https://github.com/stuckj/mkvdup/commit/f0704adb9d144c16083ea650c1cbd23f7dddce8a"
        },
        "date": 1769052340276,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.93,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32328471 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.93,
            "unit": "ns/op",
            "extra": "32328471 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32328471 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32328471 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32466146 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.7,
            "unit": "ns/op",
            "extra": "32466146 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32466146 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32466146 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.68,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32290442 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.68,
            "unit": "ns/op",
            "extra": "32290442 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32290442 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32290442 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 37.17,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32625547 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 37.17,
            "unit": "ns/op",
            "extra": "32625547 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32625547 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32625547 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.82,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32583638 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.82,
            "unit": "ns/op",
            "extra": "32583638 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32583638 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32583638 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29754043 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.24,
            "unit": "ns/op",
            "extra": "29754043 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29754043 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29754043 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.08,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29644090 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.08,
            "unit": "ns/op",
            "extra": "29644090 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29644090 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29644090 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.11,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29126078 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.11,
            "unit": "ns/op",
            "extra": "29126078 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29126078 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29126078 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.15,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29690318 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.15,
            "unit": "ns/op",
            "extra": "29690318 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29690318 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29690318 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.05,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29630964 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.05,
            "unit": "ns/op",
            "extra": "29630964 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29630964 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29630964 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.57,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.57,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.48,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.44,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.44,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.5,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.3,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 48539,
            "unit": "ns/op\t1350.18 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "24642 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 48539,
            "unit": "ns/op",
            "extra": "24642 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1350.18,
            "unit": "MB/s",
            "extra": "24642 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "24642 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "24642 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46574,
            "unit": "ns/op\t1407.12 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25466 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46574,
            "unit": "ns/op",
            "extra": "25466 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1407.12,
            "unit": "MB/s",
            "extra": "25466 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25466 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25466 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 47083,
            "unit": "ns/op\t1391.93 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25557 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 47083,
            "unit": "ns/op",
            "extra": "25557 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1391.93,
            "unit": "MB/s",
            "extra": "25557 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25557 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25557 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46166,
            "unit": "ns/op\t1419.57 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25711 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46166,
            "unit": "ns/op",
            "extra": "25711 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1419.57,
            "unit": "MB/s",
            "extra": "25711 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25711 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25711 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46925,
            "unit": "ns/op\t1396.62 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25914 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46925,
            "unit": "ns/op",
            "extra": "25914 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1396.62,
            "unit": "MB/s",
            "extra": "25914 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25914 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25914 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46035,
            "unit": "ns/op\t1423.62 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46035,
            "unit": "ns/op",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1423.62,
            "unit": "MB/s",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45863,
            "unit": "ns/op\t1428.95 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26097 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45863,
            "unit": "ns/op",
            "extra": "26097 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1428.95,
            "unit": "MB/s",
            "extra": "26097 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26097 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26097 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 48343,
            "unit": "ns/op\t1355.64 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "24830 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 48343,
            "unit": "ns/op",
            "extra": "24830 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1355.64,
            "unit": "MB/s",
            "extra": "24830 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "24830 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "24830 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 47143,
            "unit": "ns/op\t1390.14 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25924 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 47143,
            "unit": "ns/op",
            "extra": "25924 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1390.14,
            "unit": "MB/s",
            "extra": "25924 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25924 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25924 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 48102,
            "unit": "ns/op\t1362.45 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "24811 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 48102,
            "unit": "ns/op",
            "extra": "24811 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1362.45,
            "unit": "MB/s",
            "extra": "24811 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "24811 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "24811 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 469.8,
            "unit": "ns/op\t 544.92 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2551262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 469.8,
            "unit": "ns/op",
            "extra": "2551262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 544.92,
            "unit": "MB/s",
            "extra": "2551262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2551262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2551262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 471.3,
            "unit": "ns/op\t 543.15 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2490339 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 471.3,
            "unit": "ns/op",
            "extra": "2490339 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 543.15,
            "unit": "MB/s",
            "extra": "2490339 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2490339 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2490339 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 474.6,
            "unit": "ns/op\t 539.41 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2557520 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 474.6,
            "unit": "ns/op",
            "extra": "2557520 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 539.41,
            "unit": "MB/s",
            "extra": "2557520 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2557520 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2557520 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 470,
            "unit": "ns/op\t 544.67 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2552852 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 470,
            "unit": "ns/op",
            "extra": "2552852 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 544.67,
            "unit": "MB/s",
            "extra": "2552852 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2552852 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2552852 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 482.8,
            "unit": "ns/op\t 530.29 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2543068 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 482.8,
            "unit": "ns/op",
            "extra": "2543068 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 530.29,
            "unit": "MB/s",
            "extra": "2543068 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2543068 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2543068 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 937,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1291718 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 937,
            "unit": "ns/op",
            "extra": "1291718 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1291718 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1291718 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 933.8,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1290046 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 933.8,
            "unit": "ns/op",
            "extra": "1290046 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1290046 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1290046 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 933.6,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1288012 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 933.6,
            "unit": "ns/op",
            "extra": "1288012 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1288012 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1288012 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 937.4,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1285088 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 937.4,
            "unit": "ns/op",
            "extra": "1285088 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1285088 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1285088 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 936.9,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1286842 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 936.9,
            "unit": "ns/op",
            "extra": "1286842 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1286842 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1286842 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30790,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39007 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30790,
            "unit": "ns/op",
            "extra": "39007 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39007 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39007 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30805,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38803 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30805,
            "unit": "ns/op",
            "extra": "38803 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38803 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38803 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30700,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38953 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30700,
            "unit": "ns/op",
            "extra": "38953 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38953 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38953 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30694,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38739 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30694,
            "unit": "ns/op",
            "extra": "38739 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38739 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38739 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30671,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38856 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30671,
            "unit": "ns/op",
            "extra": "38856 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38856 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38856 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30554,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38977 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30554,
            "unit": "ns/op",
            "extra": "38977 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38977 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38977 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30659,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38277 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30659,
            "unit": "ns/op",
            "extra": "38277 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38277 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38277 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30786,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38658 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30786,
            "unit": "ns/op",
            "extra": "38658 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38658 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38658 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30530,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39015 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30530,
            "unit": "ns/op",
            "extra": "39015 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39015 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39015 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30731,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38758 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30731,
            "unit": "ns/op",
            "extra": "38758 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38758 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38758 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "stuckj@gmail.com",
            "name": "Jonathan Stucklen",
            "username": "stuckj"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f34777bb15d906741bc856484adfa587ca3f8a91",
          "message": "Add CLI tests, integration tests, and gotestsum CI (Phase 3) (#53)\n\n* Add CLI tests, expand integration tests, and enhance CI with gotestsum (Phase 3)\n\nCLI Tests:\n- Add cmd/mkvdup/commands_test.go with comprehensive tests for:\n  - samplePackets: stratified sampling with distribution validation\n  - calculateFileChecksum: file hashing with edge cases\n  - ProbeResult struct validation\n\nIntegration Tests:\n- Add TestConcurrentReaders: validates thread-safety with 4 concurrent readers\n- Add TestVerifyIntegrity_Integration: tests integrity verification with real data\n- Add TestReaderInfo_Integration: tests Info method with real dedup files\n\nCI Enhancements:\n- Install and use gotestsum for better test output formatting\n- Generate JUnit XML test results for CI integration\n- Update coverage workflow to include test statistics\n- Publish test results to gh-pages alongside coverage data\n\nCoverage improvements:\n- cmd/mkvdup: 0% → 5.3% (new package tests)\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Address Copilot review feedback on concurrent test and CI\n\n- Use sync.WaitGroup instead of channel counting to prevent test hangs\n  when goroutines encounter errors\n- Replace misleading io.ErrShortWrite/io.ErrUnexpectedEOF sentinels with\n  descriptive fmt.Errorf messages including reader index, offset, and context\n- Use jq to properly parse gotestsum JSON output, counting only test-level\n  events (where Test field is non-empty) for accurate test statistics\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Address additional Copilot review feedback\n\n- Pin gotestsum to v1.12.0 for reproducible CI builds\n- Clarify TestConcurrentReaders comment: tests independent readers, not\n  shared Reader thread-safety\n- Add defer writer.Close() after NewWriter in all 3 integration tests\n  to prevent file descriptor leaks on early test failures\n- Use t.Cleanup() for per-reader cleanup in loop to prevent leaks if\n  reader creation fails mid-loop\n- Use t.TempDir() instead of hardcoded /nonexistent path for portable\n  file-not-found test\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n---------\n\nCo-authored-by: Claude Opus 4.5 <noreply@anthropic.com>",
          "timestamp": "2026-01-22T07:49:02-05:00",
          "tree_id": "9adaddd6952edcbbbee341c01cce1375768ad10a",
          "url": "https://github.com/stuckj/mkvdup/commit/f34777bb15d906741bc856484adfa587ca3f8a91"
        },
        "date": 1769086280645,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.74,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32576071 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.74,
            "unit": "ns/op",
            "extra": "32576071 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32576071 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32576071 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32429138 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.72,
            "unit": "ns/op",
            "extra": "32429138 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32429138 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32429138 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.65,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32418054 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.65,
            "unit": "ns/op",
            "extra": "32418054 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32418054 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32418054 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.67,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32109282 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.67,
            "unit": "ns/op",
            "extra": "32109282 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32109282 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32109282 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.73,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32573109 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.73,
            "unit": "ns/op",
            "extra": "32573109 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32573109 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32573109 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28701074 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.24,
            "unit": "ns/op",
            "extra": "28701074 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28701074 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28701074 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.09,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28298126 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.09,
            "unit": "ns/op",
            "extra": "28298126 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28298126 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28298126 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.08,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28380826 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.08,
            "unit": "ns/op",
            "extra": "28380826 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28380826 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28380826 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.11,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28548177 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.11,
            "unit": "ns/op",
            "extra": "28548177 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28548177 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28548177 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.27,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28597737 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.27,
            "unit": "ns/op",
            "extra": "28597737 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28597737 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28597737 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.76,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "114411607 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.76,
            "unit": "ns/op",
            "extra": "114411607 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "114411607 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "114411607 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.46,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "96490092 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.46,
            "unit": "ns/op",
            "extra": "96490092 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "96490092 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "96490092 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.45,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "97712858 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.45,
            "unit": "ns/op",
            "extra": "97712858 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "97712858 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "97712858 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.43,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "96768308 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.43,
            "unit": "ns/op",
            "extra": "96768308 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "96768308 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "96768308 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.58,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "99469495 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.58,
            "unit": "ns/op",
            "extra": "99469495 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "99469495 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "99469495 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46448,
            "unit": "ns/op\t1410.95 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25710 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46448,
            "unit": "ns/op",
            "extra": "25710 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1410.95,
            "unit": "MB/s",
            "extra": "25710 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25710 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25710 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46411,
            "unit": "ns/op\t1412.08 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25903 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46411,
            "unit": "ns/op",
            "extra": "25903 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1412.08,
            "unit": "MB/s",
            "extra": "25903 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25903 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25903 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46222,
            "unit": "ns/op\t1417.85 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46222,
            "unit": "ns/op",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1417.85,
            "unit": "MB/s",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 45871,
            "unit": "ns/op\t1428.70 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26095 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 45871,
            "unit": "ns/op",
            "extra": "26095 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1428.7,
            "unit": "MB/s",
            "extra": "26095 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26095 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26095 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46145,
            "unit": "ns/op\t1420.23 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26001 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46145,
            "unit": "ns/op",
            "extra": "26001 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1420.23,
            "unit": "MB/s",
            "extra": "26001 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26001 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26001 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46402,
            "unit": "ns/op\t1412.36 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26164 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46402,
            "unit": "ns/op",
            "extra": "26164 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1412.36,
            "unit": "MB/s",
            "extra": "26164 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26164 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26164 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46132,
            "unit": "ns/op\t1420.62 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46132,
            "unit": "ns/op",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1420.62,
            "unit": "MB/s",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26082 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45945,
            "unit": "ns/op\t1426.41 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26270 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45945,
            "unit": "ns/op",
            "extra": "26270 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1426.41,
            "unit": "MB/s",
            "extra": "26270 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26270 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26270 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46135,
            "unit": "ns/op\t1420.52 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25921 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46135,
            "unit": "ns/op",
            "extra": "25921 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1420.52,
            "unit": "MB/s",
            "extra": "25921 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25921 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25921 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45979,
            "unit": "ns/op\t1425.35 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26113 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45979,
            "unit": "ns/op",
            "extra": "26113 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1425.35,
            "unit": "MB/s",
            "extra": "26113 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26113 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26113 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 455.1,
            "unit": "ns/op\t 562.45 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2635197 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 455.1,
            "unit": "ns/op",
            "extra": "2635197 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 562.45,
            "unit": "MB/s",
            "extra": "2635197 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2635197 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2635197 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.2,
            "unit": "ns/op\t 564.92 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2628273 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.2,
            "unit": "ns/op",
            "extra": "2628273 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 564.92,
            "unit": "MB/s",
            "extra": "2628273 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2628273 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2628273 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 455.2,
            "unit": "ns/op\t 562.42 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2642540 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 455.2,
            "unit": "ns/op",
            "extra": "2642540 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 562.42,
            "unit": "MB/s",
            "extra": "2642540 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2642540 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2642540 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 458.2,
            "unit": "ns/op\t 558.70 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2635066 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 458.2,
            "unit": "ns/op",
            "extra": "2635066 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 558.7,
            "unit": "MB/s",
            "extra": "2635066 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2635066 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2635066 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.8,
            "unit": "ns/op\t 564.17 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2631704 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.8,
            "unit": "ns/op",
            "extra": "2631704 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 564.17,
            "unit": "MB/s",
            "extra": "2631704 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2631704 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2631704 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 912,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1304223 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 912,
            "unit": "ns/op",
            "extra": "1304223 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1304223 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1304223 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 916.6,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1302006 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 916.6,
            "unit": "ns/op",
            "extra": "1302006 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1302006 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1302006 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 916.7,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1305991 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 916.7,
            "unit": "ns/op",
            "extra": "1305991 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1305991 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1305991 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 919.9,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1269220 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 919.9,
            "unit": "ns/op",
            "extra": "1269220 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1269220 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1269220 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 920.2,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1308543 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 920.2,
            "unit": "ns/op",
            "extra": "1308543 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1308543 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1308543 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30187,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39658 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30187,
            "unit": "ns/op",
            "extra": "39658 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39658 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39658 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 29978,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39484 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 29978,
            "unit": "ns/op",
            "extra": "39484 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39484 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39484 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 29922,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39534 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 29922,
            "unit": "ns/op",
            "extra": "39534 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39534 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39534 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 29929,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39674 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 29929,
            "unit": "ns/op",
            "extra": "39674 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39674 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39674 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30062,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39826 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30062,
            "unit": "ns/op",
            "extra": "39826 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39826 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39826 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 29783,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39974 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 29783,
            "unit": "ns/op",
            "extra": "39974 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39974 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39974 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 29924,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39906 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 29924,
            "unit": "ns/op",
            "extra": "39906 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39906 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39906 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 29876,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39607 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 29876,
            "unit": "ns/op",
            "extra": "39607 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39607 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39607 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30000,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39718 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30000,
            "unit": "ns/op",
            "extra": "39718 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39718 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39718 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 29806,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "40020 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 29806,
            "unit": "ns/op",
            "extra": "40020 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "40020 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "40020 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "stuckj@gmail.com",
            "name": "Jonathan Stucklen",
            "username": "stuckj"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "4baa187de8bd08cd8c83cc90794ce607c55e0764",
          "message": "Add FUSE package testability and test coverage (Phase 4) (#54)\n\n* Add FUSE package testability and test coverage (Phase 4)\n\nRefactor the fuse package for dependency injection to enable unit testing:\n\n- Add interfaces.go with DedupReader, ReaderInitializer, ReaderFactory,\n  and ConfigReader interfaces for mocking\n- Add adapters.go with default implementations wrapping dedup package\n- Refactor fs.go to use interfaces via NewMKVFSWithFactories()\n- Add fs_test.go with 20 unit tests using mock implementations\n- Add integration_test.go with 8 FUSE mount/read tests using real data\n\nThe interface-based design enables testing FUSE operations without\nrequiring actual filesystem mounts for unit tests.\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Fix Copilot review feedback and lint issues\n\n- Fix resource leak: store source.Index in adapter and close in Close()\n- Remove unused sourceDir field from dedupReaderAdapter\n- Use exec.LookPath for portable fusermount detection\n- Fix gofmt formatting in fs_test.go\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n---------\n\nCo-authored-by: Claude Opus 4.5 <noreply@anthropic.com>",
          "timestamp": "2026-01-22T10:41:05-05:00",
          "tree_id": "ba294ac974943dc11b5a202a328998fa5ead4ab4",
          "url": "https://github.com/stuckj/mkvdup/commit/4baa187de8bd08cd8c83cc90794ce607c55e0764"
        },
        "date": 1769096599809,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.86,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32379957 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.86,
            "unit": "ns/op",
            "extra": "32379957 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32379957 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32379957 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32369535 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.72,
            "unit": "ns/op",
            "extra": "32369535 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32369535 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32369535 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.69,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32358019 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.69,
            "unit": "ns/op",
            "extra": "32358019 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32358019 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32358019 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32434508 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.72,
            "unit": "ns/op",
            "extra": "32434508 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32434508 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32434508 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.75,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32395663 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.75,
            "unit": "ns/op",
            "extra": "32395663 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32395663 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32395663 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.26,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28403677 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.26,
            "unit": "ns/op",
            "extra": "28403677 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28403677 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28403677 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28499192 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.2,
            "unit": "ns/op",
            "extra": "28499192 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28499192 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28499192 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.35,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28786904 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.35,
            "unit": "ns/op",
            "extra": "28786904 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28786904 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28786904 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28476642 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.21,
            "unit": "ns/op",
            "extra": "28476642 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28476642 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28476642 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.18,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28487295 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.18,
            "unit": "ns/op",
            "extra": "28487295 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28487295 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28487295 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.59,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "113186802 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.59,
            "unit": "ns/op",
            "extra": "113186802 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "113186802 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "113186802 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.65,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "114369256 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.65,
            "unit": "ns/op",
            "extra": "114369256 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "114369256 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "114369256 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.65,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "110059504 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.65,
            "unit": "ns/op",
            "extra": "110059504 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "110059504 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "110059504 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.58,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.58,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.59,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "95771588 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.59,
            "unit": "ns/op",
            "extra": "95771588 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "95771588 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "95771588 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46878,
            "unit": "ns/op\t1398.01 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25569 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46878,
            "unit": "ns/op",
            "extra": "25569 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1398.01,
            "unit": "MB/s",
            "extra": "25569 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25569 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25569 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46719,
            "unit": "ns/op\t1402.76 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46719,
            "unit": "ns/op",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1402.76,
            "unit": "MB/s",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25669 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46518,
            "unit": "ns/op\t1408.84 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26961 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46518,
            "unit": "ns/op",
            "extra": "26961 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1408.84,
            "unit": "MB/s",
            "extra": "26961 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26961 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26961 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46327,
            "unit": "ns/op\t1414.64 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26055 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46327,
            "unit": "ns/op",
            "extra": "26055 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1414.64,
            "unit": "MB/s",
            "extra": "26055 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26055 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26055 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46307,
            "unit": "ns/op\t1415.24 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25966 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46307,
            "unit": "ns/op",
            "extra": "25966 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1415.24,
            "unit": "MB/s",
            "extra": "25966 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25966 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25966 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46376,
            "unit": "ns/op\t1413.15 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25737 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46376,
            "unit": "ns/op",
            "extra": "25737 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1413.15,
            "unit": "MB/s",
            "extra": "25737 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25737 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25737 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46355,
            "unit": "ns/op\t1413.78 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25975 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46355,
            "unit": "ns/op",
            "extra": "25975 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1413.78,
            "unit": "MB/s",
            "extra": "25975 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25975 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25975 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46358,
            "unit": "ns/op\t1413.70 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25848 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46358,
            "unit": "ns/op",
            "extra": "25848 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1413.7,
            "unit": "MB/s",
            "extra": "25848 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25848 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25848 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45944,
            "unit": "ns/op\t1426.43 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25970 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45944,
            "unit": "ns/op",
            "extra": "25970 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1426.43,
            "unit": "MB/s",
            "extra": "25970 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25970 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25970 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46065,
            "unit": "ns/op\t1422.67 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25958 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46065,
            "unit": "ns/op",
            "extra": "25958 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1422.67,
            "unit": "MB/s",
            "extra": "25958 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25958 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25958 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 455,
            "unit": "ns/op\t 562.63 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2639143 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 455,
            "unit": "ns/op",
            "extra": "2639143 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 562.63,
            "unit": "MB/s",
            "extra": "2639143 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2639143 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2639143 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 455,
            "unit": "ns/op\t 562.65 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2620075 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 455,
            "unit": "ns/op",
            "extra": "2620075 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 562.65,
            "unit": "MB/s",
            "extra": "2620075 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2620075 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2620075 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 456,
            "unit": "ns/op\t 561.38 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2629280 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 456,
            "unit": "ns/op",
            "extra": "2629280 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 561.38,
            "unit": "MB/s",
            "extra": "2629280 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2629280 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2629280 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 457.5,
            "unit": "ns/op\t 559.59 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2614585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 457.5,
            "unit": "ns/op",
            "extra": "2614585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 559.59,
            "unit": "MB/s",
            "extra": "2614585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2614585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2614585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 456,
            "unit": "ns/op\t 561.43 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2627557 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 456,
            "unit": "ns/op",
            "extra": "2627557 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 561.43,
            "unit": "MB/s",
            "extra": "2627557 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2627557 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2627557 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 915.9,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1310388 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 915.9,
            "unit": "ns/op",
            "extra": "1310388 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1310388 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1310388 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 928.9,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1273622 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 928.9,
            "unit": "ns/op",
            "extra": "1273622 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1273622 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1273622 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 923.6,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1300032 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 923.6,
            "unit": "ns/op",
            "extra": "1300032 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1300032 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1300032 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 928.5,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1299596 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 928.5,
            "unit": "ns/op",
            "extra": "1299596 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1299596 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1299596 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 941.8,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1297058 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 941.8,
            "unit": "ns/op",
            "extra": "1297058 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1297058 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1297058 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 29927,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "40035 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 29927,
            "unit": "ns/op",
            "extra": "40035 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "40035 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "40035 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 29883,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "40194 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 29883,
            "unit": "ns/op",
            "extra": "40194 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "40194 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "40194 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30076,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39841 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30076,
            "unit": "ns/op",
            "extra": "39841 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39841 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39841 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30030,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "40082 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30030,
            "unit": "ns/op",
            "extra": "40082 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "40082 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "40082 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30140,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "40075 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30140,
            "unit": "ns/op",
            "extra": "40075 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "40075 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "40075 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30323,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39717 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30323,
            "unit": "ns/op",
            "extra": "39717 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39717 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39717 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30430,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39361 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30430,
            "unit": "ns/op",
            "extra": "39361 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39361 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39361 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30191,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39913 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30191,
            "unit": "ns/op",
            "extra": "39913 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39913 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39913 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30102,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38842 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30102,
            "unit": "ns/op",
            "extra": "38842 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38842 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38842 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 29904,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39687 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 29904,
            "unit": "ns/op",
            "extra": "39687 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39687 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39687 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "stuckj@gmail.com",
            "name": "Jonathan Stucklen",
            "username": "stuckj"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "290e525a8d05cc79e2b812a003f38e556a3e4975",
          "message": "Use stuckj identity for benchmark baseline commits (#56)\n\nSince BENCHMARK_PAT uses stuckj's PAT for branch protection bypass,\nthe commit author should match.\n\nCo-authored-by: Claude Opus 4.5 <noreply@anthropic.com>",
          "timestamp": "2026-01-22T14:22:03-05:00",
          "tree_id": "20198f6d4bc4230d3dc63b1b6d24fd8e3c3758cd",
          "url": "https://github.com/stuckj/mkvdup/commit/290e525a8d05cc79e2b812a003f38e556a3e4975"
        },
        "date": 1769109857918,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.74,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32430828 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.74,
            "unit": "ns/op",
            "extra": "32430828 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32430828 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32430828 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.84,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32558125 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.84,
            "unit": "ns/op",
            "extra": "32558125 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32558125 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32558125 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.75,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32400492 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.75,
            "unit": "ns/op",
            "extra": "32400492 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32400492 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32400492 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.84,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32438668 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.84,
            "unit": "ns/op",
            "extra": "32438668 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32438668 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32438668 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.71,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32193900 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.71,
            "unit": "ns/op",
            "extra": "32193900 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32193900 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32193900 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.52,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28477982 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.52,
            "unit": "ns/op",
            "extra": "28477982 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28477982 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28477982 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.05,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28744880 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.05,
            "unit": "ns/op",
            "extra": "28744880 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28744880 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28744880 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.11,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29435847 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.11,
            "unit": "ns/op",
            "extra": "29435847 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29435847 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29435847 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.05,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28799562 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.05,
            "unit": "ns/op",
            "extra": "28799562 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28799562 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28799562 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28562958 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.24,
            "unit": "ns/op",
            "extra": "28562958 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28562958 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28562958 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.56,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "99671946 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.56,
            "unit": "ns/op",
            "extra": "99671946 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "99671946 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "99671946 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.66,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "95238752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.66,
            "unit": "ns/op",
            "extra": "95238752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "95238752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "95238752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.56,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "99548390 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.56,
            "unit": "ns/op",
            "extra": "99548390 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "99548390 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "99548390 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.53,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.53,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.57,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "96382770 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.57,
            "unit": "ns/op",
            "extra": "96382770 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "96382770 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "96382770 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 47572,
            "unit": "ns/op\t1377.61 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25272 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 47572,
            "unit": "ns/op",
            "extra": "25272 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1377.61,
            "unit": "MB/s",
            "extra": "25272 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25272 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25272 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 47281,
            "unit": "ns/op\t1386.11 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25195 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 47281,
            "unit": "ns/op",
            "extra": "25195 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1386.11,
            "unit": "MB/s",
            "extra": "25195 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25195 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25195 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46743,
            "unit": "ns/op\t1402.06 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25425 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46743,
            "unit": "ns/op",
            "extra": "25425 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1402.06,
            "unit": "MB/s",
            "extra": "25425 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25425 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25425 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 47895,
            "unit": "ns/op\t1368.33 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25952 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 47895,
            "unit": "ns/op",
            "extra": "25952 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1368.33,
            "unit": "MB/s",
            "extra": "25952 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25952 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25952 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 47750,
            "unit": "ns/op\t1372.48 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "24982 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 47750,
            "unit": "ns/op",
            "extra": "24982 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1372.48,
            "unit": "MB/s",
            "extra": "24982 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "24982 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "24982 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 47187,
            "unit": "ns/op\t1388.85 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25351 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 47187,
            "unit": "ns/op",
            "extra": "25351 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1388.85,
            "unit": "MB/s",
            "extra": "25351 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25351 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25351 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 47718,
            "unit": "ns/op\t1373.40 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25162 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 47718,
            "unit": "ns/op",
            "extra": "25162 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1373.4,
            "unit": "MB/s",
            "extra": "25162 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25162 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25162 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 47509,
            "unit": "ns/op\t1379.43 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 47509,
            "unit": "ns/op",
            "extra": "25264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1379.43,
            "unit": "MB/s",
            "extra": "25264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25264 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45307,
            "unit": "ns/op\t1446.49 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "24822 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45307,
            "unit": "ns/op",
            "extra": "24822 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1446.49,
            "unit": "MB/s",
            "extra": "24822 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "24822 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "24822 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45705,
            "unit": "ns/op\t1433.90 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26320 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45705,
            "unit": "ns/op",
            "extra": "26320 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1433.9,
            "unit": "MB/s",
            "extra": "26320 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26320 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26320 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 466.8,
            "unit": "ns/op\t 548.47 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2587171 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 466.8,
            "unit": "ns/op",
            "extra": "2587171 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 548.47,
            "unit": "MB/s",
            "extra": "2587171 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2587171 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2587171 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 463.8,
            "unit": "ns/op\t 551.98 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2583705 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 463.8,
            "unit": "ns/op",
            "extra": "2583705 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 551.98,
            "unit": "MB/s",
            "extra": "2583705 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2583705 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2583705 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 463.3,
            "unit": "ns/op\t 552.58 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2588136 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 463.3,
            "unit": "ns/op",
            "extra": "2588136 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 552.58,
            "unit": "MB/s",
            "extra": "2588136 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2588136 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2588136 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 464.5,
            "unit": "ns/op\t 551.18 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2592604 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 464.5,
            "unit": "ns/op",
            "extra": "2592604 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 551.18,
            "unit": "MB/s",
            "extra": "2592604 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2592604 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2592604 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 457.6,
            "unit": "ns/op\t 559.41 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2602387 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 457.6,
            "unit": "ns/op",
            "extra": "2602387 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 559.41,
            "unit": "MB/s",
            "extra": "2602387 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2602387 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2602387 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 928.6,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1294659 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 928.6,
            "unit": "ns/op",
            "extra": "1294659 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1294659 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1294659 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 947.8,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1290754 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 947.8,
            "unit": "ns/op",
            "extra": "1290754 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1290754 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1290754 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 979.3,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1232800 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 979.3,
            "unit": "ns/op",
            "extra": "1232800 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1232800 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1232800 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 960.8,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1210372 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 960.8,
            "unit": "ns/op",
            "extra": "1210372 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1210372 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1210372 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 964.5,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1234282 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 964.5,
            "unit": "ns/op",
            "extra": "1234282 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1234282 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1234282 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30762,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38954 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30762,
            "unit": "ns/op",
            "extra": "38954 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38954 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38954 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30798,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38857 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30798,
            "unit": "ns/op",
            "extra": "38857 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38857 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38857 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30693,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38745 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30693,
            "unit": "ns/op",
            "extra": "38745 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38745 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38745 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30853,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38256 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30853,
            "unit": "ns/op",
            "extra": "38256 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38256 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38256 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30766,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39134 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30766,
            "unit": "ns/op",
            "extra": "39134 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39134 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39134 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30480,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39162 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30480,
            "unit": "ns/op",
            "extra": "39162 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39162 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39162 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30484,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39266 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30484,
            "unit": "ns/op",
            "extra": "39266 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39266 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39266 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30453,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39319 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30453,
            "unit": "ns/op",
            "extra": "39319 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39319 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39319 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30776,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38727 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30776,
            "unit": "ns/op",
            "extra": "38727 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38727 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38727 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30676,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38673 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30676,
            "unit": "ns/op",
            "extra": "38673 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38673 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38673 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "stuckj@gmail.com",
            "name": "Jonathan Stucklen",
            "username": "stuckj"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "a6843bcf42f25ad10454f8a267f80f6db0317d32",
          "message": "Add Dependabot configuration for package updates\n\nConfigured Dependabot for version updates with a weekly schedule.",
          "timestamp": "2026-01-22T20:57:14-05:00",
          "tree_id": "e53cbe9ace8e422741096cd1eee354ea4b88ca26",
          "url": "https://github.com/stuckj/mkvdup/commit/a6843bcf42f25ad10454f8a267f80f6db0317d32"
        },
        "date": 1769133557510,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32107257 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.7,
            "unit": "ns/op",
            "extra": "32107257 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32107257 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32107257 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32227510 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.72,
            "unit": "ns/op",
            "extra": "32227510 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32227510 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32227510 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32206617 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.72,
            "unit": "ns/op",
            "extra": "32206617 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32206617 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32206617 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.81,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32510372 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.81,
            "unit": "ns/op",
            "extra": "32510372 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32510372 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32510372 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.79,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32578615 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.79,
            "unit": "ns/op",
            "extra": "32578615 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32578615 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32578615 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28764331 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.21,
            "unit": "ns/op",
            "extra": "28764331 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28764331 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28764331 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28884025 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.21,
            "unit": "ns/op",
            "extra": "28884025 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28884025 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28884025 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.33,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28884693 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.33,
            "unit": "ns/op",
            "extra": "28884693 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28884693 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28884693 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.25,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28825189 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.25,
            "unit": "ns/op",
            "extra": "28825189 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28825189 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28825189 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.18,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29167706 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.18,
            "unit": "ns/op",
            "extra": "29167706 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29167706 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29167706 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.56,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "97922971 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.56,
            "unit": "ns/op",
            "extra": "97922971 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "97922971 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "97922971 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.45,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.45,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.47,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.47,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.49,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.49,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.53,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "97399411 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.53,
            "unit": "ns/op",
            "extra": "97399411 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "97399411 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "97399411 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46936,
            "unit": "ns/op\t1396.29 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "24962 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46936,
            "unit": "ns/op",
            "extra": "24962 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1396.29,
            "unit": "MB/s",
            "extra": "24962 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "24962 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "24962 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46832,
            "unit": "ns/op\t1399.39 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25308 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46832,
            "unit": "ns/op",
            "extra": "25308 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1399.39,
            "unit": "MB/s",
            "extra": "25308 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25308 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25308 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46020,
            "unit": "ns/op\t1424.06 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25675 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46020,
            "unit": "ns/op",
            "extra": "25675 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1424.06,
            "unit": "MB/s",
            "extra": "25675 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25675 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25675 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46508,
            "unit": "ns/op\t1409.15 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25626 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46508,
            "unit": "ns/op",
            "extra": "25626 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1409.15,
            "unit": "MB/s",
            "extra": "25626 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25626 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25626 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46421,
            "unit": "ns/op\t1411.79 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25802 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46421,
            "unit": "ns/op",
            "extra": "25802 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1411.79,
            "unit": "MB/s",
            "extra": "25802 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25802 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25802 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 44261,
            "unit": "ns/op\t1480.68 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "27278 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 44261,
            "unit": "ns/op",
            "extra": "27278 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1480.68,
            "unit": "MB/s",
            "extra": "27278 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "27278 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "27278 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46902,
            "unit": "ns/op\t1397.29 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46902,
            "unit": "ns/op",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1397.29,
            "unit": "MB/s",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46452,
            "unit": "ns/op\t1410.84 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25677 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46452,
            "unit": "ns/op",
            "extra": "25677 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1410.84,
            "unit": "MB/s",
            "extra": "25677 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25677 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25677 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46315,
            "unit": "ns/op\t1415.02 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25713 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46315,
            "unit": "ns/op",
            "extra": "25713 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1415.02,
            "unit": "MB/s",
            "extra": "25713 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25713 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25713 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46244,
            "unit": "ns/op\t1417.17 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26080 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46244,
            "unit": "ns/op",
            "extra": "26080 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1417.17,
            "unit": "MB/s",
            "extra": "26080 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26080 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26080 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 454.4,
            "unit": "ns/op\t 563.41 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2618215 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 454.4,
            "unit": "ns/op",
            "extra": "2618215 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 563.41,
            "unit": "MB/s",
            "extra": "2618215 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2618215 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2618215 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.6,
            "unit": "ns/op\t 564.41 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2607715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.6,
            "unit": "ns/op",
            "extra": "2607715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 564.41,
            "unit": "MB/s",
            "extra": "2607715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2607715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2607715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.4,
            "unit": "ns/op\t 564.59 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2637909 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.4,
            "unit": "ns/op",
            "extra": "2637909 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 564.59,
            "unit": "MB/s",
            "extra": "2637909 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2637909 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2637909 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 452.5,
            "unit": "ns/op\t 565.79 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2643596 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 452.5,
            "unit": "ns/op",
            "extra": "2643596 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 565.79,
            "unit": "MB/s",
            "extra": "2643596 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2643596 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2643596 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 454.4,
            "unit": "ns/op\t 563.33 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2632453 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 454.4,
            "unit": "ns/op",
            "extra": "2632453 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 563.33,
            "unit": "MB/s",
            "extra": "2632453 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2632453 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2632453 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 922.3,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1296932 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 922.3,
            "unit": "ns/op",
            "extra": "1296932 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1296932 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1296932 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 923,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1306551 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 923,
            "unit": "ns/op",
            "extra": "1306551 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1306551 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1306551 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 950,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1256148 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 950,
            "unit": "ns/op",
            "extra": "1256148 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1256148 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1256148 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 917.2,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1304685 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 917.2,
            "unit": "ns/op",
            "extra": "1304685 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1304685 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1304685 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 924.7,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1305529 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 924.7,
            "unit": "ns/op",
            "extra": "1305529 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1305529 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1305529 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30979,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38893 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30979,
            "unit": "ns/op",
            "extra": "38893 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38893 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38893 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30841,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39043 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30841,
            "unit": "ns/op",
            "extra": "39043 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39043 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39043 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30781,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38580 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30781,
            "unit": "ns/op",
            "extra": "38580 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38580 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38580 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30782,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38601 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30782,
            "unit": "ns/op",
            "extra": "38601 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38601 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38601 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30647,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38767 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30647,
            "unit": "ns/op",
            "extra": "38767 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38767 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38767 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30766,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38889 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30766,
            "unit": "ns/op",
            "extra": "38889 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38889 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38889 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30695,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38226 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30695,
            "unit": "ns/op",
            "extra": "38226 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38226 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38226 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30738,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38301 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30738,
            "unit": "ns/op",
            "extra": "38301 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38301 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38301 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30581,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38638 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30581,
            "unit": "ns/op",
            "extra": "38638 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38638 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38638 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30678,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38734 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30678,
            "unit": "ns/op",
            "extra": "38734 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38734 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38734 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "stuckj-claude-bot@stuckj.me",
            "name": "stuckj-claude-bot",
            "username": "stuckj-claude-bot"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "2199936ed495f4c2251eb276a7378500ee1eaaff",
          "message": "Add directory structure support for FUSE filesystem (#57)\n\n* Add directory structure support for FUSE filesystem (#55)\n\n- Add MKVFSDirNode type for directory nodes with FUSE interfaces\n- Create BuildDirectoryTree function to auto-create directories from paths\n- Integrate directory tree with MKVFSRoot for hierarchical file organization\n- Add read-only error handlers (EROFS) for write operations\n- Add comprehensive unit tests for tree building and directory operations\n- Add integration tests for directory traversal and read-only verification\n- Update documentation with directory structure and OverlayFS examples\n\nFiles with paths like \"Movies/Action/film.mkv\" now create the directory\nhierarchy automatically when mounted via FUSE.\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Address Copilot review feedback for directory structure support\n\n- Add input validation in tree.go: reject paths with \"..\" components\n  (security), empty names, and handle absolute paths\n- Add duplicate detection with warnings when multiple configs specify\n  the same path\n- Add file/directory collision detection with warnings\n- Fix race conditions in MKVFSRoot.Lookup and MKVFSDirNode.Lookup by\n  locking subdir before accessing its fields\n- Add deterministic sorting to Readdir for predictable directory listings\n- Replace custom itoa() with strconv.Itoa in tests\n- Add comprehensive edge case tests for path validation\n- Document path handling rules in FUSE.md\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Replace real movie names with generic names in docs and tests\n\nUse generic names like Video1.mkv instead of real movie titles\nin documentation examples and test cases.\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Clarify --name documentation for directory paths\n\nExplain that each create command produces one dedup file with one name,\nand the directory structure becomes visible when mounting multiple\nconfigs together.\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Fix gofmt formatting in fs_test.go\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n* Fix dependabot configuration\n\nConfigure dependabot to monitor:\n- gomod: Go module dependencies\n- github-actions: GitHub Actions versions\n\nCo-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>\n\n---------\n\nCo-authored-by: Jonathan Stucklen <stuckj@gmail.com>\nCo-authored-by: Claude Opus 4.5 <noreply@anthropic.com>",
          "timestamp": "2026-01-22T21:17:45-05:00",
          "tree_id": "e382c5e77bf204ab287faa397809d217b7f23402",
          "url": "https://github.com/stuckj/mkvdup/commit/2199936ed495f4c2251eb276a7378500ee1eaaff"
        },
        "date": 1769134789883,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.78,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32417845 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.78,
            "unit": "ns/op",
            "extra": "32417845 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32417845 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32417845 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.68,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32500152 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.68,
            "unit": "ns/op",
            "extra": "32500152 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32500152 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32500152 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32309036 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.7,
            "unit": "ns/op",
            "extra": "32309036 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32309036 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32309036 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32536206 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.7,
            "unit": "ns/op",
            "extra": "32536206 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32536206 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32536206 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32430212 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.7,
            "unit": "ns/op",
            "extra": "32430212 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32430212 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32430212 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29166752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.24,
            "unit": "ns/op",
            "extra": "29166752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29166752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29166752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29059708 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.2,
            "unit": "ns/op",
            "extra": "29059708 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29059708 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29059708 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.25,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29078404 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.25,
            "unit": "ns/op",
            "extra": "29078404 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29078404 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29078404 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.22,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28258557 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.22,
            "unit": "ns/op",
            "extra": "28258557 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "28258557 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "28258557 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.22,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29304763 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.22,
            "unit": "ns/op",
            "extra": "29304763 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29304763 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29304763 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.57,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.57,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.53,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "97597070 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.53,
            "unit": "ns/op",
            "extra": "97597070 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "97597070 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "97597070 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.62,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.62,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.52,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.52,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.63,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.63,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46500,
            "unit": "ns/op\t1409.37 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46500,
            "unit": "ns/op",
            "extra": "25585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1409.37,
            "unit": "MB/s",
            "extra": "25585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25585 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46915,
            "unit": "ns/op\t1396.92 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26043 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46915,
            "unit": "ns/op",
            "extra": "26043 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1396.92,
            "unit": "MB/s",
            "extra": "26043 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26043 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26043 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46787,
            "unit": "ns/op\t1400.72 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25600 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46787,
            "unit": "ns/op",
            "extra": "25600 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1400.72,
            "unit": "MB/s",
            "extra": "25600 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25600 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25600 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46583,
            "unit": "ns/op\t1406.85 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46583,
            "unit": "ns/op",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1406.85,
            "unit": "MB/s",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25954 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46288,
            "unit": "ns/op\t1415.83 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25730 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46288,
            "unit": "ns/op",
            "extra": "25730 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1415.83,
            "unit": "MB/s",
            "extra": "25730 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25730 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25730 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46271,
            "unit": "ns/op\t1416.34 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26030 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46271,
            "unit": "ns/op",
            "extra": "26030 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1416.34,
            "unit": "MB/s",
            "extra": "26030 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26030 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26030 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46562,
            "unit": "ns/op\t1407.49 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46562,
            "unit": "ns/op",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1407.49,
            "unit": "MB/s",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25683 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46565,
            "unit": "ns/op\t1407.39 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46565,
            "unit": "ns/op",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1407.39,
            "unit": "MB/s",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46303,
            "unit": "ns/op\t1415.37 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25881 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46303,
            "unit": "ns/op",
            "extra": "25881 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1415.37,
            "unit": "MB/s",
            "extra": "25881 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25881 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25881 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46126,
            "unit": "ns/op\t1420.80 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25987 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46126,
            "unit": "ns/op",
            "extra": "25987 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1420.8,
            "unit": "MB/s",
            "extra": "25987 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25987 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25987 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 454.6,
            "unit": "ns/op\t 563.11 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2647412 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 454.6,
            "unit": "ns/op",
            "extra": "2647412 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 563.11,
            "unit": "MB/s",
            "extra": "2647412 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2647412 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2647412 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 454.2,
            "unit": "ns/op\t 563.68 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2624019 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 454.2,
            "unit": "ns/op",
            "extra": "2624019 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 563.68,
            "unit": "MB/s",
            "extra": "2624019 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2624019 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2624019 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453,
            "unit": "ns/op\t 565.12 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2629812 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453,
            "unit": "ns/op",
            "extra": "2629812 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 565.12,
            "unit": "MB/s",
            "extra": "2629812 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2629812 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2629812 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.9,
            "unit": "ns/op\t 564.06 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2634176 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.9,
            "unit": "ns/op",
            "extra": "2634176 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 564.06,
            "unit": "MB/s",
            "extra": "2634176 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2634176 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2634176 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.6,
            "unit": "ns/op\t 564.40 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2640079 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.6,
            "unit": "ns/op",
            "extra": "2640079 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 564.4,
            "unit": "MB/s",
            "extra": "2640079 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2640079 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2640079 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 919.3,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1298247 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 919.3,
            "unit": "ns/op",
            "extra": "1298247 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1298247 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1298247 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 922.1,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1302109 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 922.1,
            "unit": "ns/op",
            "extra": "1302109 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1302109 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1302109 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 924.4,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1304364 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 924.4,
            "unit": "ns/op",
            "extra": "1304364 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1304364 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1304364 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 923.8,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1304139 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 923.8,
            "unit": "ns/op",
            "extra": "1304139 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1304139 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1304139 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 922.2,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1308379 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 922.2,
            "unit": "ns/op",
            "extra": "1308379 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1308379 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1308379 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30195,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39637 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30195,
            "unit": "ns/op",
            "extra": "39637 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39637 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39637 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30172,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39200 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30172,
            "unit": "ns/op",
            "extra": "39200 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39200 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39200 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30299,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39580 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30299,
            "unit": "ns/op",
            "extra": "39580 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39580 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39580 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30100,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39159 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30100,
            "unit": "ns/op",
            "extra": "39159 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39159 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39159 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30074,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39555 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30074,
            "unit": "ns/op",
            "extra": "39555 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39555 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39555 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 29942,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39595 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 29942,
            "unit": "ns/op",
            "extra": "39595 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39595 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39595 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30029,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39801 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30029,
            "unit": "ns/op",
            "extra": "39801 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39801 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39801 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30026,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39643 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30026,
            "unit": "ns/op",
            "extra": "39643 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39643 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39643 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 29916,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39912 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 29916,
            "unit": "ns/op",
            "extra": "39912 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39912 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39912 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30093,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39452 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30093,
            "unit": "ns/op",
            "extra": "39452 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39452 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39452 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "49699333+dependabot[bot]@users.noreply.github.com",
            "name": "dependabot[bot]",
            "username": "dependabot[bot]"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "671022c9d8ce1d1d55540039077c853a9b5863bf",
          "message": "Bump actions/setup-go from 5 to 6 (#59)\n\nBumps [actions/setup-go](https://github.com/actions/setup-go) from 5 to 6.\n- [Release notes](https://github.com/actions/setup-go/releases)\n- [Commits](https://github.com/actions/setup-go/compare/v5...v6)\n\n---\nupdated-dependencies:\n- dependency-name: actions/setup-go\n  dependency-version: '6'\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>\nCo-authored-by: dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>",
          "timestamp": "2026-01-22T21:25:30-05:00",
          "tree_id": "9f3f9d2b28b3416730da03d91561cebc13f5ff6e",
          "url": "https://github.com/stuckj/mkvdup/commit/671022c9d8ce1d1d55540039077c853a9b5863bf"
        },
        "date": 1769135265090,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.75,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32382636 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.75,
            "unit": "ns/op",
            "extra": "32382636 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32382636 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32382636 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.71,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32488690 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.71,
            "unit": "ns/op",
            "extra": "32488690 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32488690 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32488690 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.71,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32543827 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.71,
            "unit": "ns/op",
            "extra": "32543827 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32543827 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32543827 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.69,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32520500 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.69,
            "unit": "ns/op",
            "extra": "32520500 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32520500 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32520500 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.71,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32481888 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.71,
            "unit": "ns/op",
            "extra": "32481888 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32481888 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32481888 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.28,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29436096 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.28,
            "unit": "ns/op",
            "extra": "29436096 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29436096 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29436096 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.32,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29212141 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.32,
            "unit": "ns/op",
            "extra": "29212141 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29212141 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29212141 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.31,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29479376 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.31,
            "unit": "ns/op",
            "extra": "29479376 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29479376 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29479376 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.26,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29398488 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.26,
            "unit": "ns/op",
            "extra": "29398488 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29398488 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29398488 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.26,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29204028 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.26,
            "unit": "ns/op",
            "extra": "29204028 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29204028 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29204028 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.45,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.45,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.58,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.58,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.56,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.56,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.54,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.54,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.48,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44308,
            "unit": "ns/op\t1479.09 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26590 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44308,
            "unit": "ns/op",
            "extra": "26590 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1479.09,
            "unit": "MB/s",
            "extra": "26590 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26590 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26590 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44464,
            "unit": "ns/op\t1473.92 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26919 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44464,
            "unit": "ns/op",
            "extra": "26919 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1473.92,
            "unit": "MB/s",
            "extra": "26919 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26919 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26919 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44026,
            "unit": "ns/op\t1488.57 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26976 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44026,
            "unit": "ns/op",
            "extra": "26976 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1488.57,
            "unit": "MB/s",
            "extra": "26976 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26976 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26976 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44155,
            "unit": "ns/op\t1484.21 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26953 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44155,
            "unit": "ns/op",
            "extra": "26953 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1484.21,
            "unit": "MB/s",
            "extra": "26953 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26953 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26953 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44369,
            "unit": "ns/op\t1477.06 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26988 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44369,
            "unit": "ns/op",
            "extra": "26988 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1477.06,
            "unit": "MB/s",
            "extra": "26988 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26988 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26988 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 44114,
            "unit": "ns/op\t1485.60 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "27150 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 44114,
            "unit": "ns/op",
            "extra": "27150 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1485.6,
            "unit": "MB/s",
            "extra": "27150 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "27150 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "27150 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46657,
            "unit": "ns/op\t1404.64 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46657,
            "unit": "ns/op",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1404.64,
            "unit": "MB/s",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46636,
            "unit": "ns/op\t1405.28 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46636,
            "unit": "ns/op",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1405.28,
            "unit": "MB/s",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25539 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46651,
            "unit": "ns/op\t1404.81 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46651,
            "unit": "ns/op",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1404.81,
            "unit": "MB/s",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25832 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 43693,
            "unit": "ns/op\t1499.91 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "27534 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 43693,
            "unit": "ns/op",
            "extra": "27534 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1499.91,
            "unit": "MB/s",
            "extra": "27534 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "27534 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "27534 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 454.3,
            "unit": "ns/op\t 563.51 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2642010 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 454.3,
            "unit": "ns/op",
            "extra": "2642010 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 563.51,
            "unit": "MB/s",
            "extra": "2642010 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2642010 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2642010 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 455.2,
            "unit": "ns/op\t 562.35 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2634061 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 455.2,
            "unit": "ns/op",
            "extra": "2634061 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 562.35,
            "unit": "MB/s",
            "extra": "2634061 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2634061 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2634061 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.2,
            "unit": "ns/op\t 564.85 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2640660 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.2,
            "unit": "ns/op",
            "extra": "2640660 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 564.85,
            "unit": "MB/s",
            "extra": "2640660 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2640660 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2640660 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.1,
            "unit": "ns/op\t 565.05 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2625795 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.1,
            "unit": "ns/op",
            "extra": "2625795 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 565.05,
            "unit": "MB/s",
            "extra": "2625795 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2625795 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2625795 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 452.2,
            "unit": "ns/op\t 566.17 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2633642 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 452.2,
            "unit": "ns/op",
            "extra": "2633642 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 566.17,
            "unit": "MB/s",
            "extra": "2633642 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2633642 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2633642 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 918.2,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1311232 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 918.2,
            "unit": "ns/op",
            "extra": "1311232 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1311232 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1311232 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 920.7,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1305636 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 920.7,
            "unit": "ns/op",
            "extra": "1305636 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1305636 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1305636 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 915,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1310793 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 915,
            "unit": "ns/op",
            "extra": "1310793 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1310793 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1310793 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 918.6,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1302714 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 918.6,
            "unit": "ns/op",
            "extra": "1302714 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1302714 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1302714 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 922.9,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1309116 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 922.9,
            "unit": "ns/op",
            "extra": "1309116 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1309116 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1309116 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30760,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38700 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30760,
            "unit": "ns/op",
            "extra": "38700 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38700 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38700 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30722,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38736 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30722,
            "unit": "ns/op",
            "extra": "38736 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38736 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38736 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30741,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "37560 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30741,
            "unit": "ns/op",
            "extra": "37560 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "37560 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "37560 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30589,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38865 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30589,
            "unit": "ns/op",
            "extra": "38865 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38865 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38865 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30716,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38719 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30716,
            "unit": "ns/op",
            "extra": "38719 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38719 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38719 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30675,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38524 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30675,
            "unit": "ns/op",
            "extra": "38524 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38524 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38524 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30944,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38724 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30944,
            "unit": "ns/op",
            "extra": "38724 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38724 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38724 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30675,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38170 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30675,
            "unit": "ns/op",
            "extra": "38170 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38170 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38170 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30646,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38804 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30646,
            "unit": "ns/op",
            "extra": "38804 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38804 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38804 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30688,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38568 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30688,
            "unit": "ns/op",
            "extra": "38568 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38568 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38568 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "49699333+dependabot[bot]@users.noreply.github.com",
            "name": "dependabot[bot]",
            "username": "dependabot[bot]"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "c9f70d050fa43e4e784816eea42228e84890cc67",
          "message": "Bump actions/download-artifact from 4 to 7 (#63)\n\nBumps [actions/download-artifact](https://github.com/actions/download-artifact) from 4 to 7.\n- [Release notes](https://github.com/actions/download-artifact/releases)\n- [Commits](https://github.com/actions/download-artifact/compare/v4...v7)\n\n---\nupdated-dependencies:\n- dependency-name: actions/download-artifact\n  dependency-version: '7'\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>\nCo-authored-by: dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>",
          "timestamp": "2026-01-22T21:26:13-05:00",
          "tree_id": "120da6940e5d95c8af2c3130bfb5f25c480e7c31",
          "url": "https://github.com/stuckj/mkvdup/commit/c9f70d050fa43e4e784816eea42228e84890cc67"
        },
        "date": 1769135306402,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32480755 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.7,
            "unit": "ns/op",
            "extra": "32480755 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32480755 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32480755 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.69,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32533378 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.69,
            "unit": "ns/op",
            "extra": "32533378 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32533378 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32533378 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.84,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32562542 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.84,
            "unit": "ns/op",
            "extra": "32562542 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32562542 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32562542 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.71,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32381091 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.71,
            "unit": "ns/op",
            "extra": "32381091 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32381091 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32381091 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential",
            "value": 36.76,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32521536 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - ns/op",
            "value": 36.76,
            "unit": "ns/op",
            "extra": "32521536 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "32521536 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Sequential - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "32521536 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.61,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29047230 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.61,
            "unit": "ns/op",
            "extra": "29047230 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29047230 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29047230 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.19,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29478793 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.19,
            "unit": "ns/op",
            "extra": "29478793 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29478793 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29478793 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.15,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29581696 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.15,
            "unit": "ns/op",
            "extra": "29581696 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29581696 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29581696 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.15,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29647752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.15,
            "unit": "ns/op",
            "extra": "29647752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29647752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29647752 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random",
            "value": 40.23,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29098858 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - ns/op",
            "value": 40.23,
            "unit": "ns/op",
            "extra": "29098858 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29098858 times\n4 procs"
          },
          {
            "name": "BenchmarkGetEntry_Random - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29098858 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.42,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.42,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.45,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.45,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.41,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "97244944 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.41,
            "unit": "ns/op",
            "extra": "97244944 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "97244944 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "97244944 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.42,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.42,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset",
            "value": 10.36,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "99861200 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - ns/op",
            "value": 10.36,
            "unit": "ns/op",
            "extra": "99861200 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "99861200 times\n4 procs"
          },
          {
            "name": "BenchmarkGetMkvOffset - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "99861200 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44283,
            "unit": "ns/op\t1479.92 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26380 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44283,
            "unit": "ns/op",
            "extra": "26380 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1479.92,
            "unit": "MB/s",
            "extra": "26380 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26380 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26380 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 44492,
            "unit": "ns/op\t1472.97 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "27124 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 44492,
            "unit": "ns/op",
            "extra": "27124 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1472.97,
            "unit": "MB/s",
            "extra": "27124 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "27124 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "27124 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46201,
            "unit": "ns/op\t1418.51 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46201,
            "unit": "ns/op",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1418.51,
            "unit": "MB/s",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25963 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46109,
            "unit": "ns/op\t1421.32 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25938 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46109,
            "unit": "ns/op",
            "extra": "25938 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1421.32,
            "unit": "MB/s",
            "extra": "25938 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25938 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25938 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential",
            "value": 46158,
            "unit": "ns/op\t1419.83 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26053 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - ns/op",
            "value": 46158,
            "unit": "ns/op",
            "extra": "26053 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - MB/s",
            "value": 1419.83,
            "unit": "MB/s",
            "extra": "26053 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26053 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Sequential - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26053 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45663,
            "unit": "ns/op\t1435.20 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26276 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45663,
            "unit": "ns/op",
            "extra": "26276 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1435.2,
            "unit": "MB/s",
            "extra": "26276 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26276 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26276 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45678,
            "unit": "ns/op\t1434.75 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26230 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45678,
            "unit": "ns/op",
            "extra": "26230 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1434.75,
            "unit": "MB/s",
            "extra": "26230 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26230 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26230 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 46017,
            "unit": "ns/op\t1424.17 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 46017,
            "unit": "ns/op",
            "extra": "26262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1424.17,
            "unit": "MB/s",
            "extra": "26262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26262 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45895,
            "unit": "ns/op\t1427.95 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "25940 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45895,
            "unit": "ns/op",
            "extra": "25940 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1427.95,
            "unit": "MB/s",
            "extra": "25940 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "25940 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "25940 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random",
            "value": 45577,
            "unit": "ns/op\t1437.93 MB/s\t   84192 B/op\t      11 allocs/op",
            "extra": "26220 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - ns/op",
            "value": 45577,
            "unit": "ns/op",
            "extra": "26220 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - MB/s",
            "value": 1437.93,
            "unit": "MB/s",
            "extra": "26220 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - B/op",
            "value": 84192,
            "unit": "B/op",
            "extra": "26220 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Random - allocs/op",
            "value": 11,
            "unit": "allocs/op",
            "extra": "26220 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453.9,
            "unit": "ns/op\t 563.96 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2582186 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453.9,
            "unit": "ns/op",
            "extra": "2582186 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 563.96,
            "unit": "MB/s",
            "extra": "2582186 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2582186 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2582186 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 451.7,
            "unit": "ns/op\t 566.70 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2647808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 451.7,
            "unit": "ns/op",
            "extra": "2647808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 566.7,
            "unit": "MB/s",
            "extra": "2647808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2647808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2647808 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 452.3,
            "unit": "ns/op\t 565.94 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2636715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 452.3,
            "unit": "ns/op",
            "extra": "2636715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 565.94,
            "unit": "MB/s",
            "extra": "2636715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2636715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2636715 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 453,
            "unit": "ns/op\t 565.07 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2653096 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 453,
            "unit": "ns/op",
            "extra": "2653096 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 565.07,
            "unit": "MB/s",
            "extra": "2653096 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2653096 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2653096 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small",
            "value": 452.6,
            "unit": "ns/op\t 565.68 MB/s\t     287 B/op\t       2 allocs/op",
            "extra": "2639712 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - ns/op",
            "value": 452.6,
            "unit": "ns/op",
            "extra": "2639712 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - MB/s",
            "value": 565.68,
            "unit": "MB/s",
            "extra": "2639712 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - B/op",
            "value": 287,
            "unit": "B/op",
            "extra": "2639712 times\n4 procs"
          },
          {
            "name": "BenchmarkReadAt_Small - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "2639712 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 911.8,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1312222 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 911.8,
            "unit": "ns/op",
            "extra": "1312222 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1312222 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1312222 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 917.9,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1297221 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 917.9,
            "unit": "ns/op",
            "extra": "1297221 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1297221 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1297221 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 916.1,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1302476 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 916.1,
            "unit": "ns/op",
            "extra": "1302476 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1302476 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1302476 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 917.1,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1309256 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 917.1,
            "unit": "ns/op",
            "extra": "1309256 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1309256 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1309256 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange",
            "value": 917.9,
            "unit": "ns/op\t    1248 B/op\t       5 allocs/op",
            "extra": "1308021 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - ns/op",
            "value": 917.9,
            "unit": "ns/op",
            "extra": "1308021 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - B/op",
            "value": 1248,
            "unit": "B/op",
            "extra": "1308021 times\n4 procs"
          },
          {
            "name": "BenchmarkFindEntriesForRange - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1308021 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30579,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38828 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30579,
            "unit": "ns/op",
            "extra": "38828 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38828 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38828 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30417,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39115 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30417,
            "unit": "ns/op",
            "extra": "39115 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39115 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39115 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30337,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39183 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30337,
            "unit": "ns/op",
            "extra": "39183 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39183 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39183 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30310,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38991 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30310,
            "unit": "ns/op",
            "extra": "38991 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38991 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38991 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader",
            "value": 30285,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39154 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - ns/op",
            "value": 30285,
            "unit": "ns/op",
            "extra": "39154 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39154 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReader - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39154 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30477,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38838 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30477,
            "unit": "ns/op",
            "extra": "38838 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38838 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38838 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30592,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39040 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30592,
            "unit": "ns/op",
            "extra": "39040 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39040 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39040 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30294,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38955 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30294,
            "unit": "ns/op",
            "extra": "38955 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38955 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38955 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30371,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "38893 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30371,
            "unit": "ns/op",
            "extra": "38893 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "38893 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "38893 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy",
            "value": 30316,
            "unit": "ns/op\t    1064 B/op\t      27 allocs/op",
            "extra": "39164 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - ns/op",
            "value": 30316,
            "unit": "ns/op",
            "extra": "39164 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - B/op",
            "value": 1064,
            "unit": "B/op",
            "extra": "39164 times\n4 procs"
          },
          {
            "name": "BenchmarkNewReaderLazy - allocs/op",
            "value": 27,
            "unit": "allocs/op",
            "extra": "39164 times\n4 procs"
          }
        ]
      }
    ]
  }
}