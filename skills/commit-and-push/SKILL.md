---
name: commit-and-push
description: Commit current Git work with meaningful boundaries and push safely. Use when the user says `commit`, `commit & push`, `ship it`, `提交`, `提交并推送`, or asks Codex to finish the current repo work by creating commits and pushing. Handles repo/PR inspection, Conventional Commits, explicit remote/upstream checks, preserving unrelated dirty changes, focused staging, safe retries, and concrete blocker reporting when no verified remote exists.
---

# Commit and Push

## Outcome

Commit the intended current work in focused commit(s), push to the verified upstream/remote, and report the result. If pushing cannot be done safely, stop with a concrete verified blocker instead of guessing.

## Operating rules

- Use standard `git` plus GitHub CLI (`gh`) commands for PR/review operations when relevant.
- Inspect repo state before choosing commit messages, PR titles, PR bodies, or review replies.
- Prefer repository conventions, existing templates, and current PR context when present.
- Split unrelated changes into separate focused commits.
- Use Conventional Commits for new commits: `type(scope): summary` or `type: summary`.
- Allowed common types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `perf`, `build`, `ci`, `style`.
- Preserve unrelated dirty worktree changes. Never use `git commit -a`.
- Stage only intended files or hunks with `git add <target_files>` or `git add -p`.
- Execute non-interactively and continue until the requested outcome is complete or a concrete blocker is verified.
- For multi-line GitHub CLI content, write to a temp file or heredoc and pass `--body-file`; do not inline large multi-line `--body` strings.
- If a command fails, resolve and retry when recovery is safe. Do not retry destructive operations blindly.

## Pre-flight checks

Run these before committing or pushing:

1. `git status --short --branch`
   - Identify current branch, staged files, unstaged files, and untracked files.
   - Mark unrelated dirty/untracked changes that must be preserved.
2. `git remote -v`
   - If no remote is configured, do not invent one.
3. `git rev-parse --abbrev-ref --symbolic-full-name @{upstream}`
   - Use this to decide whether plain `git push` is safe.
4. PR context when relevant:
   - Use `gh pr status`, `gh pr view`, or branch-specific PR lookup if GitHub CLI is available and the task touches PR/review text.
5. Inspect the diff:
   - Use `git diff`, `git diff --stat`, `git diff --cached`, and targeted file views as needed.

## Commit workflow

1. Build commit boundaries from the actual diff, not from a guessed task title.
2. For each commit:
   - choose the smallest coherent set of files/hunks;
   - run the lightest credible validation for that change when practical;
   - run `git diff --check` or the relevant staged equivalent before committing;
   - stage explicitly;
   - create a concise imperative Conventional Commit message grounded in the staged diff.
3. After each commit, re-check `git status --short --branch` to avoid accidental carry-over.
4. Leave unrelated pre-existing dirty/untracked changes untouched and remember to report them.

## Push workflow

- If upstream exists, run `git push`.
- If upstream is absent but a verified `origin` remote exists, run `git push --set-upstream origin HEAD`.
- If no remote exists, or the target remote cannot be safely inferred from current repo evidence, do not push. Report the blocker and any commits already created.
- If push fails for a recoverable reason, diagnose, fix, and retry. Examples: stale branch needing safe rebase, authentication already available via `gh`, or transient network failure.
- Do not force-push unless the user explicitly asked or the repo/PR workflow clearly requires it and you can use a safe form such as `--force-with-lease`.

## Done criteria

- Intended changes are committed in focused commit(s).
- The branch is pushed to the verified remote/upstream, with upstream configured if needed.
- The working tree is clean, or only pre-existing unrelated dirty/untracked changes remain and are explicitly reported.

## Final response

Briefly report:

- commit hash(es) and subject(s);
- push target and whether upstream was configured;
- validation performed;
- any unrelated dirty/untracked changes left in place;
- any blocker if the push could not be completed safely.
