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
- **Never write a bracketed skip-ci marker in a commit message — not even when describing one.**
  GitHub scans the *entire* message, body included, for `[skip ci]`, `[ci skip]`, `[no ci]`,
  `[skip actions]` and `[actions skip]`. A commit whose body merely *explains* the markers
  disables CI on itself. The affected runs are never created, so nothing reports as "skipped" and
  the PR still looks green from whatever is not skip-ci gated — here CodeQL and the
  `pull_request_target` project job. This bites disproportionately often because release tooling
  is exactly what needs to talk about the markers. When writing about them use the regex form
  `\[(skip ci|...)\]` — safe, since the bracket is followed by `(` — or prose like "skip-ci
  marker". It survives merge too: GitHub seeds the squash message from the commit messages, so a
  marker in *any* commit on the branch can follow the squash onto `main`. **After pushing, confirm
  the expected workflows actually appear** in
  `gh api "repos/stuckj/mkvdup/actions/runs?branch=<branch>"` — a green PR is not evidence they
  ran.
- **`gh pr edit` and `gh issue view` are broken against this repo.** Both fail on the deprecated
  projects-classic GraphQL field, and `gh pr edit` **silently discards the edit** while appearing
  to succeed. Use the REST API instead:
  `gh api -X PATCH repos/stuckj/mkvdup/pulls/<n> --input body.json`,
  `gh api repos/stuckj/mkvdup/issues/<n>`. Same for labels:
  `gh api -X POST repos/stuckj/mkvdup/issues/<n>/labels`.
- **Only the repo owner can request a Copilot review.** A bot account's request returns success
  but creates no timeline event and no review.
- **Releases are `workflow_dispatch`**, take a version *without* the `v` prefix, and must be
  dispatched from a **branch** — the commit to tag is resolved from that branch's history, and it
  is not necessarily `main`. There is no commit-SHA input. See [RELEASING.md](RELEASING.md).

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
