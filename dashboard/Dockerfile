# Phase 0 STUB Dockerfile.
#
# Replaced in Phase 7 by a real Next.js 14 multi-stage build. For now: a
# tiny Node http server on :3000 so the compose stack lights up.
FROM node:20-alpine

WORKDIR /app

RUN printf '%s\n' \
    'const http = require("http");' \
    'const backend = process.env.NEXT_PUBLIC_BACKEND_URL || "<unset>";' \
    'const server = http.createServer((_req, res) => {' \
    '  res.writeHead(200, { "Content-Type": "text/plain" });' \
    '  res.end(`citadel-dashboard Phase 0 stub\nNEXT_PUBLIC_BACKEND_URL=${backend}\n`);' \
    '});' \
    'server.listen(3000, "0.0.0.0", () => {' \
    '  console.log("citadel-dashboard stub listening on :3000");' \
    '});' \
    > stub.js

EXPOSE 3000
CMD ["node", "stub.js"]
