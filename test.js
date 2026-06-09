const { exec } = require('child_process');

// Run the built binary
const child = exec('./out', { env: { ...process.env, PORT: '3000' } });

child.stdout.on('data', console.log);
child.stderr.on('data', console.error);

setTimeout(() => {
    fetch('http://127.0.0.1:3000/api/test_error')
      .then(r => r.text())
      .then(t => { console.log("Result:", t); child.kill(); process.exit(); })
      .catch(e => { console.error("Fetch err:", e.message); child.kill(); process.exit(1); });
}, 5000);
