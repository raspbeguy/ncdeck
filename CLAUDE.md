# Project conventions for AI assistants

## Code comments

Default to no comments. Only add one when the **why** is non-obvious: a hidden constraint, a subtle invariant, a workaround for a specific bug, an API quirk that would surprise the next reader. Never describe **what** the code does, well-named identifiers do that already.

- Skip docstrings on functions whose name and signature say enough (`daysInMonth() int`, `setStatus(text string)`).
- Skip inline comments that paraphrase the next line.
- When the comment is justified, keep it short, one line is usually enough.
- When editing code, drop existing WHAT-only comments instead of preserving them out of inertia.

## Commits

- One-line subject, no body unless asked.
- Conventional-commit prefixes (`feat:`, `fix:`, `refactor:`, `chore:`, `docs:`, `ci:`) are fine.
- Never add a `Co-Authored-By:` trailer.

## Writing style

No em-dashes anywhere (commit messages, comments, prose). Use commas, colons, or parentheses.

## Verification

`go vet ./...`, `go test ./...`, `go build ./...` and the three linters (`staticcheck`, `unparam`, `deadcode`) all kept clean.
