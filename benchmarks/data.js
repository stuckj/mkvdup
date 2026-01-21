window.BENCHMARK_DATA = {
  "lastUpdate": 1769025856867,
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
      }
    ]
  }
}