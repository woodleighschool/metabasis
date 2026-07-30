# ADOverseas

Adds user to appropriate groups when notified by webhook

```bash
curl -X POST http://localhost:3500/schedule \
     -H "Authorization: Bearer supersecrettoken" \
     -H "Content-Type: application/json" \
     -d '{
          "username": "teststudent",
          "start_date": "2023-08-09 14:25:00",
          "end_date": "2023-08-09 14:26:00"
         }'
```

ensure to change start_date and end_date accordingly

## Development

Install the pinned toolchain and dependencies, then run the backend and web development servers:

```bash
mise install
mise run deps
mise run dev
```
