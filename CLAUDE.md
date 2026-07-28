# Notes for AI agents

Repo-specific things that are easy to get wrong here. Everything useful to humans lives in the
normal docs and is linked rather than duplicated — prefer following the link over trusting a
summary in this file.

- [CONTRIBUTING.md](CONTRIBUTING.md) — environment, pre-commit checklist, test policy
- [DESIGN.md](DESIGN.md) — architecture; [docs/](docs/) — CLI, FUSE, file format, matching
- [RELEASING.md](RELEASING.md) — release and canary process

## `go test ./...` does not run everything

Tests that can only run in one context are excluded by **build tag**, not skipped, so a plain
run silently omits them. See
[CONTRIBUTING.md](CONTRIBUTING.md#never-skip-a-test-to-express-cannot-run-here) for the policy
and the tag table.

```bash
go test ./...                                                    # default set
go test -race ./...                                              # CI runs -race in Unit Tests only
go test -tags=rootonly ./internal/security/...                   # must be root
go test -tags=integration ./internal/fuse/...                    # needs FUSE + testdata
go test -tags=integration,nonroot -run TestFUSE_Permission ./internal/fuse/...
```

**CI fails on any skipped test** (`scripts/check-no-skips.py`). If a test cannot run somewhere,
give it a build tag — do not reach for `t.Skip`. This exists because seven tests once went
completely unexecuted behind green checks (#201).

## Tooling gotchas

- **Put git worktrees *beside* the repo, never inside it.** The convention here is a sibling
  directory named after the topic — `../mkvdup-fuse-timestamps`, `../mkvdup-nix-packaging`. A
  worktree nested under the checkout is essentially a second clone living inside the repo, which
  is confusing to browse and to reason about. Claude Code's `EnterWorktree` tool defaults to
  `.claude/worktrees/<name>` *inside* the repo, so do not create one with it directly: run
  `git worktree add ../mkvdup-<topic> -b <branch>` yourself, then enter it by passing that path
  to `EnterWorktree`.
- **`gh pr edit` is broken against this repo.** It fails on the deprecated projects-classic
  GraphQL field and **silently discards the edit** while appearing to succeed. Use the REST API
  instead:
  `gh api -X PATCH repos/stuckj/mkvdup/pulls/<n> --input body.json`. Same for labels:
  `gh api -X POST repos/stuckj/mkvdup/issues/<n>/labels`.
- **Only the repo owner can request a Copilot review.** A bot account's request returns success
  but creates no timeline event and no review.
- **Releases are `workflow_dispatch`**, take a version *without* the `v` prefix, and default to
  the branch they were dispatched from — not `main`. See [RELEASING.md](RELEASING.md).

## Verifying claims about this codebase

Three rules, each learned from a wrong answer produced here:

1. **When independent surfaces appear to fail identically, suspect the checker.** Four separate
   docs "missing" the same twelve flags was a broken regex, not four broken docs. This caught
   six consecutive false positives during a docs audit.
2. **Sanity-check a pattern against a known-present control before trusting a negative.** A
   suspiciously clean "nothing found" is usually a bad pattern — `\|` under `grep -E` matches a
   literal pipe and once "proved" whole test categories didn't exist.
3. **Parse the source of truth, not prose.** Comparing against the dispatch table in
   `cmd/mkvdup/main.go` and `<cmd> --help` found real gaps; grepping documentation produced
   only false ones. Note flags are parsed both as `case "--flag"` and `if arg == "--flag"`.

Corollary: documentation here has repeatedly claimed things that were not true — tests that
never ran, a race detector that was never enabled, a linter that was deprecated and unused.
When a doc asserts that something is covered, verify it before relying on it, and fix the doc
rather than working around it.

## Surfaces that must stay in sync

A user-visible CLI change touches more than the code. When adding or changing a command or
flag, update all of:

`cmd/mkvdup/help.go` · `docs/mkvdup.1` · `docs/CLI.md` ·
`scripts/mkvdup-completion.bash` · `scripts/mkvdup-completion.zsh` · `scripts/mkvdup.fish`

The man page is the one most often forgotten. Note the completions use different syntax for
long options (fish uses `-l name` without dashes), and the man page escapes hyphens as `\-`, so
naive greps across these files give misleading results — see rule 1 above.
