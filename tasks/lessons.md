# Lessons

## Persistent state changes shutdown requirements

When adding a persistent volume to a previously disposable container, review every container removal path for abrupt termination. A forced removal can interrupt writes to browser databases. Test the stop behavior and recovery from a stopped container whose stable name blocks a new launch. Trigger this check whenever storage survives a container's lifecycle.

## Verify shell success after process checks

In a `set -e` shutdown script, a final `condition && action` can leave a failure status even when the condition correctly reports that no processes remain. End a successful process check explicitly with `exit 0`, and make the Docker smoke test fail on shutdown warnings.
