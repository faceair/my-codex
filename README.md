# my-codex

用于在仓库与本地 `~/.codex` 之间同步配置：
- `pull_codex.sh`：从 GitHub 拉取到本地 `~/.codex`，并保留本地 `model_provider` 相关配置。
- `sync_codex.sh`：从本地 `~/.codex` 同步回当前仓库。

## 快速运行 pull_codex.sh（一行命令）

```bash
curl -fsSL https://raw.githubusercontent.com/faceair/my-codex/main/pull_codex.sh | bash
```

## 本地运行 sync_codex.sh

```bash
cd /Users/faceair/Developer/my-codex
./sync_codex.sh
```
