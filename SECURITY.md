# Security Policy

## Supported versions

Only the latest `main` branch (and the most recent tag built from it) is supported.

## Reporting a vulnerability

Report privately. Do not open a public issue.

- GitHub Security Advisory: https://github.com/ijon6k/gopod/security/advisories/new
- Email: erikghafari@gmail.com

Include what you can of the following:

- Affected component: `backend/`, `frontend/`, `caddy/`, or the installer
- Commit or version
- Environment: Podman version, rootless/user-socket setup, Caddy configuration, OS
- Minimal reproduction steps or proof of concept

## Scope

In scope:

- Go HTTP API, authentication/session handling, credential storage
- The Podman rootless socket client and everything it can be made to execute
- Caddy admin API usage and generated route configuration (header injection, host confusion, path mistakes)
- The SvelteKit frontend where it handles auth state, tokens, or API responses

Out of scope:

- Vulnerabilities in third-party images that GOPOD merely deploys; report those upstream
- Issues that already require root access to the host, or access to the user's Podman socket
- Security of features you have explicitly exposed to the public internet without Caddy/limits

## Handling

- Acknowledgement within 7 days, best effort
- Fix on `main` for confirmed issues, then coordinated disclosure. Request up to 90 days before publishing details
- Reports are credited by default; say so if you prefer anonymity
