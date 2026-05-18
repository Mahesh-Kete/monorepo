"""Citadel detection rules.

Each rule is a class with a single ``evaluate(event, context)`` method that
returns either ``None`` or a list of ``Detection`` objects. The engine
(``app/engine.py``) instantiates every rule once and runs each incoming event
through all of them.
"""

from .new_domain import NewDomainRule
from .reverse_shell import ReverseShellRule
from .source_tamper import SourceTamperRule
from .suspicious_exec import SuspiciousExecRule
from .token_in_payload import TokenInPayloadRule

ALL_RULES = [
    NewDomainRule(),
    ReverseShellRule(),
    SourceTamperRule(),
    SuspiciousExecRule(),
    TokenInPayloadRule(),
]
