# Phase 0 STUB Dockerfile.
#
# Replaced in Phase 5 by a real multi-stage Go build (chi + modernc.org/sqlite).
# This stub just brings the container up green on :8080 so `make docker-up`
# satisfies Phase 0 acceptance.
FROM python:3.11-slim

WORKDIR /app

RUN printf '%s\n' \
    'from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer' \
    '' \
    'class H(BaseHTTPRequestHandler):' \
    '    def do_GET(self):' \
    '        self.send_response(200)' \
    '        self.send_header("Content-Type", "text/plain")' \
    '        self.end_headers()' \
    '        self.wfile.write(b"citadel-backend Phase 0 stub\n")' \
    '    def log_message(self, *a, **kw):' \
    '        return' \
    '' \
    'print("citadel-backend stub listening on :8080", flush=True)' \
    'ThreadingHTTPServer(("0.0.0.0", 8080), H).serve_forever()' \
    > stub.py

EXPOSE 8080
CMD ["python", "stub.py"]
