import * as esbuild from 'esbuild';
import { watch } from 'node:fs';

const isWatch = process.argv.includes('--watch');

const extensionOptions = {
  entryPoints: ['src/extension.ts'],
  bundle: true,
  platform: 'node',
  format: 'cjs',
  target: 'node20',
  outfile: 'dist/extension.js',
  external: ['vscode'],
  sourcemap: true,
  minify: !isWatch,
  logLevel: 'info',
};

const webviewOptions = {
  entryPoints: ['src/webview/main.tsx'],
  bundle: true,
  platform: 'browser',
  format: 'iife',
  target: 'es2022',
  outfile: 'dist/webview.js',
  sourcemap: true,
  minify: !isWatch,
  logLevel: 'info',
};

if (isWatch) {
  const ctx1 = await esbuild.context(extensionOptions);
  const ctx2 = await esbuild.context(webviewOptions);
  await Promise.all([ctx1.watch(), ctx2.watch()]);
  console.log('watching...');
} else {
  await Promise.all([esbuild.build(extensionOptions), esbuild.build(webviewOptions)]);
}
