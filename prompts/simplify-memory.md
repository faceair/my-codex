Rewrite the repository root `./MEMORY.md`.

Before rewriting, inspect the repo itself to understand the real project structure, module boundaries, naming patterns, and how these constraints are reflected in code. Read the current `./MEMORY.md` first, then use the repository layout and nearby code/tests as context. Do not treat this as a text-only summarization task.

Requirements:
- Preserve the existing language, format, and entry style of `./MEMORY.md`.
- Do not introduce new sections, headings, or a new template unless the file already uses them.
- Keep only durable constraints that are likely to affect future work.
- Remove redundancy, dates, historical framing, temporary migration notes, and execution details.
- Generalize where appropriate, but do not lose important boundaries or exceptions.
- Prefer clarity over compression: use multiple short entries rather than dense combined ones.
- Keep one durable constraint per entry whenever possible.
- Preserve repo terminology, file names, code identifiers, and technical terms as they are.
- Output only the final rewritten `./MEMORY.md` content.
- Do not explain your reasoning.

Use both:
1. the current `./MEMORY.md`
2. the repository code/tests/docs structure
