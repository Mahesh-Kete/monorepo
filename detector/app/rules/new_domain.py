"""Rule: outbound TCP to a domain that's not in the per-job baseline."""

from __future__ import annotations

from ..models import Detection, ListedEvent


class NewDomainRule:
    name = "new_outbound_domain"

    def evaluate(self, le: ListedEvent, context) -> list[Detection]:
        e = le.payload
        if e.type != "network" or not e.network:
            return []
        host = e.network.hostname or e.network.dst_ip
        if not host:
            return []

        # Only emit once the baseline is stable — otherwise every run is noisy.
        if context.baseline.status(context.key_for(e)) != "stable":
            return []
        if context.baseline.is_known_domain(context.key_for(e), host):
            return []

        proc = e.network.process or "?"
        step = e.workflow.step or "(unknown step)"
        return [
            Detection(
                run_id=le.run_id,
                event_id=le.id,
                rule_name=self.name,
                severity="medium",
                message=f"Unexpected outbound destination {host}:{e.network.dst_port} from {proc} during {step!r}",
            )
        ]
