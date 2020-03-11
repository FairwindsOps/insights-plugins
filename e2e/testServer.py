from os import curdir
from os.path import join as pjoin

from http.server import BaseHTTPRequestHandler, HTTPServer

class FileHandler(BaseHTTPRequestHandler):
    store_path = pjoin(curdir, "../output")
    def do_POST(self):
        if len(self.path.split(".")) == 1:
            length = self.headers['content-length']
            data = self.rfile.read(int(length))

            with open(pjoin(self.store_path, self.path.split("/")[-1] + ".json"), 'w') as fh:
                fh.write(data.decode())

            self.send_response(200)


server = HTTPServer(('', 8080), FileHandler)
server.serve_forever()