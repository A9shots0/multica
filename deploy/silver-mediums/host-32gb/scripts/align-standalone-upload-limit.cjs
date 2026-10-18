// A config-only overlay for an already-built Next standalone image. Preserve
// every route/asset/build option; require the observed 10 MiB default before
// changing the one bounded proxy setting to match Go's multipart request cap.
const fs = require('node:fs');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const file = process.argv[2];
if (!file) throw new Error('Pass the standalone server.js path');
const source = fs.readFileSync(file, 'utf8');
const pattern = /const nextConfig = (.+)\n/;
const match = source.match(pattern);
assert(match, 'Expected generated Next standalone config');
const before = JSON.parse(match[1]);
assert.equal(before.experimental.proxyClientMaxBodySize, 10 * 1024 * 1024);
const after = structuredClone(before);
after.experimental.proxyClientMaxBodySize = 100 * 1024 * 1024;
const updated = source.replace(pattern, 'const nextConfig = ' + JSON.stringify(after) + '\n');
assert.equal(updated.replace(JSON.stringify(after), match[1]), source,
  'Only the embedded config may change');
fs.writeFileSync(file, updated);
console.log(JSON.stringify({
  before: crypto.createHash('sha256').update(source).digest('hex'),
  after: crypto.createHash('sha256').update(updated).digest('hex'),
  proxyClientMaxBodySize: after.experimental.proxyClientMaxBodySize,
}));
