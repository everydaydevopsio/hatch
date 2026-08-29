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

## Initialize

Configure the hostname that users will use to reach the server:

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

1. validate that the start URL uses HTTP or HTTPS;
2. allocate an available TCP port from `18000-18999`;
3. generate a short session ID;
4. create a Docker container named `hatch-<session>`;
5. run the container with host networking so Chromium can reach OAuth callback listeners on server loopback;
6. set `HATCH_START_URL` to the supplied URL;
7. set the Hatch HTTPS listener to the allocated port;
8. wait for the container to emit its signed Guacamole launch path;
9. print a browser URL built from the configured hostname, allocated port, and Guacamole JWT path.

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
