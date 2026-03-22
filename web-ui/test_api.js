const http = require('http');

http.get('http://localhost:8080/api/projects', (res) => {
  let data = '';
  res.on('data', (chunk) => {
    data += chunk;
  });
  res.on('end', () => {
    console.log("RESPONSE:", data);
  });
}).on("error", (err) => {
  console.log("Error: " + err.message);
});
