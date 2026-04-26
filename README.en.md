# my-codex

[中文](./README.md)

`my-codex` is a Codex workflow configuration designed for long-running, high-context engineering work.

The goal is not to make the model more talkative. The goal is to make the agent more reliable when a task is long, ambiguous, multi-step, and expensive to get wrong.

## Why this exists

Default chat-style agents tend to fail in a few predictable ways:

- state drifts in long sessions
- complex decisions are made by a single model with no review
- the agent stops too early and waits for another “continue”
- open-ended work either converges too early or keeps expanding without direction

This workflow is built around those problems.

## Core ideas

### 1. Plan files as persistent state

The agent is expected to maintain plan files and execution records inside the workspace.

The point is not just to keep a TODO list. The point is to move critical task state out of the live conversation and into files that survive compaction:

- current objective
- verified facts
- active milestone
- next action
- blockers and risks

After compaction, the agent can reload that state from disk and continue from the same execution line instead of reconstructing everything from memory.

### 2. GPT for execution, Gemini 3 for review

This workflow separates execution from review.

- GPT is used for execution:
  - reading code
  - editing files
  - running commands
  - pushing implementation forward
- Gemini 3 is used as reviewer:
  - reassessing the current framing
  - surfacing hidden risks
  - comparing technical options
  - reviewing difficult decisions before execution locks in

The result is a more reliable split: execution stays steady, while major decisions get a second opinion from a model with stronger breadth and better top-down judgment.

### 3. Stop hook keeps milestones moving until the work is actually done

This workflow includes a `Stop` hook to prevent the agent from stopping too early.

As long as the current plan still has unfinished milestones, the agent should either:

- keep moving forward, or
- record a real blocker explicitly

That changes the default behavior from “finish one step and stop” to “keep going until the plan is actually closed”.

### 4. Reviewer loop for open-ended work

Some of the most important engineering tasks are not linear:

- performance work
- ambiguous root-cause debugging
- architectural cleanup
- exploratory technical research

For those tasks, this workflow supports a reviewer loop:

1. collect evidence
2. ask the reviewer what is worth doing next
3. execute the next step
4. loop back with updated evidence

This helps exploratory work converge instead of stalling or drifting.

## What this is good for

This workflow is a good fit for:

- long-running tasks
- complex debugging
- multi-stage refactors
- technical decisions that benefit from review
- open-ended tasks that may run for hours

If you want a lightweight assistant for quick one-off chats, this is probably not the right configuration.

## Getting started

### Pull the latest workflow to your local Codex

#### macOS / Linux

```bash
tmp_dir="$(mktemp -d)"
os="$(uname -s)"
arch="$(uname -m)"
case "$os/$arch" in
  Darwin/arm64) asset="my-codex_darwin_arm64.tar.gz" ;;
  Darwin/x86_64) asset="my-codex_darwin_amd64.tar.gz" ;;
  Linux/x86_64) asset="my-codex_linux_amd64.tar.gz" ;;
  Linux/aarch64|Linux/arm64) asset="my-codex_linux_arm64.tar.gz" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac
curl -fsSL -o "$tmp_dir/$asset" "https://github.com/faceair/my-codex/releases/latest/download/$asset"
tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
"$tmp_dir/my-codex" pull
rm -rf "$tmp_dir"
```

#### Windows PowerShell

```powershell
$TmpDir = Join-Path $env:TEMP ([guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $TmpDir | Out-Null
$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$Asset = "my-codex_windows_$Arch.zip"
Invoke-WebRequest -Uri "https://github.com/faceair/my-codex/releases/latest/download/$Asset" -OutFile (Join-Path $TmpDir $Asset)
Expand-Archive -Path (Join-Path $TmpDir $Asset) -DestinationPath $TmpDir -Force
& (Join-Path $TmpDir 'my-codex.exe') pull
Remove-Item -Recurse -Force $TmpDir
```

## License

[MIT](./LICENSE)
