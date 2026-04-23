---
name: changelog-skill
description: >
    Generate a changelog entry from commits or a change description, combining Keep a Changelog
    structure with Conventional Commits. Use this skill whenever the user wants to: write
    changelog entries, convert a git log or commit list into release notes, or format changes
    into Added/Changed/Fixed/Removed/etc. sections. Trigger on phrases like "write a changelog",
    "generate changelog entry", "add release notes", "convert commits to changelog", or when
    the user pastes a list of commits and wants structured release notes.
---

# Changelog Skill

Produces a single, clean changelog **entry** — just the content block, ready to paste into
a `CHANGELOG.md`. No full file wrapper unless the user explicitly asks for one.

---

## Input modes

Detect which form the user is providing:

**A) Conventional Commits list** — raw commits like `feat(auth): add OAuth2 login`
→ Map to Keep a Changelog sections (see mapping table below).

**B) Free-form description** — plain language like _"added dark mode, fixed logout crash"_
→ Organize into the appropriate sections.

---

## Commit → section mapping

| Commit type                                                   | Section                                    |
| ------------------------------------------------------------- | ------------------------------------------ |
| `feat`                                                        | Added                                      |
| `fix`                                                         | Fixed                                      |
| `perf`, `refactor`                                            | Changed                                    |
| `deprecate`                                                   | Deprecated                                 |
| `revert`, `remove`                                            | Removed                                    |
| `security`                                                    | Security                                   |
| `BREAKING CHANGE` footer or `!` suffix (e.g. `feat!`, `fix!`) | **Breaking:** prefix, first in its section |
| `chore`, `ci`, `build`, `test`, `docs`, `style`               | **Omit** — noise for end-users             |

---

## Output format

Return only the entry block. No `# Changelog` header, no footer links, no preamble.

```
## [X.Y.Z] - YYYY-MM-DD

### Added
- ...

### Fixed
- ...
```

- Include only sections that have content — never output an empty `### Fixed` with no bullets.
- Section order: Added → Changed → Deprecated → Removed → Fixed → Security.
- Use today's date if none is given.
- If no version is provided, infer the bump (see below) and show it — don't ask unless truly ambiguous.

---

## Writing style

- **User benefit first**, not implementation detail.
    - ❌ `refactor button to use CSS variables`
    - ✅ `Redesigned buttons for consistent theming`
- **Past-tense verb** to start each bullet: _Added_, _Fixed_, _Removed_, _Improved_.
- **Merge duplicate commits** touching the same feature into one bullet.
- **Breaking changes**: bold `**Breaking:**` prefix, listed first within their section.
- **Omit** CI, formatting, typo-only, and dependency bump commits — they don't matter to users.

---

## Version inference (silent)

| Change set contains                   | Suggested bump |
| ------------------------------------- | -------------- |
| `BREAKING CHANGE` or removed features | major          |
| Any `feat`                            | minor          |
| Only `fix` / `perf` / `security`      | patch          |

State the inferred version in the entry header. Only ask if it's genuinely unclear.

---

## Example

Input:

```
feat(auth): add Google OAuth login
feat: add dark mode
fix: crash on logout with expired session
fix(api): rate limit counter not resetting
perf: lazy-load dashboard widgets
BREAKING CHANGE: drop support for Node.js 14
chore: bump eslint
ci: fix Actions workflow
```

Output:

```
## [2.0.0] - 2026-04-15

### Added
- Added Google OAuth login support
- Added dark mode with system preference detection

### Changed
- **Breaking:** Dropped support for Node.js 14. Upgrade to Node.js 18+.
- Improved dashboard load time with lazy-loaded widgets

### Fixed
- Fixed crash when logging out with an expired session
- Fixed API rate limit counter not resetting correctly
```
