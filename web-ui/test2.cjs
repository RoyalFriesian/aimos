const http = require('http');

http.get('http://localhost:8080/api/projects', {
  headers: {}
}, (res) => {
  console.log("STATUS:", res.statusCode);
  let data = '';
  res.on('data', (c) => data += c);
  res.on('end', () => console.log("DATA:", data.slice(0, 100)));
});
