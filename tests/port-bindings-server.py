#!/usr/bin/env python3
"""The port-bindings bats test's server: answers every GET with a fixed body,
exiting after two requests or two seconds — the session's exec returns.

usage: python3 port-bindings-server.py <port>
"""

import http.server
import sys
import time

SERVE_REQUESTS = 2
SERVE_SECONDS = 2


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        # no Content-Length: HTTP/1.0's reply body ends at the connection close
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"ccbox port bindings")

    def log_message(self, *args):
        pass


server = http.server.HTTPServer(("0.0.0.0", int(sys.argv[1])), Handler)
server.timeout = 1  # an idle handle_request() blocks without it — the deadline never fires
server.served = 0
deadline = time.monotonic() + SERVE_SECONDS
while server.served < SERVE_REQUESTS and time.monotonic() < deadline:
    server.handle_request()
