# my-codex

[English](./README.en.md)

`my-codex` 是一套面向复杂工程任务的 Codex 工作流配置。

它关注的不是“让模型多说一点”，而是让 agent 在长任务、复杂决策和探索性工作里，能够更稳定地持续推进并最终收口。

## Why this exists

默认的对话式 agent 在真实工程任务里常见几个问题：

- 会话一长，状态容易漂
- 复杂决策只有一个模型在拍板
- 任务做到一半就停下来等一句“继续”
- 开放性任务容易过早收敛，或者无限发散

这套配置就是围绕这些问题设计的。

## Core ideas

### 1. Plan file as persistent state

这套工作流要求 agent 自己维护计划文件和执行记录。

重点不在“列 TODO”，而在于把真正关键的任务状态从会话里拿出来，落到工作区文件里，例如：

- 当前目标
- 已验证事实
- active milestone
- 下一步动作
- blocker 和风险

这样即使会话发生 compaction，agent 也可以重新读取计划文件，继续沿着原来的执行线往下做。

对使用者来说，直接收益是：

- 长任务连续性更稳定
- 多小时任务不容易“失忆”
- agent 更像是在维护一个持续推进的工作现场

### 2. GPT execution + Gemini 3 review

这套配置把“执行”和“评审”拆成两个角色。

- GPT 负责执行：
  - 读代码
  - 改文件
  - 跑命令
  - 推进实现
- Gemini 3 负责 reviewer：
  - 重新审视判断
  - 识别隐藏风险
  - 比较技术路线
  - 给复杂任务做同行评审

这不是为了增加流程，而是为了让复杂任务少走弯路。

对使用者来说，这种分工尤其适合：

- 重构
- 复杂排障
- 架构调整
- 开放性技术决策

### 3. Stop hook drives milestones to completion

这套配置带有 `Stop` hook，用来限制“做了一步就停”的行为。

只要计划文件里还有未完成 milestone，agent 就不会轻易结束；它要么继续推进，要么明确记录 blocker。

这使得任务推进从“每一步都要人来催”变成：

- agent 沿 milestone 持续工作
- 做完再停
- 做不下去时明确说明为什么停

对使用者来说，这意味着这套工作流可以稳定支撑持续数小时的长任务，而不是一轮一轮地手动续命。

### 4. Reviewer loop for open-ended work

很多真正困难的工程任务并不是线性的，例如：

- 性能优化
- 模糊根因排查
- 架构找补
- 研究型或探索型任务

这类任务既不能太早下结论，也不能无限发散。

所以这套配置支持 reviewer loop：

1. 收集当前证据
2. 让 reviewer 判断下一步最值得做什么
3. 执行下一步
4. 再带着新证据回到 reviewer

这样可以持续推进探索性任务，并让它逐步收敛，而不是停在一个“看起来差不多”的答案上。

## What this is good for

这套配置更适合：

- 长任务
- 复杂 debug
- 多阶段代码改造
- 需要 reviewer 把关的技术决策
- 可以持续推进数小时的开放性任务

如果你只是想要一个轻量、随聊随停的通用助手，这套配置未必是最合适的。

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
