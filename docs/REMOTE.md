# Hatch Remote Sessions

`hatch open-remote` launches the same ephemeral browser desktop as `hatch open`, then registers the session with the Hatch remote service so paired mobile and desktop apps can be notified.

```bash
export HATCH_REMOTE_TOKEN='<server-registration-token>'
hatch open-remote 'https://example.com/oauth/authorize?...'
```

The default rendezvous service is `https://hatch.orchael.com`. Development deployments can override it with `HATCH_REMOTE_URL`.

## Pairing direction

The companion app is `everydaydevopsio/hatch-app`. The pairing flow will use a short-lived URL in this form:

```text
https://hatch.orchael.com/h/<unique-id>
```

`hatch remote pair` will print that URL and a terminal QR code containing the same HTTPS URL. The HTTPS URL is intended to become an iOS Universal Link and Android App Link so scanning it opens the Hatch app when installed.

The short ID is a rendezvous identifier, not the server authentication secret. Pairing credentials must be high entropy, short lived, and single use.

## Session notification

The CLI sends the remote service a session ID, the configured server hostname, the target hostname, the temporary Hatch launch URL, and expiry time. The OAuth query string is not sent as notification metadata.

If notification fails, `open-remote` stops the newly-created Hatch container rather than leaving an inaccessible session running.

## Clipboard scope

The first mobile/desktop app supports text-only paste. Clipboard access must happen only after the user taps **Paste Text**. The app then sends that text to the Guacamole remote clipboard. There is no background clipboard monitoring, automatic synchronization, image/file clipboard support, or clipboard history.

## Follow-up

The registration and pairing API behind `hatch.orchael.com` is a separate deployment concern. Until the pairing command lands, `HATCH_REMOTE_TOKEN` supplies the server registration credential used by `open-remote`.
