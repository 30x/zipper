var http = require('http');
var os = require('os');


console.log(os.networkInterfaces())
console.log(os.networkInterfaces()['eth0'][0].address);
var sub = os.networkInterfaces()['eth0'][0].address.split('.');
sub[3] = 1;
var nodeIp = sub.join('.');
var opts = {
  host: nodeIp,
  port: 80,
  path: '/example',
  headers: {
    Host: 'centralitews.k8s.dev'
  }
};

setInterval(function() {
  http.get(opts, function(res) {
    console.log(res.statusCode);
  }).on('error', console.error)
}, 1000);

var idx = Math.round(Math.random()*1000);
http.createServer(function(req, res) {
  console.log(req.method, req.url, req.headers)
  req.on('data', function() {});
  req.on('end', function() {
    res.statusCode = 200;
    res.end('Test ' + new Date().getTime() + ' ' + idx);
  });
}).listen(process.env.PORT || 3000);
