# adoverseas

Schedules temporary Microsoft Entra group changes for people travelling away from school. The Go backend serves the web interface and stores schedules in PostgreSQL.

## 🚀 Usage

Create a `.env` file, then start the published image and PostgreSQL:

```bash
docker compose up -d
```

Submit a schedule with the configured bearer token:

```bash
curl --fail-with-body http://localhost:8080/api/schedule/ \
  --header "Authorization: Bearer $API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "email": "traveller@example.com",
    "leaving_date": "2026-09-01T08:00:00+10:00",
    "returning_date": "2026-09-15T17:00:00+10:00"
  }'
```

Dates use RFC 3339. The web interface is served from the same address.

## ⚙️ Configuration

Configuration comes from environment variables.

| Variable                   | Required | Purpose or default                          |
| -------------------------- | -------- | ------------------------------------------- |
| `API_KEY`                  | Yes      | Bearer token for schedule submissions       |
| `SITE_BASE_URL`            | Yes      | Public application URL                      |
| `SESSION_SECRET`           | Yes      | Session signing secret; at least 32 bytes   |
| `ADMIN_OIDC_ISSUER`        | Yes      | OIDC issuer for administrator sign-in       |
| `ADMIN_OIDC_CLIENT_ID`     | Yes      | OIDC client ID                              |
| `ADMIN_OIDC_CLIENT_SECRET` | Yes      | OIDC client secret                          |
| `GRAPH_TENANT_ID`          | Yes      | Microsoft Graph tenant                      |
| `GRAPH_CLIENT_ID`          | Yes      | Microsoft Graph client ID                   |
| `GRAPH_CLIENT_SECRET`      | Yes      | Microsoft Graph client secret               |
| `STAFF_DEPARTMENT`         | Yes      | Comma-separated staff departments           |
| `AWAY_GROUPS`              | Yes      | Comma-separated groups applied while away   |
| `HOME_GROUPS`              | Yes      | Comma-separated groups restored on return   |
| `ENABLE_MFA_GROUP`         | No       | Group used when MFA is enabled              |
| `FORCE_MFA_GROUP`          | No       | Group used to require MFA while away        |
| `TIME_LOCATION`            | No       | Schedule timezone; `Australia/Melbourne`    |
| `LISTEN_ADDR`              | No       | HTTP listen address; `:8080`                |
| `LOG_LEVEL`                | No       | `debug`, `info`, `warn`, or `error`; `info` |
| `DATABASE_*`               | Yes      | PostgreSQL connection settings              |
| `DB_MIN_CONNECTIONS`       | No       | Minimum pool size; `2`                      |
| `DB_MAX_CONNECTIONS`       | No       | Maximum pool size; `10`                     |
| `DB_MAX_CONN_LIFETIME`     | No       | Maximum connection lifetime; `30m`          |
| `INITIAL_ADMIN_PASSWORD`   | No       | Initial local administrator password        |

The Compose file supplies its own local PostgreSQL connection values.

## 🧑‍💻 Development

Mise owns the toolchain and repository commands:

```bash
mise install
mise run deps
mise run dev
```

Run `mise tasks` for the available checks and generation commands.

## 📄 License

Licensed under the [Apache License 2.0](LICENSE).
