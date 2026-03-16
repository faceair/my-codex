# Smart Commit

Create a git commit for the currently staged changes.

Workflow:
- Check staged changes first with `git diff --staged`.
- If nothing is staged, stop and tell the user there is nothing to commit.
- Review `git status --short` so you do not accidentally stage or commit unrelated work.
- Do not run `git add` unless the user explicitly asked for it.

Commit message rules:
- Analyze all staged changes and write a Git commit message.
- The summary line should describe the most impactful change.
- Pick a specific scope such as a component, file, or concrete subsystem.
- Format the summary as `[TYPE](scope): [brief description]`.
- Keep the summary line within 74 characters when possible.
- Prefer these commit types when they fit: `fix`, `feat`, `refactor`, `docs`, `config`.
- Use english for all commit text.
- If there are other notable changes, add them as extra body lines starting with `- `.
- Only include body lines for changes that are meaningfully different from the summary.
- Skip trivial details and implementation noise.
- Never mention Codex, AI, or generated content.

Execution rules:
- After drafting the message, run `git commit` yourself.
- Use non-interactive commit flags such as repeated `-m` arguments instead of opening an editor.
- Commit only what is already staged.
- If the commit fails, report the exact failure briefly and stop.

Final response format:
- First line: commit hash
- Second line onward: exact commit message
