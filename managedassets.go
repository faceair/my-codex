package managedassets

import "embed"

// FS embeds the repo-managed assets used by `my-codex pull` release binaries.
//
//go:embed agents prompts instructions config.toml hooks.json hooks/README.md
var FS embed.FS
