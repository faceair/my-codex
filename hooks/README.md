# hooks

运行时 Stop hook 已迁移到独立 Go 二进制 `codex-stop-guard`。

- 仓库源码：`cmd/codex-stop-guard/`
- 本地安装位置：`~/.codex/hooks/codex-stop-guard`（Windows 为 `codex-stop-guard.exe`）
- `hooks.json` 会由 `my-codex pull` 按本地平台自动改写调用路径

保留这个目录是为了：
- 让 repo <-> `~/.codex` 继续保持 `hooks/` 目录级同步语义
- 在 pull 时清理旧的 `stop_continue_if_todo.py` 等遗留文件
