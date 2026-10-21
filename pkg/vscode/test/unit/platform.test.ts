import { describe, expect, it } from 'vitest';
import { detectTarget } from '../../src/binary/platform';

describe('binary target detection', () => {
  it('maps linux x64', () => {
    expect(detectTarget('linux', 'x64')).toEqual({
      os: 'linux',
      arch: 'amd64',
      asset: 'pegg_linux_amd64.tar.gz',
      binary: 'pegg',
    });
  });

  it('maps darwin arm64', () => {
    expect(detectTarget('darwin', 'arm64').asset).toBe('pegg_darwin_arm64.tar.gz');
  });

  it('maps windows x64', () => {
    expect(detectTarget('win32', 'x64')).toMatchObject({
      asset: 'pegg_windows_amd64.zip',
      binary: 'pegg.exe',
    });
  });

  it('rejects unsupported platforms', () => {
    expect(() => detectTarget('freebsd', 'x64')).toThrow(/Unsupported operating system/);
    expect(() => detectTarget('linux', 'ia32')).toThrow(/Unsupported architecture/);
  });
});
