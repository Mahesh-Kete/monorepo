"""Rule: source_modified_after_checkout.

Catches writes under ``$GITHUB_WORKSPACE`` that happen *outside* the legitimate
build-tooling pathways. The agent's file probe also produces ``file_tamper``
events at the end of the job (via snapshot+diff); this rule fires LIVE during
the run, before the diff runs.
"""

from __future__ import annotations

from ..models import Detection, ListedEvent


# Comms allowed to write into the workspace — anything else is suspicious.
LEGIT_WRITERS = {
    "git", "rsync", "tar", "unzip", "cp", "mv",
    "node", "npm", "yarn", "pnpm", "tsc", "webpack", "rollup", "vite",
    "go", "cargo", "rustc", "cc", "gcc", "ld", "as",
    "python", "python3", "pip",
    "java", "javac", "mvn", "gradle",
    "make", "cmake", "ninja",
}

# Steps where source writes are expected and harmless.
CHECKOUT_STEPS_SUBSTRINGS = ("checkout", "actions/checkout")


def _is_checkout_step(step: str) -> bool:
    s = step.lower()
    return any(sub in s for sub in CHECKOUT_STEPS_SUBSTRINGS)


class SourceTamperRule:
    name = "source_modified_after_checkout"

    def evaluate(self, le: ListedEvent, context) -> list[Detection]:
        e = le.payload
        if e.type != "file" or not e.file:
            return []

        # Only care about workspace writes. The file probe already filters most
        # of these via CITADEL_WATCH_PATH, but we double-check.
        if "/home/runner/work" not in e.file.path and "/_work/" not in e.file.path:
            return []
        if _is_checkout_step(e.workflow.step):
            return []

        # The agent doesn't carry comm directly on file events, so reach into
        # the process chain (which the file probe enriches inline).
        head_comm = e.process_chain[0] if e.process_chain else ""
        if head_comm and head_comm in LEGIT_WRITERS:
            return []

        return [
            Detection(
                run_id=le.run_id,
                event_id=le.id,
                rule_name=self.name,
                severity="high",
                message=(
                    f"Workspace file {e.file.path!r} written by "
                    f"{head_comm or '(unknown)'!r} during {e.workflow.step or '(unknown step)'!r}"
                ),
            )
        ]
