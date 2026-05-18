"""Rules: suspicious_shell_spawn and possible_reverse_shell.

``suspicious_shell_spawn`` fires on a process exec whose ``comm`` is shell-like
*and* whose parent is a known build tool. ``possible_reverse_shell`` upgrades
to severity=critical when the same parent also made an outbound TCP connection
within the previous second.
"""

from __future__ import annotations

from ..models import Detection, ListedEvent


SHELL_COMMS = {"sh", "bash", "dash", "zsh", "ksh", "nc", "ncat", "socat", "python", "python3", "perl", "ruby"}
BUILD_TOOL_COMMS = {"node", "npm", "yarn", "pnpm", "make", "gcc", "g++", "ld", "go", "cargo", "pip", "python", "python3"}


class ReverseShellRule:
    name = "reverse_shell"

    def evaluate(self, le: ListedEvent, context) -> list[Detection]:
        e = le.payload
        if e.type != "process" or not e.process:
            return []
        if e.process.comm not in SHELL_COMMS:
            return []

        parent_comm = context.comm_for(e.process.ppid)
        if not parent_comm or parent_comm not in BUILD_TOOL_COMMS:
            return []

        # Baseline check: if a stable job has always had this shell appear under
        # this parent, don't keep flagging it.
        if context.baseline.status(context.key_for(e)) == "stable" \
                and context.baseline.is_known_process(context.key_for(e), e.process.comm):
            return []

        message = (
            f"Build tool {parent_comm!r} (pid {e.process.ppid}) spawned shell "
            f"{e.process.comm!r} (pid {e.process.pid}) during {e.workflow.step or '(unknown step)'!r}"
        )

        # Upgrade to critical if the parent made an outbound connection within
        # the last 1s — that's the reverse-shell pattern in the wild.
        if context.recent_network_window(e.process.ppid, within_seconds=1):
            return [
                Detection(
                    run_id=le.run_id,
                    event_id=le.id,
                    rule_name="possible_reverse_shell",
                    severity="critical",
                    message=message + " — parent also made outbound TCP within 1s",
                )
            ]

        return [
            Detection(
                run_id=le.run_id,
                event_id=le.id,
                rule_name="suspicious_shell_spawn",
                severity="high",
                message=message,
            )
        ]
