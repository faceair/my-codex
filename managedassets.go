package managedassets

import "embed"

// FS embeds the repo-managed assets used by `my-codex pull` release binaries.
//
//go:embed agents skills instructions config.toml
var FS embed.FS
