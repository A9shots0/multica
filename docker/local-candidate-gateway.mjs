// Fixed ingress for an internal-only application network. No arbitrary proxy
// destinations, application configuration, request logging or credentials.
import net from 'node:net';

for (const [port, host] of [[3000, 'frontend'], [8080, 'backend'], [5432, 'postgres']]) {
  const server = net.createServer(client => {
    const upstream = net.connect({ host, port });
    client.on('error', () => upstream.destroy());
    upstream.on('error', () => client.destroy());
    client.on('close', () => upstream.destroy());
    client.pipe(upstream).pipe(client);
  });
  server.on('error', error => { console.error(error.code); process.exit(1); });
  server.listen(port, '0.0.0.0');
}
