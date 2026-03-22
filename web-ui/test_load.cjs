const http = require('http');

const req = http.request('http://localhost:8080/api/projects/load', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' }
}, (res) => {
  let data = '';
  res.on('data', (c) => data += c);
  res.on('end', () => console.log("DATA:", data.slice(0, 500)));
});
req.write(JSON.stringify({ threadId: 'proj-1774143057571' }));
req.end();
