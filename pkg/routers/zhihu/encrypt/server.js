const http = require('http');
const url = require('url');
const { getEncryptCode } = require('./encrypt.js');

const server = http.createServer((req, res) => {
  const queryObject = url.parse(req.url, true).query;
  if (req.method === 'GET' && queryObject.md5) {
    try {
      const encryptedCode = getEncryptCode(queryObject.md5);
      res.writeHead(200, { 'Content-Type': 'text/plain' });
      res.end(encryptedCode);
    } catch (error) {
      res.writeHead(500, { 'Content-Type': 'text/plain' });
      res.end("Internal Server Error");
    }
  } else {
    res.writeHead(400, { 'Content-Type': 'text/plain' });
    res.end("Bad Request");
  }
});

const PORT = 3000;
server.listen(PORT, () => {
  console.log(`Server running at http://localhost:${PORT}/`);
});