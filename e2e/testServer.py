from os import curdir
from os.path import join as pjoin

from http.server import BaseHTTPRequestHandler, HTTPServer

class FileHandler(BaseHTTPRequestHandler):
    store_path = "/workspace/output"
    protocol_version = "HTTP/1.1"

    def do_GET(self):
        content = "".encode(encoding="utf-8")
        self.send_response(404, "not found")
        self.send_header("Content-type", "application/json")
        self.send_header("Content-length", len(content))
        self.end_headers()
        self.wfile.write(content)
        return

    def do_POST(self):
        content = "x".encode(encoding="utf-8")
        try:
            if len(self.path.split(".")) == 1:
                length = self.headers['content-length']
                data = self.rfile.read(int(length))

                with open(pjoin(self.store_path, self.path.split("/")[-1] + ".json"), 'w') as fh:
                    fh.write(data.decode())

                self.send_response(200, "success")
            else:
                self.send_response(404, "not found")
        except Exception as e:
            print(e)

            self.send_response(500)
            content = str(e).encode(encoding="utf-8")
        self.send_header("Content-type", "application/text")
        self.send_header("Content-length", len(content))
        self.end_headers()
        self.wfile.write(content)
        return


server = HTTPServer(('', 8080), FileHandler)

server.serve_forever()