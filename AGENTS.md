# Project Agent Rules

These rules apply to the whole repository unless a deeper `AGENTS.md` overrides them for a subdirectory.

## Migration Goal

- The primary goal is to gradually rewrite the backend in Go.
- Do not rewrite the frontend unless the user explicitly asks for it.
- Preserve the current frontend behavior during migration.

## API Compatibility

- Keep existing API paths compatible with the current frontend.
- Keep existing JSON response structures compatible with the current frontend.
- If a compatibility change appears necessary, explain it first before making code changes.

## Migration Scope

- Migrate only one small module at a time.
- Prefer incremental replacement over large rewrites.
- Do not delete the old Python backend unless the user explicitly requests it.

## Change Process

- Before modifying code, present a short plan for the proposed change.
- After making changes, run any relevant tests, checks, or startup commands that are available and practical.
- Report which files were changed and what was verified.

## Learning Mode

- Default to a learn-by-doing workflow.
- For completely unfamiliar technical points, it is allowed to provide the smallest complete example code first.
- Only provide one small file or one small interface at a time; do not generate a whole project in one step.
- After providing example code, explain the key syntax and each part's responsibility section by section.
- Assume the user will retype the code rather than copy it directly.
- After the user writes code, review it before expanding the implementation.
- During review, prioritize bugs, layering problems, and API compatibility issues.
- Do not proactively expand scope or add new features.
- Do not rewrite the whole project unless the user explicitly asks.
- Current phase: it is allowed to provide a minimal complete implementation for `/api/version`, but do not extend that implementation to other interfaces unless the user asks.

## Database Safety

- Do not change the database schema by default.
- If a schema change is necessary, explain why it is needed before making it.

## Go Backend Structure

- All new Go backend code should use a simple layered structure:
- `handler`
- `service`
- `repository`

- Keep the architecture straightforward and avoid unnecessary abstraction.
