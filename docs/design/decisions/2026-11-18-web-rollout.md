# Decision log: 2026-11-18, Web rollout, mail and settings defaults

Decided with the project owner on 2026-11-18, for
[../web-go.md](../web-go.md) and [../web-accounts.md](../web-accounts.md):

- Mail: web-email is built and tested against a local test mail
  server; the real SMTP relay is configured in the environment later.
- Language: English first. Every user-facing text (mail templates,
  pages) goes through a lookup by language from the start, so other
  languages, Indonesian first, can be added without code changes.
- Settings defaults for the site: timezone Asia/Jakarta, 24-hour
  clock, English, theme following the system.
- Pages move from Drogon to the Go service route by route, each once
  its data is ported and checked, as if Drogon were still in use. A
  temporary outage of the dashboard during the migration is
  acceptable.
- The Node render service may be deployed as its own container once
  ssr-render-service is ready, without a further decision.
