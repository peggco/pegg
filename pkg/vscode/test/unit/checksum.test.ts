import { describe, expect, it } from 'vitest';
import { parseChecksums } from '../../src/binary/checksum';

describe('checksums', () => {
  it('parses standard sha256 checksum files', () => {
    const parsed = parseChecksums(
      [
        '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  pegg_linux_amd64.tar.gz',
        'ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789 *pegg_windows_amd64.zip',
        '',
        'not a checksum',
      ].join('\n'),
    );
    expect(parsed.get('pegg_linux_amd64.tar.gz')).toBe(
      '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
    );
    expect(parsed.get('pegg_windows_amd64.zip')).toBe(
      'abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789',
    );
  });
});
