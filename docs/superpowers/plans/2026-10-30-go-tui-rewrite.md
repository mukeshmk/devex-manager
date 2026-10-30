# DevEx Manager v2 (Go + TUI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Bash `git wt` / `dx` tools with one Go binary that keeps every command, flag, config and state file working, adds width-aware output and two full-screen apps, makes sync fast, and tightens deletion safety.

**Architecture:** One binary, `cmd/dx`, dispatching on `argv[0]` (`dx` or `git-wt`) into two Cobra command trees. Domain logic lives in small `internal/` packages that are pure where possible (sync planning, ctx file edits, skills planning) and are wired to I/O at the CLI layer. Full-screen views are Bubble Tea models that reuse the same data collectors as the print commands.

**Tech Stack:** Go (`go 1.27` in `go.mod`), `github.com/spf13/cobra`, Charm `bubbletea`, `bubbles`, `lipgloss` (incl. `lipgloss/table`), `huh`, `golang.org/x/term`. Use the latest stable major of each Charm library at the time of Task 1 and keep imports consistent with it.

**Spec:** `docs/superpowers/specs/2026-10-30-go-tui-rewrite-design.md` (approved). Executors read both.

## Global Constraints

- Module path: `github.com/mukeshmk/devex-manager`. Binary name `dx`; `git-wt` is a symlink to it.
- Platforms: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64. `CGO_ENABLED=0`.
- Config format, keys, defaults, file locations unchanged:
  - Repo: `<repo_root>/.devex.conf`; defaults `base_branch=main`, `main_worktree_name=main`, `symlinks.paths=.claude,.kiro,.vscode`, `copies.paths=` (empty), `worktree.naming_strategy=ticket-prefix`.
  - Skills: `~/.devex.conf` `[skills]`; defaults `master_dir=~/personal/skills`, `tools=claude,kiro,gemini`, dirs `claude=~/.claude/skills`, `kiro=~/.kiro/skills`, `gemini=~/.gemini/antigravity/skills`. Env overrides `DX_MASTER_SKILLS_DIR`, `DX_TOOLS`, `DX_<TOOL>_SKILLS_DIR` then `DX_<TOOL>_DIR`, `DX_<TOOL>_SYNC_ONLY`, `DX_SYNC_ONLY`, `DX_IGNORE` take precedence over the file.
  - State: sync manifest `<common>/info/devex/sync-manifest` (lines `rel|mtime|size`, mtime in Unix seconds); ctx notes `<common>/info/devex/contexts/<branch with / as __>.md`; skills backups `<master>/.backups/YYYY-MM-DD-HHMMSS/`; **new** sync backups `<common>/info/devex/sync-backups/YYYY-MM-DD-HHMMSS/<worktree-folder>/<rel>`.
- `<common>` is always `git rev-parse --git-common-dir` made absolute. Never use `.git` directly.
- Non-TTY never opens a full-screen app and never blocks. Prompter rules are in Task 4.
- Copies preserve file mode and mtime (parity with `cp -p`).
- Git operations against **this repo** (status/add/commit) run only through the `personal-git:v3` container. Never push. Commit step command for every task:
  ```bash
  name="$(basename "$(pwd)")"; docker run --rm -v "$(pwd)":"/$name" -w "/$name" personal-git:v3 bash -c 'git add <files> && git commit -m "<msg>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git log --oneline -1'
  ```
- Tests may run the host `git` binary **only inside `t.TempDir()` repos** created by `internal/testutil`, with `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_SYSTEM=/dev/null`, and fixed `GIT_AUTHOR_*`/`GIT_COMMITTER_*` set by the helper. Never against this repo.
- Bash scripts stay in the repo until Task 17. Do not edit them.
- Test command for all tasks: `go test ./...` from repo root (plus `-run` filters shown per step).

## Review Focus

1. **Paths with spaces** in worktree folders and copy paths: every command must treat them as single paths (sync, rm, clean, status). Test pinned in Task 6 and Task 3.
2. **"Am I inside this worktree" prefix collision:** cwd `/r/DEVX-12` must not block removing `/r/DEVX-1` (Bash's `"$(pwd)" == "$path"*` gets this wrong). Equal or `path+"/"` prefix only. Test pinned in Task 7.
3. **Very narrow terminals and live resize:** below 60 columns the tables truncate without panicking, and the apps show a "terminal too small" line instead of a broken layout. Tests pinned in Task 4 and Task 12.
4. **Sync failing midway** (read-only destination, disk full): stop at the first failed op, do not rewrite the manifest, print the backup dir. Test pinned in Task 6.
5. **Odd `git worktree list` entries:** bare entry, detached HEAD, `locked`/`prunable` lines must parse without dropping or mislabeling worktrees. Test pinned in Task 3.

---

## File Structure

```
go.mod, go.sum
cmd/dx/main.go                 argv[0] dispatch, git-wt passthrough exec
internal/cli/dx.go             dx root + subcommand wiring
internal/cli/gitwt.go          git-wt root + subcommand wiring
internal/config/ini.go         shared INI reader
internal/config/repo.go        RepoConfig (.devex.conf)
internal/config/skills.go      SkillsConfig (~/.devex.conf + env)
internal/gitx/git.go           Run helper, Repo discovery
internal/gitx/worktrees.go     worktree list parse + per-branch queries
internal/testutil/repo.go      temp bare-repo fixture for tests
internal/ui/table.go           width-aware table
internal/ui/prompt.go          Prompter (TTY / line / defaults)
internal/ui/term.go            IsTTY, Width
internal/wtsync/scan.go        file state collection
internal/wtsync/plan.go        classification rules -> Plan
internal/wtsync/apply.go       backups, copy, delete, manifest, prune
internal/wt/safety.go          Safe(), inside-worktree check
internal/wt/rm.go, clean.go, add.go, clone.go, status.go
internal/wtapp/app.go          git wt full-screen model
internal/ctx/ctx.go            context file parse/edit
internal/nb/nb.go              notebook commands
internal/skills/scan.go, plan.go, apply.go, ops.go
internal/skillsapp/app.go      dx skills grid model
.github/workflows/release.yml
install.sh, uninstall.sh       rewritten
```

---

### Task 1: Scaffold binary, dispatch and git-wt passthrough

**Files:**
- Create: `go.mod`, `cmd/dx/main.go`, `internal/cli/dx.go`, `internal/cli/gitwt.go`, `internal/cli/cli_test.go`
- Modify: `.gitignore` (add `/dx`, `/dist/`)

**Interfaces:**
- Produces:
  - `func cli.Main(argv0 string, args []string) int`: selects tree by `filepath.Base(argv0)` (`git-wt` → git-wt tree, anything else → dx tree).
  - `func cli.Passthrough(args []string) (gitArgs []string, ok bool)`: returns `["worktree", verb, rest...]` for `lock|move|prune|repair|unlock`, and for `list` only when `len(args) > 1`.
  - `func cli.NewGitWtRoot() *cobra.Command`, `func cli.NewDxRoot() *cobra.Command`, with subcommand stubs registered by later tasks via `cli.registerWt(cmd)` / `cli.registerDx(group string, cmd)`.
  - `main.go` calls `syscall.Exec` on the resolved `git` path when `Passthrough` returns ok (preserves exit code and argv, matching the Bash `exec`).

- [ ] **Step 1: Write failing tests** `TestPassthrough` (table: `lock x` → `worktree lock x`, `list` → not ok, `list --porcelain` → `worktree list --porcelain`, `add foo` → not ok) and `TestMainUnknownPrintsHelp` (`Main("git-wt", ["bogus"])` returns `1` and output contains `Usage: git wt <command> [arguments]`; `Main("dx", ["bogus"])` returns 1 and lists `skills`, `nb`, `ctx`).
- [ ] **Step 2: Run** `go test ./internal/cli/ -run 'TestPassthrough|TestMainUnknown' -v`, expect FAIL (undefined).
- [ ] **Step 3: Implement** `go mod init github.com/mukeshmk/devex-manager`, add cobra, implement the interfaces. Help text for `git wt` copies the command list in `git-wt` lines 41-53 **minus** the `(supports: --force, -f)` note on `sync` (spec: known oddity).
- [ ] **Step 4: Run** `go test ./internal/cli/ -v` and `go build -o dx ./cmd/dx && ln -sf dx git-wt && ./git-wt bogus; echo $?`, expect PASS and `1`.
- [ ] **Step 5: Commit** `go.mod go.sum cmd internal/cli .gitignore`, message `feat: scaffold Go binary with argv[0] dispatch`.

### Task 2: Config loading

**Files:**
- Create: `internal/config/ini.go`, `repo.go`, `skills.go`, `config_test.go`

**Interfaces:**
- Produces:
  - `type RepoConfig struct { BaseBranch, MainWorktreeName string; SymlinkPaths, CopyPaths []string; NamingStrategy string }`
  - `func LoadRepo(repoRoot string) (RepoConfig, error)`: missing file → defaults, no error. Paths split on `,` and trimmed, empties dropped.
  - `func ResolveWorktreeName(branch, strategy string) (string, error)`: `ticket-prefix` = first two `-`-separated fields of the whole branch string (parity with `cut -d- -f1,2`); `full-branch` = `/` replaced by `--`; `custom` and unknown → error with the Bash messages from `devex-lib.sh:79-87`.
  - `type SkillTool struct { Name, Dir string; SyncOnly []string }`
  - `type SkillsConfig struct { MasterDir string; Tools []SkillTool; SyncOnly, Ignore []string; Warning string }`
  - `func LoadSkills(home string, getenv func(string) string) (SkillsConfig, error)`: precedence and defaults per Global Constraints; `~` expanded; trailing `/` trimmed from MasterDir; tools whose dir doesn't exist are dropped (parity). `Warning` holds the missing-file or template-placeholder message from `dx-skills-lib.sh:48-62`.
  - `func (c SkillsConfig) IsIgnored(name string) bool` (parity with `is_ignored`).

- [ ] **Step 1: Write failing tests:** `TestLoadRepoDefaults` (no file → the five defaults), `TestLoadRepoParses` (file with comments, blank lines, spaces around `=`, `copies.paths = .env , config/local.yaml` → `[".env","config/local.yaml"]`), `TestResolveWorktreeName` (`DEVX-1423-retry` → `DEVX-1423`, `feature/DEVX-1-x` → `feature/DEVX-1`, full-branch `a/b` → `a--b`, `custom` → error), `TestLoadSkillsPrecedence` (env `DX_CLAUDE_SKILLS_DIR` beats file `claude_dir`, `DX_CLAUDE_DIR` used when the former unset, missing dir dropped), `TestLoadSkillsWarning` (no file → warning mentions `~/.devex.conf not found`; file containing `<tool_1>` → template warning).
- [ ] **Step 2: Run** `go test ./internal/config/ -v`, expect FAIL.
- [ ] **Step 3: Implement.** One INI reader `func readINI(path string) (map[string]map[string]string, error)` used by both loaders.
- [ ] **Step 4: Run** `go test ./internal/config/ -v`, expect PASS.
- [ ] **Step 5: Commit** `internal/config`, `feat(config): load repo and skills config`.

### Task 3: Git helpers and test fixture

**Files:**
- Create: `internal/gitx/git.go`, `internal/gitx/worktrees.go`, `internal/gitx/gitx_test.go`, `internal/testutil/repo.go`

**Interfaces:**
- Produces:
  - `func Run(dir string, args ...string) (string, error)`: runs `git`, returns trimmed stdout; error includes stderr.
  - `type Repo struct { CommonDir, Root string }` and `func Open(cwd string) (*Repo, error)`. Root resolution is the parity logic in `git-wt-sync:38-56` (bare repo → common dir, parent if basename is `.git`; else `--show-toplevel`).
  - `type Worktree struct { Path, Branch string; Bare, Detached, Locked, Prunable bool }`, `func (r *Repo) Worktrees() ([]Worktree, error)`: parses `git worktree list --porcelain`. Branch is short name (`refs/heads/` stripped).
  - `func (r *Repo) MergedBranches(base string) (map[string]bool, error)`
  - `func (r *Repo) RemoteHeads(remote string) (map[string]bool, error)`: one `git ls-remote --heads <remote>` call.
  - `func (r *Repo) Upstream(branch string) (remote string, ok bool)`: from `branch.<b>.remote`.
  - `func AheadBehind(dir, branch string) (ahead, behind int, hasUpstream bool, err error)`
  - `func ChangedFiles(dir string) ([]string, error)`: `status --porcelain` lines.
  - `func UnpushedCount(dir, branch string) (int, error)`: `rev-list --count <branch> --not --remotes`.
  - `testutil.NewRepo(t) *Fixture` creates `<tmp>/origin.git` (bare, with an initial commit on `main`), then a `git wt clone`-style layout `<tmp>/repo/.git` (bare) + `<tmp>/repo/main`. `func (f *Fixture) AddWorktree(t, folder, branch string) string` returns abs path. `func (f *Fixture) Env() []string` returns the isolated git env (also applied via `t.Setenv`).

- [ ] **Step 1: Write failing tests:** `TestOpenBareLayout` (Root = `<tmp>/repo`, CommonDir = `<tmp>/repo/.git`, from inside `main`), `TestWorktreesParsesOddEntries` (fixture + one detached worktree + one locked (`git worktree lock`) + folder `"with space"`: expect bare entry flagged, detached has empty Branch and `Detached`, locked flagged, the spaced path intact), `TestRemoteHeadsSingleCall` (push branch `a`, delete nothing: map has `main`,`a`).
- [ ] **Step 2: Run** `go test ./internal/gitx/ -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/gitx/ -v`, expect PASS.
- [ ] **Step 5: Commit** `internal/gitx internal/testutil`, `feat(gitx): git helpers and temp repo fixture`.

### Task 4: Terminal, table and prompter

**Files:**
- Create: `internal/ui/term.go`, `table.go`, `prompt.go`, `ui_test.go`

**Interfaces:**
- Produces:
  - `func IsTTY(f *os.File) bool`, `func Width(f *os.File) int` (falls back to `$COLUMNS`, then 100).
  - `type Column struct { Title string; Min int; Flex bool }`
  - `func Table(cols []Column, rows [][]string, width int) string`: rounded lipgloss table; fixed columns get content width; `Flex` columns share the remaining width and truncate with `…`; never exceeds `width`; cells may contain lipgloss-styled text (width measured with `lipgloss.Width`, so colours never misalign). When `width` < sum of `Min`, drop Flex columns to `Min` and let the table exceed nothing by truncating every cell to `Min`.
  - `type Prompter interface { Confirm(q string, def bool) bool; Choose(q string, options []string, def int) int; MultiSelect(q string, options []string, preselected []bool) []bool }`
  - `func NewPrompter(in *os.File, out io.Writer) Prompter`: TTY → huh-based; else `LinePrompter`.
  - `type LinePrompter struct{ R *bufio.Reader; W io.Writer }`: reads one line per question; `y/Y` → true; empty line → `def`; **EOF → false for Confirm, `def` for Choose, all-false for MultiSelect**. `MultiSelect` accepts `y` (all preselected), `n`, or space-separated 1-based numbers (parity with clean's `individual`).
  - `type DefaultPrompter struct{}`: always returns defaults (used by non-TTY sync).

- [ ] **Step 1: Write failing tests:** `TestTableFitsWidth` (6 columns, a 60-char branch in the Flex column, widths 60/90/160: every output line's `lipgloss.Width` ≤ width; at 160 the full branch appears; at 90 it ends with `…`), `TestTableColouredCellsAlign` (cells wrapped in lipgloss colour: all rows have equal visible width), `TestTableNarrowNoPanic` (width 20), `TestLinePrompterEOF` (empty reader: Confirm(…, true) == false), `TestLinePrompterMulti` (`"1 3\n"` with 3 options → `[true,false,true]`).
- [ ] **Step 2: Run** `go test ./internal/ui/ -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -v`, expect PASS.
- [ ] **Step 5: Commit** `internal/ui`, `feat(ui): width-aware table and prompters`.

### Task 5: Sync scan and plan (pure)

**Files:**
- Create: `internal/wtsync/scan.go`, `plan.go`, `plan_test.go`

**Interfaces:**
- Consumes: `config.RepoConfig`, `gitx.Worktree`.
- Produces:
  - `type Stat struct { Exists bool; Mtime, Size int64 }`
  - `type FileState struct { Rel string; Recorded Stat; Main Stat; Others map[string]Stat }` (key = worktree abs path)
  - `func ReadManifest(path string) (map[string]Stat, error)`, `func WriteManifest(path string, entries map[string]Stat) error` (sorted lines).
  - `func Scan(main string, others []string, copyPaths []string, manifest map[string]Stat) ([]FileState, error)`: union of files under each copy path (file or dir, walked with `filepath.WalkDir`) across all worktrees, sorted; stats via `os.Lstat`, worktrees scanned concurrently.
  - `func FindConflictMarkers(main string, copyPaths []string) []string`: files matching `*.devex-conflict-*` **only inside copy paths**.
  - `type Loc struct { WT, Rel string }` (`Path()` = `filepath.Join(WT, Rel)`)
  - `type Op struct { Kind OpKind /* Copy|Delete */; Src, Dst Loc }`
  - `type Choice struct { Question string; Options []string; Default int; Ops [][]Op; YesNo bool }`
  - `type Step struct { Rel string; Summary string; Ops []Op; Choice *Choice; Then []Step }` (`Then` holds the follow-up question for the "removed in WT" flow)
  - `type Plan struct { Main string; Steps []Step }`
  - `func BuildPlan(main string, others []string, files []FileState, same func(a, b string) bool) Plan`
  - `func CheckOutOfSync(main string, others []string, files []FileState, same func(a, b string) bool) []string`: report lines in the exact wording of `git-wt-sync:337-362`.
- `same(a,b)` is the content comparison (byte compare, called only when the rules need it).

Classification (parity with `git-wt-sync:264-327`). Per file: `rec.ok = Recorded.Mtime != 0`; `mainModified = Main.Exists && (!rec.ok || Main.Mtime != rec.Mtime || Main.Size != rec.Size)`; `mainDeleted = !Main.Exists && rec.ok`. Each other worktree: present → **modifying** if `!rec.ok`, or `mtime >= rec.mtime && (mtime > rec.mtime || size != rec.size)`, or `mtime >= rec.mtime && Main.Exists && !mainModified && !same(wt, main)`; otherwise **untouched**. Absent → **deleting** if `rec.ok`, else **missing**.

Rules, first match wins. Conflict file names are `<Rel>.devex-conflict-<wtfolder>` (and `-main` suffix in R2). Option labels and questions copy `prompt_conflict`/`prompt_multi_wt_conflict`/`prompt_yn` wording; default option is "Resolve Later" (index 2).
- **R1 mainDeleted:** no modifying → Delete from each untouched. Else per modifying wt: Choice {keep main → Delete wt; keep wt → Copy wt→main + wt→every other except it; later → Copy wt→main conflict file}.
- **R2 deleting non-empty:** if mainModified → per deleting wt: Choice {restore main→wt; delete from main and every worktree except it; later → Copy main→main conflict file `-main`}. Else per deleting wt (in order): YesNo "File <f> was removed in <wt>. Remove it from main as well?" default No; yes → Delete from main and all worktrees and **stop processing remaining deleting wts**; no → `Then` YesNo "Restore File <f> in <wt>?" default Yes → Copy main→wt.
- **R3 not in main (new file):** 1 modifying → Copy to main, fan out to all others. >1 → all `same` → Copy first to main, fan out to all non-main; else Choice over the first two modifying wts {keep A; keep B; later → each modifying wt Copy to main conflict file}.
- **R4 in main, mainModified:** no modifying → Copy main→all others. Else per modifying: `same(main, wt)` → Copy main→wt silently (aligns mtime); else Choice {main→wt; wt→main then main→all others except wt; later → wt→main conflict file}.
- **R5 in main, unmodified:** 1 modifying → wt→main, fan out. >1 → as R3's >1 branch. 0 modifying and missing non-empty → Copy main→each missing.

- [ ] **Step 1: Write failing tests** porting `tests/test_sync.sh` scenarios as table cases on in-memory `FileState`s (no disk): `TestPlanAdditions` (TEST 1: main-only file fans out; feat-only file → main + feat2), `TestPlanModifications` (TEST 2 both directions), `TestPlanRemovedInWT` (TEST 3 A and B: Step with YesNo default No, `Then` restore default Yes; yes-branch deletes from all), `TestPlanRemovedInMain` (TEST 4), `TestPlanConflictMainAndWT` (TEST 5: Choice default index 2 whose Ops write `settings.json.devex-conflict-feat`), `TestPlanConflictMultiWT` (TEST 6), `TestPlanIdenticalAdditions`, `TestPlanMissingInNewWorktree`. Plus `TestFindConflictMarkers` on disk (TEST 5b: marker inside a copy path found; TEST 5c: marker inside a symlink-only path ignored; marker in `node_modules/` outside copy paths ignored). `TestCheckOutOfSyncWording` asserts `Out of sync: <f>` and `    - Modified in: feat`.
- [ ] **Step 2: Run** `go test ./internal/wtsync/ -v`, expect FAIL.
- [ ] **Step 3: Implement.** Keep one fan-out helper `fanOut(src Loc, all []string, except ...string) []Op` so no rule repeats copy loops.
- [ ] **Step 4: Run** `go test ./internal/wtsync/ -v`, expect PASS.
- [ ] **Step 5: Commit** `internal/wtsync`, `feat(sync): scan and plan with parity rules`.

### Task 6: Sync apply and `git wt sync`

**Files:**
- Create: `internal/wtsync/apply.go`, `apply_test.go`, `internal/cli/wt_sync.go`, `internal/cli/wt_sync_test.go`

**Interfaces:**
- Consumes: Task 3 `gitx.Open`, `Worktrees`; Task 2 `LoadRepo`; Task 4 `Prompter`, `Table`; Task 5 everything.
- Produces:
  - `func Resolve(p Plan, pr ui.Prompter) []Op`: walks Steps in order, asking Choices; returns the flat op list.
  - `func Apply(ops []Op, backupRoot string) error`: per op, if `Dst` exists → copy it to `backupRoot/<base(Dst.WT)>/<Dst.Rel>` first; Copy = mkdir parents, copy bytes, `Chmod` + `Chtimes` from source; Delete = remove file. Stops at first error, wrapping it with the op and `backupRoot`.
  - `func PruneEmptyDirs(worktrees, copyPaths []string)`
  - `func Session(repo *gitx.Repo, cfg config.RepoConfig) (*SyncSession, error)` with `Main string; Others []string; Files []FileState; Markers []string` (main worktree = folder named `MainWorktreeName`, else `<Root>/<MainWorktreeName>`, else first non-bare, parity `git-wt-sync:66-95`).
  - `func (s *SyncSession) InSync() (bool, []string)` (used by rm/clean in-process).
  - CLI `git wt sync [--check|-c] [--dry-run] [--yes|-y]`. Flow: no copy paths → yellow "Nothing to sync" exit 0; markers → print the blocked message and files, exit 1; `--check` → `CheckOutOfSync`, exit 1 if any; build plan; print plan (one line per Step summary, conflicts marked); `--dry-run` → exit 0; TTY and not `--yes` → Confirm "Apply these changes?" default yes; prompter = TTY → huh, non-TTY → `DefaultPrompter`; `Resolve` → `Apply` → `PruneEmptyDirs` → rebuild manifest from main's files under copy paths → "✓ Synchronization complete!" plus backup dir if any backups were written. On Apply error: print error, **do not write the manifest**, exit 1.

- [ ] **Step 1: Write failing tests:** `TestApplyBacksUpBeforeOverwrite` (dst exists → backup file has old content; dst has src content and src mtime), `TestApplyStopsOnError` (second op's dst dir read-only → error mentions backup root; third op not applied), `TestSyncEndToEnd` (fixture with `copies.paths = .vscode, my dir/conf.json` (space), main + feat + feat2; run the CLI with non-TTY stdin: file added in feat lands in main and feat2; manifest has `my dir/conf.json|…|…`), `TestSyncFailureKeepsManifest` (make feat2's `.vscode` read-only, run sync: exit 1, manifest bytes unchanged), `TestSyncCheckExitCodes` (in sync → 0 + `✓ Everything is in sync.`; modify in feat → 1 + `Out of sync: .vscode/settings.json`), `TestSyncDryRunWritesNothing`.
- [ ] **Step 2: Run** `go test ./internal/wtsync/ ./internal/cli/ -run 'Apply|Sync' -v`, expect FAIL.
- [ ] **Step 3: Implement.** Backup timestamp format `2006-01-02-150405`.
- [ ] **Step 4: Run** same command, expect PASS. Also run `bash tests/test_sync.sh` to confirm the Bash version still passes untouched.
- [ ] **Step 5: Commit** `internal/wtsync internal/cli`, `feat(sync): apply with backups, git wt sync command`.

### Task 7: Safety rules and `git wt rm`

**Files:**
- Create: `internal/wt/safety.go`, `internal/wt/rm.go`, `internal/wt/wt_test.go`, `internal/cli/wt_rm.go`

**Interfaces:**
- Consumes: Tasks 2, 3, 6 (`SyncSession.InSync`).
- Produces:
  - `type Safety struct { Uncommitted []string; Unpushed int; Merged, UpstreamGone, HasUpstream bool }` and `func (s Safety) Safe() bool` = `len(Uncommitted)==0 && (Merged || UpstreamGone || Unpushed==0)`.
  - `func Assess(repo *gitx.Repo, wt gitx.Worktree, merged map[string]bool, remoteHeads map[string]map[string]bool) (Safety, error)`: `UpstreamGone` = has upstream and branch absent from that remote's heads; `remoteHeads` keyed by remote name (one `RemoteHeads` call per remote, made by the caller).
  - `func Inside(cwd, path string) bool`: `cwd == path || strings.HasPrefix(cwd, path+string(os.PathSeparator))` after `filepath.EvalSymlinks` on both.
  - `func FindTarget(wts []gitx.Worktree, repo *gitx.Repo, target string) (wt *gitx.Worktree, branch string)`: parity with `git-wt-rm:62-91` (path ends with `/target` or branch equals target; if no worktree and no local branch named target, first branch matching `target*`).
  - `func Remove(repo *gitx.Repo, cfg config.RepoConfig, wt gitx.Worktree, branch string, out io.Writer) error`: remove symlinks listed in `SymlinkPaths` (only if they are symlinks), `git worktree remove --force <path>`, `git branch -D <branch>` if it exists. Shared by rm and clean.
  - CLI `git wt rm [-f|--force] <target>`: errors and wording for no target / too many args from `git-wt-rm:30-43`. Refuse (exit 1, no changes) when: cwd `Inside` target; target is the main worktree or branch == base; not `--force` and sync not in sync (message: `Cannot remove worktree while files are not in sync.` + `git wt sync` hint); not `--force` and `!Safe()` (list uncommitted files and unpushed count, hint `--force`).

- [ ] **Step 1: Write failing tests:** `TestInsidePrefixCollision` (`/r/DEVX-12` not inside `/r/DEVX-1`; `/r/DEVX-1/sub` inside), `TestSafeMatrix` (dirty tree → unsafe; never-pushed with 1 local commit → unsafe; never-pushed with 0 commits → safe; upstream gone → safe; merged → safe; upstream exists and ahead 1 → unsafe), and CLI tests porting `tests/test_wt_deletion_sync_checks.sh` TEST 4-9 (`rm` aborts when out of sync; `-f`, `--force`, `target -f` bypass; no target fails; too many args fails), plus `TestRmRefusesDirty` and `TestRmRefusesMainAndBase`.
- [ ] **Step 2: Run** `go test ./internal/wt/ ./internal/cli/ -run 'Inside|Safe|Rm' -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** same, expect PASS.
- [ ] **Step 5: Commit** `internal/wt internal/cli`, `feat(wt): safety rules and git wt rm`.

### Task 8: `git wt clean`

**Files:**
- Create: `internal/wt/clean.go`, `internal/cli/wt_clean.go`, `internal/wt/clean_test.go`

**Interfaces:**
- Consumes: Task 7 `Assess`, `Inside`, `Remove`; Task 6 `InSync`; Task 4 `Prompter.MultiSelect`.
- Produces:
  - `type Candidate struct { WT gitx.Worktree; Reason string; Safety wt.Safety }`
  - `func FindStale(repo, cfg) ([]Candidate, error)`: skip bare, detached, base branch, main worktree. Stale if merged (`Merged into <base>`), or upstream gone (`Remote branch deleted (on <remote>)`), or no upstream at all (`No remote branch`). One `RemoteHeads` call per distinct remote.
  - CLI `git wt clean [--yes|-y]`: sync check first (same abort message as Bash `git-wt-clean:41-45`); none → `✓ No stale worktrees found.`; else MultiSelect with each candidate preselected **only if `Safe()`**, unsafe rows labelled with what would be lost; `--yes` takes the preselection; skip and report any candidate the cwd is `Inside`; `Remove` each selected; `✓ Cleanup complete!`.

- [ ] **Step 1: Write failing tests:** port `tests/test_clean.sh` (merged branch and remote-deleted branch both removed with piped `y\n`; active branch kept), `TestCleanSkipsDirtyByDefault` (merged but with an uncommitted file: piped `y` leaves it), `TestCleanNeverPushedWithCommits` (listed as `No remote branch`, not preselected), `TestCleanAbortsOutOfSync` (port TEST 3 of the deletion-checks script), `TestCleanEOFDeletesNothing`.
- [ ] **Step 2: Run** `go test ./internal/wt/ ./internal/cli/ -run Clean -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** same, expect PASS.
- [ ] **Step 5: Commit** `internal/wt internal/cli`, `feat(wt): git wt clean with dirty refusal`.

### Task 9: `git wt add` and `git wt clone`

**Files:**
- Create: `internal/wt/add.go`, `internal/wt/clone.go`, `internal/cli/wt_add.go`, `internal/cli/wt_clone.go`, `internal/wt/add_test.go`

**Interfaces:**
- Produces:
  - `func Add(repo *gitx.Repo, cfg config.RepoConfig, branch, customFolder string, out io.Writer) (path string, err error)`: parity with `git-wt-add:83-155` (naming, existing local-or-`origin/` branch → checkout, else `-b` from base; symlink then copy steps with `[1/3]..[3/3]` output; final `cd "<path>"` hint).
  - `func Clone(url, cwd string, out io.Writer) error`: parity with `git-wt-clone` (non-empty target dir → error; bare clone into `<name>/.git`, fetch refspec, default branch worktree, `.devex.conf` generated with the exact template in that script, which sets `copies.paths = .claude,.kiro,.vscode` and empty symlinks).
  - CLI `git wt add [branch] [folder]`: no branch + TTY → filterable huh Select of remote branches without a worktree (replaces fzf); no branch + non-TTY → the Bash usage error, exit 1.

- [ ] **Step 1: Write failing tests:** `TestAddNewBranchFromBase` (folder `DEVX-9`, symlinks `.claude` points into main, copy path copied), `TestAddExistingRemoteBranch`, `TestAddCustomRequiresFolder`, `TestCloneLayout` (from fixture origin: `.git` bare, `main/` worktree, `.devex.conf` content equals template).
- [ ] **Step 2: Run** `go test ./internal/wt/ -run 'Add|Clone' -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** same, expect PASS.
- [ ] **Step 5: Commit** `internal/wt internal/cli`, `feat(wt): add and clone`.

### Task 10: Status collection and plain `git wt status`

**Files:**
- Create: `internal/wt/status.go`, `internal/cli/wt_status.go`, `internal/wt/status_test.go`

**Interfaces:**
- Produces:
  - `type Row struct { WT gitx.Worktree; Folder string; Changed []string; Ahead, Behind int; HasUpstream, Merged bool; Modified time.Time; Err error }`
  - `func Collect(repo *gitx.Repo, cfg config.RepoConfig) ([]Row, error)`: skips bare; one goroutine per worktree; rows in `Worktrees()` order.
  - `func RenderTable(rows []Row, width int) string`: columns Worktree, Branch (Flex), Changes (`● N files` red / `○ Clean` green), Remote (`↑a ↓b`, `Synced`, `No upstream`, `N/A` detached), Merged, Last edited (`2006-01-02 15:04` when width ≥ 120, else relative `5m ago`), then the summary lines of `git-wt-status:138-145`.
  - CLI: `git wt status`, and `git wt list` with no args. Non-TTY → `RenderTable` to stdout. TTY → Task 12 app (until Task 12 lands, TTY also prints the table).

- [ ] **Step 1: Write failing tests:** port `tests/test_status.sh` (uncommitted file → `● 1 files`), `TestStatusDetachedAndAhead`, `TestRenderTableWidths` (60/160: every line ≤ width).
- [ ] **Step 2: Run** `go test ./internal/wt/ -run Status -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** same, expect PASS.
- [ ] **Step 5: Commit** `internal/wt internal/cli`, `feat(wt): status collection and table`.

### Task 11: `dx ctx`

**Files:**
- Create: `internal/ctx/ctx.go`, `internal/ctx/ctx_test.go`, `internal/cli/dx_ctx.go`

**Interfaces:**
- Produces:
  - `func Path(commonDir, branch string) string` (`/` → `__`), `func EnsureFile(path, branch string) error` (template from `dx-ctx-helper.sh:38-44`).
  - `type Todo struct { Index, Line int; Done bool; Text string }`
  - `func ParseTodos(content string) []Todo`: only lines under a `## Todos` heading (case-insensitive) until the next `#`/`##` heading; regex `^\s*- \[[ xX]\]\s+(.*)`; 1-based Index.
  - `func AddTodo(content, task string) string` (after the last todo in the section; append `## Todos` if absent), `func SetDone(content string, idx int, done bool) (string, error)`, `func RemoveTodo(content string, idx int) (string, error)`, `func ClearDone(content string) (string, int)`, `func Notes(content string) []string`.
  - `func Orphans(commonDir string, branchExists func(string) bool) []string`.
  - CLI verbs and aliases exactly as `dx-ctx-tools/dx-ctx` and `dx-ctx-todo`: `show` (default), `edit`, `add`, `done|check`, `undo|uncheck`, `rm|delete`, `clear`, `clean`, `todo <verb>`, `help`. Editor order `$EDITOR`, nano, vim, vi. `clean` confirms via Prompter (EOF → no). Detached HEAD → `Error: Not currently on a branch (detached HEAD)`, exit 1.

- [ ] **Step 1: Write failing tests:** port `tests/test_dx_ctx.sh` assertions as unit tests on strings (add/done/undo/rm/clear indices, case-insensitive heading, todos outside the section ignored), `TestCtxPathSlashes` (`feature/a` → `feature__a.md`), `TestOrphans`.
- [ ] **Step 2: Run** `go test ./internal/ctx/ -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** same, expect PASS.
- [ ] **Step 5: Commit** `internal/ctx internal/cli`, `feat(ctx): dx ctx commands`.

### Task 12: `git wt` full-screen app

**Files:**
- Create: `internal/wtapp/app.go`, `internal/wtapp/app_test.go`
- Modify: `internal/cli/wt_status.go` (TTY → app)

**Interfaces:**
- Consumes: Task 10 `Collect`, `Row`; Task 11 `Path`, `ParseTodos`, `SetDone`; Task 7 `Safe`.
- Produces: `func New(repo *gitx.Repo, cfg config.RepoConfig, self string) tea.Model` (`self` = path of own executable for exec actions); `func Run(m tea.Model) error` (alt screen).
- Behaviour (spec "Decisions", Option B minus Enter-to-cd):
  - Layout: header (repo name + counts), left list (folder, changes, remote, merged), right details (branch, path, remote, changed files, ctx todos), footer key help. Split ~40/60 of `WindowSizeMsg.Width`; below 60 columns or 12 rows render only `Terminal too small (need 60×12)`.
  - Keys: `↑/↓/j/k` move; `tab` focus todos pane; `space` toggles selected todo (writes file via `SetDone`); `/` filter by folder or branch substring; `s` → `tea.ExecProcess(self git-wt sync)` then refresh; `d` → in-app confirm modal listing what would be lost (from `Assess`), then `tea.ExecProcess(self git-wt rm <folder>)`; `c` → `tea.ExecProcess(self git-wt clean)`; `r` refresh; `q`/`ctrl+c` quit. Enter does nothing.
  - Data loads asynchronously (`tea.Cmd` running `Collect`) with a "Loading…" placeholder.

- [ ] **Step 1: Write failing tests** driving the model with messages (no TTY): `TestAppResize` (send `WindowSizeMsg{80,24}` then `{200,50}`: `View()` lines ≤ width each time), `TestAppTooSmall` (`{50,10}` → contains `Terminal too small`), `TestAppToggleTodo` (fixture worktree with ctx file `- [ ] a`; tab, space → file contains `- [x] a`), `TestAppFilter` (`/` + `DEVX-2` → only that row), `TestAppDeleteShowsConfirm` (`d` on dirty worktree → View contains the uncommitted file name).
- [ ] **Step 2: Run** `go test ./internal/wtapp/ -v`, expect FAIL.
- [ ] **Step 3: Implement**, then wire TTY branch in `wt_status.go`.
- [ ] **Step 4: Run** `go test ./internal/wtapp/ -v`, expect PASS. Manual: `go build -o dx ./cmd/dx && ln -sf "$PWD/dx" git-wt`, then in a scratch bare repo run `PATH="$PWD:$PATH" git wt` and resize the window.
- [ ] **Step 5: Commit** `internal/wtapp internal/cli`, `feat(wt): full-screen git wt app`.

### Task 13: `dx nb`

**Files:**
- Create: `internal/nb/nb.go`, `internal/nb/nb_test.go`, `internal/cli/dx_nb.go`

**Interfaces:**
- Produces:
  - `type Info struct { Path string; Size int64; Cells, Code, Markdown int; HasOutput bool; Modified time.Time; Err error }`, `func Inspect(path string) Info` (parse JSON with `encoding/json`; no `jq` needed), `func Find(root string) []string` (`*.ipynb`, skipping any path segment starting with `.`).
  - CLI `dx nb strip|diff|kernel|list`: `strip`/`diff`/`kernel` keep the exact external-tool fallbacks and messages of `dx-nb-strip`, `dx-nb-diff`, `dx-nb-kernel` (exec `nbstripout` → `jupyter nbconvert --clear-output --inplace`; `nbdiff` → `git diff`; `.venv/bin/python -m ipykernel install --user --name <dir> --display-name "DevEx: <dir>"`). `list` renders `ui.Table` (Path Flex, Size, Cells(C/M), Status `● Dirty`/`○ Stripped`/`Error (JSON)`, Last Modified) and the summary of `dx-nb-list:107-114`. No-arg/unknown → help, exit 1.

- [ ] **Step 1: Write failing tests:** `TestInspect` (fixture notebook 3 cells, 2 code with one output → Cells 3, Code 2, HasOutput), `TestInspectBadJSON`, `TestFindSkipsHidden`, and port the dispatch assertions of `tests/test_dx_nb_*.sh` using a fake `nbstripout`/`nbdiff` on `PATH` in `t.TempDir()`.
- [ ] **Step 2: Run** `go test ./internal/nb/ ./internal/cli/ -run Nb -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** same, expect PASS.
- [ ] **Step 5: Commit** `internal/nb internal/cli`, `feat(nb): dx nb commands`.

### Task 14: `dx skills` commands

**Files:**
- Create: `internal/skills/scan.go`, `plan.go`, `apply.go`, `ops.go`, `skills_test.go`, `internal/cli/dx_skills.go`

**Interfaces:**
- Consumes: Task 2 `SkillsConfig`; Task 4.
- Produces:
  - `type Copy struct { Location string /* "master" or tool name */; Dir string; Mtime int64 }`, `type Skill struct { Name string; Copies []Copy; Newest string; Description string }`
  - `func Scan(cfg config.SkillsConfig) ([]Skill, error)`: union of child dirs containing `SKILL.md` or `skill.md` across master + tools, excluding `.backups`/`backups` and `IsIgnored`; mtime = newest file mtime under the dir (dir's own mtime if empty); newest wins with master checked first and ties keep the earlier location (parity); description from `description:` in frontmatter of the newest copy, else `No description provided`.
  - `type Update struct { Skill, From, FromDir, To, ToDir string }`, `func Plan(cfg, skills []Skill, same func(a, b string) bool) []Update`: parity with `dx-skills-sync:13-62` (dest missing or recursive contents differ; respects per-tool SyncOnly).
  - `func ApplyUpdates(ups []Update, master string, now time.Time) (backupRoot string, err error)`: back up existing dest to `<master>/.backups/<ts>/<To>/<skill>`, then mirror `FromDir` → `ToDir` (copy all, delete extras; preserve mode/mtime), replacing `rsync -a --delete`.
  - `func Remove(cfg, name string, now time.Time) (backupRoot string, err error)` (backup to `.backups/<ts>/deleted/<location>/<skill>` then delete each copy), `func Diff(dirA, dirB string) (string, error)` (runs `diff -ruN`, exit 1 is not an error), `func CreateTemplate(dir, name string) error` (exact template of `dx-skills-edit`).
  - CLI `dx skills list|sync [--yes]|diff <s>|edit <s>|rm <s>`; no args: TTY → Task 15 app, else `list`. `list` prints `Master Directory:` / `Active Agents:` then `ui.Table` (Skill, Description Flex, Newest, Last Modified). Warnings from `cfg.Warning` go to stderr once. Sync confirm: `--yes`, TTY Confirm, else LinePrompter (EOF → cancelled, exit 1, message `Synchronization cancelled.`).

- [ ] **Step 1: Write failing tests** porting `tests/test_dx_skills.sh` (list shows skills and newest location; sync copies newest to master and other tools with a backup; `sync_only` per tool respected; ignore respected; diff output; rm backs up then deletes), plus `TestApplyMirrorsDeletes` (extra file in dest removed) and `TestSyncEOFCancels`.
- [ ] **Step 2: Run** `go test ./internal/skills/ ./internal/cli/ -run Skills -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** same, expect PASS.
- [ ] **Step 5: Commit** `internal/skills internal/cli`, `feat(skills): dx skills commands`.

### Task 15: `dx skills` grid app

**Files:**
- Create: `internal/skillsapp/app.go`, `internal/skillsapp/app_test.go`
- Modify: `internal/cli/dx_skills.go` (TTY no-arg → app)

**Interfaces:**
- Consumes: Task 14 `Scan`, `Plan`, `ApplyUpdates`, `Diff`.
- Produces: `func New(cfg config.SkillsConfig) tea.Model`.
- Behaviour: grid rows = skills, columns = master + each tool; cell shows `★` newest, `≠` differs from newest, `✓` same, `·` absent. Right pane: `Diff` of selected skill's newest vs first differing copy, scrollable. `space` ticks/unticks the selected skill's pending updates (all pending ticked initially); `a` apply ticked (runs `ApplyUpdates`, shows backup dir, rescans); `↑/↓`, `/` filter, `q` quit. Same 60×12 minimum as Task 12. No source override (spec).

- [ ] **Step 1: Write failing tests:** `TestGridMarks` (skill newer in claude: claude `★`, master `≠`, kiro `·`), `TestGridUntickSkipsApply` (untick one of two skills, `a` → only the ticked one updated on disk), `TestGridResize` (lines ≤ width at 80 and 200), `TestGridTooSmall`.
- [ ] **Step 2: Run** `go test ./internal/skillsapp/ -v`, expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** same, expect PASS.
- [ ] **Step 5: Commit** `internal/skillsapp internal/cli`, `feat(skills): full-screen skills grid`.

### Task 16: Release workflow, installer, completions

**Files:**
- Create: `.github/workflows/release.yml`
- Modify: `install.sh`, `uninstall.sh`, `internal/cli/dx.go` (`dx completion bash|zsh` via Cobra; `dx --version` from `-ldflags "-X main.version=<tag>"`)

**Interfaces:**
- Release asset names: `dx_<goos>_<goarch>.tar.gz`, each containing a single `dx`. Download URL used by `install.sh`: `https://github.com/mukeshmk/devex-manager/releases/latest/download/dx_${os}_${arch}.tar.gz` (`os` from `uname -s` lowercased, `arch` `x86_64`→`amd64`, `arm64|aarch64`→`arm64`).

- [ ] **Step 1: Write `release.yml`:** on `push: tags: ['v*']`; job `test` (`actions/setup-go` with `go-version-file: go.mod`, `go test ./...`); job `build` needs test, matrix of the four platforms, `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${GITHUB_REF_NAME}" -o dx ./cmd/dx`, tar, upload artifact; job `release` downloads artifacts and publishes with `softprops/action-gh-release` (`permissions: contents: write`).
- [ ] **Step 2: Rewrite `install.sh`:** keep its colours, remote/local detection, git-aliases and auto-venv prompts, shell-rc block helpers and `~/.devex.conf` template (lines 1-60 structure, 138-270 as-is) with these changes: local clone with `go` on `PATH` → `go build -o "$INSTALL_DIR/dx" ./cmd/dx`; otherwise download+extract the release asset to `$INSTALL_DIR/dx`; `ln -sf "$INSTALL_DIR/dx" "$INSTALL_DIR/git-wt"`; remove old scripts: every file listed in the current fetch list under `$WT_TOOLS_DIR` except `git-wt-completion.sh` and `auto-venv.sh`, and the dirs `$CTX_TOOLS_DIR`, `$DX_SKILLS_TOOLS_DIR`, `$DX_NB_TOOLS_DIR`; drop the dx-ctx install prompt (always included now); write `"$INSTALL_DIR/dx" completion bash > "$WT_TOOLS_DIR/dx-completion.sh"` so the existing rc `source` line keeps working under bash and zsh (`bashcompinit`). `git-wt-completion.sh` is still copied from the repo (git's completion needs `_git_wt`). This is a deliberate narrowing of the spec's "generated completions replace both" call.
- [ ] **Step 3: Update `uninstall.sh`:** remove `$INSTALL_DIR/dx` and the `git-wt` symlink plus the existing dir removals; keep the alias and rc-block handling unchanged.
- [ ] **Step 4: Verify:** `bash -n install.sh uninstall.sh`; `HOME=$(mktemp -d) SHELL=/bin/zsh bash install.sh </dev/null` from the repo root: `$HOME/.local/bin/dx --version` runs, `$HOME/.local/bin/git-wt` is a symlink, `$HOME/.local/bin/git-wt-tools/dx-completion.sh` is non-empty, no `dx-skills-tools` dir. `actionlint .github/workflows/release.yml` if available, else skip and say so.
- [ ] **Step 5: Commit** `.github install.sh uninstall.sh internal/cli`, `feat: release workflow and binary installer`.

### Task 17: Remove Bash tools, update docs

**Files:**
- Delete: `git-wt`, `dx`, `dx-completion.sh`, `git-wt-tools/`, `dx-ctx-tools/`, `dx-skills-tools/`, `dx-nb-tools/`, `tests/*.sh` except `tests/test_auto_venv_prompt.sh` (and `tests/test_lxc.sh`, which tests a non-existent `./lxc`).
- Keep: `auto-venv/`, `git-aliases/`, `git-wt-completion.sh`.
- Modify: `README.md`, `CLAUDE.md`, `workflows.md` (Go build/test instructions, `go test ./...`, new layout, sync backups, dirty refusal, `--yes`/`--dry-run`, full-screen keys).

- [ ] **Step 1: Confirm parity** `go test ./...` passes and every deleted `tests/*.sh` scenario has a Go test named in Tasks 5-14 (grep the test names listed there).
- [ ] **Step 2: Delete files and update docs.**
- [ ] **Step 3: Verify** `go vet ./... && go test ./...` PASS; `bash tests/test_auto_venv_prompt.sh` PASS; `grep -rn 'git-wt-tools/git-wt-\|dx-skills-tools\|dx-nb-tools\|dx-ctx-tools' README.md CLAUDE.md workflows.md install.sh` only matches the cleanup lines in `install.sh`.
- [ ] **Step 4: Commit** all changes, `refactor!: replace Bash tools with Go binary (v2)`.
- [ ] **Step 5: Hand off:** tell the owner the exact commands to run themselves: `git push origin <branch>`, merge, then `git tag v2.0.0 && git push origin v2.0.0`. Do not push.
