import { spawn } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const rootDir = path.dirname(fileURLToPath(import.meta.url));
const installer = path.join(rootDir, 'plugins', 'adhoc', 'install.mjs');
const child = spawn(process.execPath, [installer, ...process.argv.slice(2)], { stdio: 'inherit' });

child.on('exit', (code) => {
  process.exit(code ?? 1);
});
