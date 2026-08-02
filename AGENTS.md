# AGENTS.md

Repository guidance for ADOverseas.

## Approach

- Stay within the requested scope and preserve unrelated local changes.
- This is a purpose-built internal application, not a SaaS platform. Prefer direct code for the current requirements.
- Simplify and modernize existing code before adding abstractions, compatibility layers, or configuration switches.
- Keep stack-specific choices local while following the shared Woodstar tooling baseline.

## Repository Map

- Process composition: `cmd/server`
- Authentication and Microsoft Graph integration: `internal/auth` and `internal/graph`
- HTTP application: `internal/http`
- Scheduling and persistence: `internal/schedules` and `internal/store`
- React frontend: `web/`
- SQL generation: `sqlc.yaml`

Keep transport, storage, and frontend concerns in their owning packages. Don't introduce a generic application framework.

## Commands

Use Mise tasks as the repository contract.

- Dependencies: `mise run deps`
- Build: `mise run build`
- Tests: `mise run test`
- Lint: `mise run lint`; fixes: `mise run lint-fix`
- Format: `mise run format`; check: `mise run fmt-check`
- Generated SQL: `mise run generate`
- Module and workflow checks: `mise run tidy-check`, `mise run workflow-lint`

Run the narrowest useful task while iterating, then run the relevant root checks before handing over.

## Engineering Rules

- Prefer concrete Go types, small consumer-owned interfaces, and wrapped errors.
- Keep Graph and identity details out of generic helpers.
- Frontend code uses Oxc and generated/domain types. Don't add a parallel formatting or lint stack.
- Keep secrets, real identities, tenant details, and local environment files out of tests and version control.
- API or generated-data changes must update their checked-in outputs in the same change.

## Commits

- Use focused Conventional Commits.
- Don't push, deploy, publish, or run live tenant operations unless explicitly requested.
- Report checks run, skipped checks, and unresolved failures.
