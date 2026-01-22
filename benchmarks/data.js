window.BENCHMARK_DATA = {
  "lastUpdate": 1769052340725,
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
      }
    ]
  }
}