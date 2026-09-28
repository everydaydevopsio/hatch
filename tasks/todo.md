# Persistent Chromium profiles

- [x] Create a feature branch and define acceptance criteria in `PRD.md`.
- [x] Add failing tests for profile parsing, volume mount, and same-profile session isolation.
- [x] Implement `hatch open --profile <name>` with a persistent named Docker volume.
- [x] Document usage, profile deletion, and the one-session-per-profile rule.
- [x] Run the full Go suite and coverage check, review the diff, and validate rollback.

Scope: CLI container creation and operator documentation. Keep the default session disposable and leave Compose behavior unchanged. Use Docker named volumes to avoid host path and ownership setup. An active profile must have a stable container name so Docker atomically rejects another session for that profile. Test the generated container options and argument validation; run a live Docker check if the local daemon is available. Rollback: use `hatch open` without `--profile`; remove any unwanted profile volume explicitly after its session stops.

Evidence: `go test -cover ./...` passed at 33.7% (base branch: 31.3%); `go vet ./...`, `sh -n scripts/entrypoint.sh`, Docker image build, and `git diff --check` passed. A Docker smoke run confirmed `/home/oauth` and the profile mount are owned by `oauth` with mode `0700`, wrote a file as `oauth` to the named volume, and read it from a second container after the first was removed. The temporary smoke volume was removed. The existing 75% coverage merge gate remains unmet; the user explicitly requested a PR with this known gap, which must be resolved before merge.

## Clean profile shutdown

- [x] Request Chromium window close and wait for its browser process to exit.
- [x] Bound shutdown time and terminate a hung browser before removing the container.
- [x] Test stop ordering and failure paths, then run Go checks and an isolated Docker smoke test.

Scope: profile-backed CLI sessions. Preserve the existing ephemeral stop behavior and profile volume. Rollback: restore the previous profile stop helper; existing volumes remain usable.

Evidence: The new lifecycle test failed before implementation and passed after it. `go test ./...`, `go vet ./...`, `sh -n cmd/hatch/chromium-close.sh`, and `git diff --check` passed. An isolated Docker/Guacamole smoke run launched Chromium, ran the updated CLI `stop`, confirmed container removal and profile volume preservation, and emitted no close warning. The running `wepro` session was untouched. Local statement coverage is 38.7%, below the existing 75% merge gate.
