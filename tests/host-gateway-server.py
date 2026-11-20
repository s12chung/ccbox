#!/usr/bin/env python3
"""The host-gateway bats test's server: answers one GET with a fixed body, then
exits — the host side of the devbox's host-gateway ask.

usage: python3 host-gateway-server.py <port>
"""

import http.server
import sys


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        # no Content-Length: HTTP/1.0's reply body ends at the connection close
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"ccbox host gateway")

    def log_message(self, *args):
        pass


# 0.0.0.0: the devbox arrives at the host's bridge address, not its loopback
server = http.server.HTTPServer(("0.0.0.0", int(sys.argv[1])), Handler)
server.handle_request()
