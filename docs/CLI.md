# Hatch Go CLI

The `hatch` CLI manages Hatch browser-desktop sessions through the Docker Engine API.

## Build

```bash
go build -o hatch ./cmd/hatch
sudo install -m 0755 hatch /usr/local/bin/hatch
```

Build the Hatch container image once on the server:

```bash
docker build -t hatch:local .
```

The initial CLI intentionally uses `hatch:local` so the container image and CLI can evolve together without introducing registry authentication into the first implementation.

## Docker preflight

Before any Docker-dependent command, Hatch checks two things:

1. whether the `docker` command is installed;
2. whether the Docker Engine API responds to a Docker SDK ping.

If Docker is not installed, commands such as `hatch <url>`, `hatch list`, and `hatch stop` tell the user to run:

```bash
hatch init
```

`hatch init` detects the operating system and prints installation instructions for macOS, Windows, or Linux. On Linux, Hatch reads `/etc/os-release` and provides more specific guidance for Ubuntu, Debian, Fedora, CentOS, and RHEL when possible.

If Docker is installed but the daemon is not running, Hatch prints platform-specific startup instructions immediately:

- macOS: start Docker Desktop with `open -a Docker` or from Applications;
- Windows: launch Docker Desktop from the Start menu or PowerShell;
- Linux: run `sudo systemctl start docker`, with an option to enable it at boot.

Hatch does not attempt to install Docker or start privileged services automatically. It explains the required action and leaves control with the operator.

## Initialize

You can run initialization without arguments first to verify Docker prerequisites:

```bash
hatch init
```

If Docker is missing, Hatch prints installation instructions and exits successfully so `init` acts as the setup/help path. If Docker is installed but stopped, it prints startup instructions. If Docker is ready, Hatch asks you to finish configuration with a hostname:

```bash
hatch init devbox.tailnet.ts.net
```

This writes:

```text
~/.config/hatch/hatch.yaml
```

with:

```yaml
hostname: devbox.tailnet.ts.net
```

The config directory is created with mode `0700`; the config file is written with mode `0600`.

You can also provide a URL. Hatch stores only its hostname:

```bash
hatch init https://devbox.tailnet.ts.net:8443/
```

## Launch a browser session

```bash
hatch 'https://example.com/oauth/authorize?...'
```

Hatch will:

1. verify that Docker is installed and running;
2. validate that the start URL uses HTTP or HTTPS;
3. allocate an available TCP port from `18000-18999`;
4. generate a short session ID;
5. create a Docker container named `hatch-<session>`;
6. run the container with host networking so Chromium can reach OAuth callback listeners on server loopback;
7. set `HATCH_START_URL` to the supplied URL;
8. set the Hatch HTTPS listener to the allocated port;
9. wait for the container to emit its signed Guacamole launch path;
10. print a browser URL built from the configured hostname, allocated port, and Guacamole JWT path.

Example output:

```text
Session:      8ac4d911
Container:    hatch-8ac4d911
Start URL:    https://example.com/oauth/authorize?...
Browser URL:  https://devbox.tailnet.ts.net:18000/hatch/?token=eyJ...
Stop with:    hatch stop 8ac4d911
```

The container uses Docker host networking intentionally. This preserves the key OAuth behavior where a redirect such as `http://127.0.0.1:8765/callback` reaches the CLI process running on the Linux host.

## List sessions

```bash
hatch list
```

Hatch identifies its containers through Docker labels rather than relying only on names.

## Stop a session

```bash
hatch stop 8ac4d911
```

A unique session prefix is also accepted. Hatch force-removes the matching ephemeral container.

## Current defaults

```text
Docker image: hatch:local
HTTPS port range: 18000-18999
Guacamole launch token TTL: 3600 seconds
Network mode: host
Shared memory: 1 GiB
no-new-privileges: enabled
```

These are code defaults for the first version. Only the public hostname is persisted in the YAML configuration.

## Docker SDK

The CLI uses the supported Moby Go modules for the Docker Engine API:

```text
github.com/moby/moby/client
github.com/moby/moby/api
```

The older monolithic `github.com/docker/docker` Go module is deprecated for new releases.
