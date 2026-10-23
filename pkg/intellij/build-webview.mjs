import * as esbuild from 'esbuild';
import { copyFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const outdir = resolve(here, 'build/webview');
const isWatch = process.argv.includes('--watch');

mkdirSync(outdir, { recursive: true });
copyFileSync(resolve(here, 'webview/index.html'), resolve(outdir, 'index.html'));

const options = {
  entryPoints: {
    bootstrap: resolve(here, 'webview/bootstrap.ts'),
    webview: resolve(here, 'webview/main.tsx'),
  },
  bundle: true,
  platform: 'browser',
  format: 'iife',
  target: 'es2022',
  jsx: 'automatic',
  outdir,
  sourcemap: true,
  minify: !isWatch,
  logLevel: 'info',
  loader: { '.ts': 'ts', '.tsx': 'tsx' },
};

if (isWatch) {
  const ctx = await esbuild.context(options);
  await ctx.watch();
  console.log('watching intellij webview...');
} else {
  await esbuild.build(options);
}
