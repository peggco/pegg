export interface Target {
  os: 'linux' | 'darwin' | 'windows';
  arch: 'amd64' | 'arm64';
  asset: string;
  binary: string;
}

export function detectTarget(platform = process.platform, arch = process.arch): Target {
  let os: Target['os'];
  switch (platform) {
    case 'linux':
      os = 'linux';
      break;
    case 'darwin':
      os = 'darwin';
      break;
    case 'win32':
      os = 'windows';
      break;
    default:
      throw new Error(`Unsupported operating system: ${platform}`);
  }

  let cpu: Target['arch'];
  switch (arch) {
    case 'x64':
      cpu = 'amd64';
      break;
    case 'arm64':
      cpu = 'arm64';
      break;
    default:
      throw new Error(`Unsupported architecture: ${arch}`);
  }

  const ext = os === 'windows' ? 'zip' : 'tar.gz';
  return {
    os,
    arch: cpu,
    asset: `pegg_${os}_${cpu}.${ext}`,
    binary: os === 'windows' ? 'pegg.exe' : 'pegg',
  };
}
