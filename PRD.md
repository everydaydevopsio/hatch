# PRD

## Integrated HTTPS Guacamole Access

### Requirement

Hatch must provide browser-based desktop access through HTTPS from the container without requiring users to connect an RDP client directly.

### Acceptance Criteria

- The default container listens on HTTPS port `443`.
- Plain HTTP requests sent to the published HTTPS port redirect to the equivalent `https://` URL.
- The HTTPS server proxies `/guacamole/` to Guacamole on `127.0.0.1:8080`.
- Guacamole connects through `guacd` on `127.0.0.1:4822`.
- `guacd` connects to xrdp on `127.0.0.1:3389`.
- xrdp is not published as a direct external service by the default image.
- A self-signed TLS certificate is generated automatically when no certificate is mounted.
- Guacamole access is launched only from a generated signed-JWT Hatch URL instead of a reusable Guacamole username/password.
- Direct visits to `/guacamole/` without a token land on the Hatch page instead of the Guacamole login page.
- The generated RDP credentials are written inside the container for operator recovery, but are not printed as Guacamole login credentials.
- Docker users can map any host port to container port `443`, for example `-p 8443:443`.
- Host-network OAuth callback mode remains documented for cases where Chromium must reach a callback listener on host loopback.
- Docker Compose uses host-network OAuth callback mode, listens on host port `8443` by default, and does not rely on ignored port mappings.
- The default desktop session maps remote Super/Mac-style shortcuts such as paste, copy, and address-bar focus to the Linux Ctrl shortcuts expected by Chromium, with an environment variable to disable the mapping.
- Operators can set `HATCH_START_URL` so Chromium opens the requested URL when the desktop session starts; the default remains `about:blank`.
- Default Docker and Docker Compose starts do not show Chromium's unsupported `--no-sandbox` warning.
- The README presents the HTTPS Guacamole flow as the primary quickstart and keeps Docker Compose as a lower-priority option.
- An E2E smoke test validates the HTTPS Guacamole login path and confirms the browser desktop starts at the configured `HATCH_START_URL`, using `HATCH_E2E_URL` as the test harness input that is passed through to the container.
