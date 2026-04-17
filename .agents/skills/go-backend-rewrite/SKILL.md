---
name: go-backend-rewrite
description: Gradually migrate a legacy backend to Go while keeping the existing frontend unchanged and API-compatible. Use when the user asks to "迁移接口", "重写 Go 后端", "保持前端 API 兼容", or any similar task that requires reading the frontend API usage first, then rewriting one backend endpoint at a time in Go without deleting the old backend.
---

# Go Backend Rewrite

## Overview

Use this skill to keep migration work disciplined and incremental. The frontend is the contract owner; the legacy backend is the compatibility reference.

## Workflow

1. Read the frontend API usage first.
Read the files that define or call backend APIs before changing server code. Start with the shared API wrapper if it exists, then inspect the pages or components that consume each endpoint.

Useful targets in this repo usually include:
- `client/src/lib/api.js`
- frontend pages/components that call `fetch`, import `api`, or parse response fields

Confirm:
- exact API path
- HTTP method
- request body, query params, and headers
- response JSON shape that the frontend actually reads

2. Read the old backend implementation second.
Inspect the legacy backend entrypoint, route registration, handler, and any service or repository logic used by the target endpoint.

Useful targets in this repo usually include:
- `app.py`
- `backend/run.py`
- `backend/app/__init__.py`
- `backend/app/routes/*.py`
- related files under `backend/app/services`, `backend/app/models`, and `backend/app/utils`

Confirm:
- route behavior and edge cases
- status codes
- auth requirements
- DB access and external API calls
- response field names and fallback behavior

3. Output a migration plan before editing code.
Do not jump directly into implementation. Provide a short plan that names:
- the single endpoint being migrated this turn
- the files you expect to add or modify
- how compatibility will be preserved
- what you will run to validate the change

4. Implement only one interface per turn.
Do not batch multiple endpoints into one migration unless the user explicitly asks for it. If a route has an alias, treat the alias and primary path as part of the same interface only when they share one handler.

## Go Structure

Place new Go backend code in a simple layout:

```text
cmd/server/main.go
internal/handler/
internal/service/
internal/repository/
```

Keep the layering simple:
- `handler`: HTTP request parsing and JSON responses
- `service`: business logic and compatibility rules
- `repository`: database or upstream data access

Avoid adding extra architectural layers unless they are clearly necessary.

## Compatibility Rules

- Keep existing frontend API paths unchanged.
- Keep existing JSON response keys and general shapes unchanged.
- Do not rewrite frontend pages as part of backend migration work.
- Do not delete the old backend unless the user explicitly asks.
- Do not change the database schema unless you explain why first and the user approves.

If the old backend behavior is inconsistent, prefer matching what the frontend depends on rather than copying internal quirks blindly. State that choice clearly in the plan.

## Validation

After making Go changes:
- run `gofmt -w` on every new or edited Go file
- run any available Go tests that are relevant, preferably `go test ./...` when the module is ready
- if the server can start in the current environment, run a startup check such as `go run ./cmd/server` or the repo's actual startup command

If a command cannot run because the module is incomplete or required environment variables are missing, say so explicitly.

## Output Requirements

At the end of the task, report:
- the interface migrated this turn
- the files modified or added
- the validation commands you ran
- any remaining compatibility risks or follow-up work
