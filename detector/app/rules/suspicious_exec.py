"""Rules: exec_from_temp and suspicious_downloader.

``exec_from_temp`` fires when a process is executed from /tmp, /dev/shm, or
/var/tmp — these are world-writable paths that should not host build binaries.
``suspicious_downloader`` fires when curl or wget is exec'd by an ancestor
that isn't on the known-CI-tooling list.
"""

from __future__ import annotations

from ..models import Detection, ListedEvent


TEMP_PREFIXES = ("/tmp/", "/dev/shm/", "/var/tmp/")

# Known CI tooling that legitimately invokes curl/wget (transitively):
# git, npm, pip, etc. Anything outside this is flagged.
CI_ANCESTORS = {
    "git", "npm", "yarn", "pnpm", "pip", "node",
    "go", "cargo",
    "apt-get", "apt", "dpkg",
    "make", "cmake",
    "actions-runner",  # the GitHub Actions runner process itself
}

DOWNLOADER_COMMS = {"curl", "wget"}


class SuspiciousExecRule:
    name = "suspicious_exec"

    def evaluate(self, le: ListedEvent, context) -> list[Detection]:
        e = le.payload
        if e.type != "process" or not e.process:
            return []

        out: list[Detection] = []

        # --- exec_from_temp ---
        if any(e.process.filename.startswith(p) for p in TEMP_PREFIXES):
            out.append(
                Detection(
                    run_id=le.run_id,
                    event_id=le.id,
                    rule_name="exec_from_temp",
                    severity="high",
                    message=(
                        f"Process executed from world-writable path: {e.process.filename!r} "
                        f"(comm={e.process.comm!r}, pid={e.process.pid})"
                    ),
                )
            )

        # --- suspicious_downloader ---
        if e.process.comm in DOWNLOADER_COMMS:
            # Look at ancestor comms (excluding the downloader itself).
            ancestors = [c for c in context.ancestry_comms(e.process.pid) if c != e.process.comm]
            if ancestors and not any(a in CI_ANCESTORS for a in ancestors):
                out.append(
                    Detection(
                        run_id=le.run_id,
                        event_id=le.id,
                        rule_name="suspicious_downloader",
                        severity="medium",
                        message=(
                            f"{e.process.comm!r} (pid {e.process.pid}) invoked by non-CI ancestor chain "
                            f"{' -> '.join(reversed(ancestors))!r}"
                        ),
                    )
                )

        return out
