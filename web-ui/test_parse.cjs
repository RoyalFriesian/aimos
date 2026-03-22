const http = require('http');

http.get('http://localhost:8080/api/projects', (res) => {
  let data = '';
  res.on('data', (c) => data += c);
  res.on('end', () => console.log("DATA KEYS:", Object.keys(JSON.parse(data).projects[0])));
});
