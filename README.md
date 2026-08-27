# metabasis

[![Release](https://img.shields.io/github/v/release/woodleighschool/metabasis?display_name=tag&sort=semver)](https://github.com/woodleighschool/metabasis/releases/latest)
[![CI](https://github.com/woodleighschool/metabasis/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/woodleighschool/metabasis/actions/workflows/ci.yaml)
[![Go](https://img.shields.io/github/go-mod/go-version/woodleighschool/metabasis?logo=go)](https://github.com/woodleighschool/metabasis/blob/main/go.mod)
[![Container](https://img.shields.io/badge/container-ghcr.io-2496ED?logo=github&logoColor=white)](https://github.com/orgs/woodleighschool/packages/container/package/metabasis)
[![License](https://img.shields.io/github/license/woodleighschool/metabasis)](https://github.com/woodleighschool/metabasis/blob/main/LICENSE)

> **μετάβασις** (_metábasis_) — “transition; passage across”

Reconciles temporary Microsoft Entra group membership from webhook-driven intents and scheduled transitions.

Accepts a small canonical intent, persists it in PostgreSQL, derives the subject's aggregate temporal state, and applies only the Entra group membership assertions selected by configuration. The scheduler only decides when to recalculate them.

```mermaid
flowchart TD
  subgraph intake["Intent intake"]
    freshservice["Freshservice<br/>travel request"] --> webhook["Intent<br/>webhook"]
    webhook --> intent[("Validate and persist<br/>the intent")]
  end

  intent --> reconcile["Wake the reconciler"]

  subgraph planning["Calculate the current state"]
    reconcile --> identity["Resolve the user and<br/>current group membership"]
    identity --> policy["Apply configured policy<br/>across all current intents"]
    policy --> state{"Aggregate subject state"}

    state -->|No active; at least one pending| pending["Pending<br/>In Australia<br/>Prepare MFA"]
    state -->|At least one active| active["Active<br/>Outside Australia<br/>Allow access and enforce MFA"]
    state -->|All ended or cancelled| settled["Settled<br/>Temporary access no longer required"]

    pending --> assertions["Select explicit present<br/>and absent assertions"]
    active --> assertions
    settled --> assertions
  end

  assertions --> entra["Add missing present groups<br/>Remove current absent groups<br/>Preserve unmentioned groups"]
  entra -->|Success| schedule["Schedule the next<br/>phase transition"]
  entra -->|Graph failure| retry["Persist the error<br/>and retry time"]
  schedule -->|starts_at or ends_at| reconcile
  retry --> reconcile
```

## 🚀 Usage

Download CLI archives for macOS, Linux, or Windows from the [latest release](https://github.com/woodleighschool/metabasis/releases/latest), or use the container `ghcr.io/woodleighschool/metabasis:rolling`.

Start with the example policy and an environment file for its `${...}` values:

```bash
cp config.example.yaml config.yaml
touch .env
```

Fill `.env` with values for the `${...}` names in `config.yaml`. Docker Compose reads this file automatically; export the same values in your shell when using a downloaded binary.

| Command                                                 | Behaviour                                   |
| ------------------------------------------------------- | ------------------------------------------- |
| `metabasis validate`                                    | Validate configuration and exit             |
| `metabasis plan --event request.json`                   | Print a read-only plan for one event        |
| `metabasis intents list`                                | Inspect accepted intents                    |
| `metabasis reconcile --subject student@example.invalid` | Reconcile one stored subject now            |
| `metabasis reconcile --all`                             | Reconcile every stored subject now          |
| `metabasis run`                                         | Serve webhooks and scheduled reconciliation |

If `config.yaml` is in the current directory, `--config` may be omitted. Multiple `--config` flags apply overlays in order.

### Docker Compose

Start PostgreSQL and the service:

```bash
docker compose up -d
```

Run one command with the same configuration:

```bash
docker compose run --rm metabasis validate
docker compose run --rm \
  --volume "$PWD/request.json:/request.json:ro" \
  metabasis plan --event /request.json
```

Daemon mode writes structured JSON to stderr. Lifecycle and material reconciliation events use `info`, warnings and failures use `warn` or `error`, and successful cycle summaries plus routine no-op evaluations use `debug`.

## ⚙️ Configuration

Configuration is strict. Mappings merge recursively; lists and scalar values replace earlier values. Environment placeholders must occupy a whole YAML value such as `${MICROSOFT_CLIENT_SECRET}`.

Runtime settings resolve from `METABASIS_*` environment variables, then the corresponding YAML value, then the default. CLI flags select configuration files or command behaviour rather than mirroring runtime settings.

| Environment variable                          | YAML fallback                       | Default  |
| --------------------------------------------- | ----------------------------------- | -------- |
| `METABASIS_LOG_LEVEL`                         | `log_level`                         | `info`   |
| `METABASIS_LISTEN`                            | `listen`                            | `:8080`  |
| `METABASIS_METRICS_LISTEN`                    | `metrics_listen`                    | `:8081`  |
| `METABASIS_DATABASE_URL`                      | `database.url`                      | required |
| `METABASIS_DATABASE_MIN_CONNECTIONS`          | `database.min_connections`          | `0`      |
| `METABASIS_DATABASE_MAX_CONNECTIONS`          | `database.max_connections`          | `10`     |
| `METABASIS_DATABASE_MAX_CONNECTION_LIFETIME`  | `database.max_connection_lifetime`  | `30m`    |
| `METABASIS_DATABASE_MAX_CONNECTION_IDLE_TIME` | `database.max_connection_idle_time` | `5m`     |
| `METABASIS_DATABASE_HEALTH_CHECK_PERIOD`      | `database.health_check_period`      | `1m`     |
| `METABASIS_RECONCILE_POLL_INTERVAL`           | `reconcile.poll_interval`           | `1m`     |
| `METABASIS_RECONCILE_RETRY_INITIAL`           | `reconcile.retry_initial`           | `30s`    |
| `METABASIS_RECONCILE_RETRY_MAX`               | `reconcile.retry_max`               | `15m`    |

## 🪝 Webhook

The canonical webhook body is:

```json
{
  "id": "SR-12345",
  "subject": "student@example.com",
  "starts_at": "2026-09-12T08:00:00+10:00",
  "ends_at": "2026-09-27T18:00:00+10:00",
  "cancelled": false
}
```

Each configured webhook source uses bearer authentication. Repeated `(source, id)` deliveries replace the stored intent and wake reconciliation immediately.

## 🔄 Reconciliation

The first matching CEL rule applies to a subject. The service selects one aggregate state across all known intents: active takes precedence over pending, while settled means every known intent has ended or was cancelled. A subject with no known intents produces no membership assertions.

`identity.groups` maps provider group IDs to aliases available to CEL and membership assertions. `present` adds a missing membership, `absent` removes an existing membership, and an unmentioned alias is preserved. Writable aliases must resolve to exactly one group ID. Adds are attempted before removals; Graph failures leave the accepted intent intact and persist retry state.

`plan` is read-only. It overlays the supplied event on persisted intents and shows the resolved user, matched rule, aggregate state, intent phases, present and absent assertions, current aliases, diff, and next transition.

## 🌐 HTTP

`run` serves configured `/webhooks/...` paths plus:

- `/healthz`
- `/readyz`

The `metrics_listen` address (default `:8081`) serves `/metrics`. It includes standard Go/process metrics and bounded service metrics for build information, webhook results, reconciliation results and duration, current intent phases, failed subjects, due subjects, and database state-collection success.

Database-derived gauges are collected at scrape time. If that query fails, `/metrics` remains available, `metabasis_state_collection_success` is `0`, the stale database gauges are omitted, and the database error is logged.

Read-only `intents` commands may run alongside the service. Webhook updates and reconciliation are serialized per subject through PostgreSQL advisory locks.

## 🧑‍💻 Development

Mise owns the toolchain and repository checks:

```bash
mise run build
mise run generate
mise run test
mise run test-postgres
mise run lint
mise run fmt-check
mise run workflow-lint
```

PostgreSQL tests require `METABASIS_TEST_DATABASE_URL`. Tests use synthetic identities and local HTTP servers; real Graph credentials are not required.

## 📄 License

Licensed under the [Apache License 2.0](LICENSE).
