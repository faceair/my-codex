# my-codex

`my-codex` 现在是 Go 驱动的本地配置同步仓库：
- `my-codex sync`：把本地 `~/.codex` 的 repo-managed 内容同步回当前仓库，并自动 commit/push
- `my-codex pull`：把 release 内嵌的 repo-managed 内容下发到本地 `~/.codex`
- `codex-stop-guard`：独立 Stop hook 二进制，负责在仍有未完成 plan 项时拦停

## 当前同步边界

双向同步这些内容：
- `agents/`
- `prompts/`
- `instructions/`
- `hooks/`（目录级同步；现在用于承载 hook 二进制与清理遗留 Python hook 文件）
- `hooks.json`（会在 repo 中保持 macOS 风格命令；pull 到本地时按本地平台改写）
- `config.toml`（白名单同步，仅管理这些配置：`model_instructions_file`、`[features].codex_hooks`、`[agents.reviewer].config_file`）

不再同步：
- `AGENTS.md`
- 本地 `model_provider` / `projects` / 其他运行偏好

## 直接拉最新 release 并下发到本地

默认会使用 GitHub 上由 `main` 最新提交自动生成的滚动 release。

### macOS

```bash
tmp_dir="$(mktemp -d)"
arch="$(uname -m)"
case "$arch" in
  arm64) asset="my-codex_darwin_arm64.tar.gz" ;;
  x86_64) asset="my-codex_darwin_amd64.tar.gz" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac
curl -fsSL -o "$tmp_dir/$asset" "https://github.com/faceair/my-codex/releases/latest/download/$asset"
tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
"$tmp_dir/my-codex" pull
rm -rf "$tmp_dir"
```

### Windows PowerShell

```powershell
$TmpDir = Join-Path $env:TEMP ([guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $TmpDir | Out-Null
$Asset = "my-codex_windows_amd64.zip"
Invoke-WebRequest -Uri "https://github.com/faceair/my-codex/releases/latest/download/$Asset" -OutFile (Join-Path $TmpDir $Asset)
Expand-Archive -Path (Join-Path $TmpDir $Asset) -DestinationPath $TmpDir -Force
& (Join-Path $TmpDir 'my-codex.exe') pull
Remove-Item -Recurse -Force $TmpDir
```

`pull` 会：
- 把 `codex-stop-guard` 安装到 `~/.codex/hooks/`（Windows 为 `.exe`）
- 更新本地 `hooks.json` 指向正确的平台路径
- 不保留临时下载的 `my-codex`

## 安装 sync 工具到 `GOPATH/bin`

如果你需要长期在 repo 里执行 `sync`，推荐安装到 `GOPATH/bin`：

```bash
go install github.com/faceair/my-codex/cmd/my-codex@latest
```

安装后直接在 repo 里运行：

```bash
my-codex sync
```

仓库里的兼容壳也会遵循这个语义：
- `./sync_codex.sh`
- `sync_codex.cmd`

如果当前 `PATH` 里没有 `my-codex`，它们会先执行本仓库源码版：

```bash
go install ./cmd/my-codex
```

然后再从 `GOBIN` / `GOPATH/bin` 运行 `my-codex sync`。

如果你在当前仓库内开发，也可以：

```bash
go run ./cmd/my-codex sync
```

## 本地源码运行

```bash
go test ./...
go run ./cmd/my-codex pull
go run ./cmd/my-codex sync
```

如果从源码执行 `pull`，程序会优先：
- 使用与 `my-codex` 同目录的 `codex-stop-guard`
- 如果不存在，则尝试在当前 repo 内临时 `go build ./cmd/codex-stop-guard`
