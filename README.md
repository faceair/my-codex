# my-codex

用于在仓库与本地 `~/.codex` 之间同步配置：
- `pull_codex.sh`：从 GitHub 拉取到本地 `~/.codex`，并保留本地 `model_provider` 相关配置。
- `sync_codex.sh`：从本地 `~/.codex` 同步回当前仓库。

当前会双向同步这些内容：
- `agents/`
- `prompts/`
- `AGENTS.md`
- `config.toml`（`pull_codex.sh` 会保留本地 `model_provider` 相关配置）

`sync_codex.sh` 在生成 commit message 时，会复用同步进仓库的 `prompts/smart-commit.md` 作为提示词，再由脚本自身执行 `git commit` 和 `git push`。

## 快速运行 pull_codex.sh（一行命令）

```bash
curl -fsSL https://raw.githubusercontent.com/faceair/my-codex/main/pull_codex.sh | bash
```

## 本地运行 sync_codex.sh

```bash
cd /Users/faceair/Developer/my-codex
./sync_codex.sh
```
