import { describe, expect, it } from 'vitest';
import { detectProvider, parseRemoteUrl, providerForHost, providerLabel } from '../../src/integrations/gitRemote';

describe('parseRemoteUrl', () => {
  it('parses https remotes for GitHub, Bitbucket and GitLab', () => {
    expect(parseRemoteUrl('https://github.com/peggco/pegg.git')).toMatchObject({
      host: 'github.com',
      owner: 'peggco',
      repo: 'pegg',
      protocol: 'https',
      provider: 'github',
    });
    expect(parseRemoteUrl('https://bitbucket.org/team/repo.git')).toMatchObject({
      host: 'bitbucket.org',
      owner: 'team',
      repo: 'repo',
      provider: 'bitbucket',
    });
    expect(parseRemoteUrl('https://gitlab.com/group/sub/repo.git')).toMatchObject({
      owner: 'group/sub',
      repo: 'repo',
      provider: 'gitlab',
    });
  });

  it('parses scp-style ssh remotes', () => {
    expect(parseRemoteUrl('git@github.com:peggco/pegg.git')).toMatchObject({
      host: 'github.com',
      owner: 'peggco',
      repo: 'pegg',
      protocol: 'ssh',
      provider: 'github',
    });
    expect(parseRemoteUrl('git@bitbucket.org:team/repo.git')).toMatchObject({
      provider: 'bitbucket',
    });
    expect(parseRemoteUrl('git@gitlab.com:group/repo.git')).toMatchObject({
      provider: 'gitlab',
    });
  });

  it('parses ssh:// remotes with ports', () => {
    expect(parseRemoteUrl('ssh://git@gitlab.example.com:2222/group/repo.git')).toMatchObject({
      host: 'gitlab.example.com',
      owner: 'group',
      repo: 'repo',
      protocol: 'ssh',
      provider: 'unknown',
    });
  });

  it('strips credentials and trailing slashes', () => {
    expect(parseRemoteUrl('https://user:token@github.com/peggco/pegg/')).toMatchObject({
      host: 'github.com',
      owner: 'peggco',
      repo: 'pegg',
      provider: 'github',
    });
  });

  it('applies a provider override for self-hosted hosts', () => {
    expect(parseRemoteUrl('https://git.company.dev/team/repo.git', 'gitlab')).toMatchObject({
      host: 'git.company.dev',
      provider: 'gitlab',
    });
    expect(parseRemoteUrl('git@git.company.dev:team/repo.git', 'bitbucket')).toMatchObject({
      provider: 'bitbucket',
    });
  });

  it('handles local paths and invalid input', () => {
    expect(parseRemoteUrl('/tmp/repo.git')).toMatchObject({
      owner: 'tmp',
      repo: 'repo',
      protocol: 'file',
      provider: 'unknown',
    });
    expect(parseRemoteUrl('not a url')).toBeUndefined();
    expect(parseRemoteUrl('   ')).toBeUndefined();
  });
});

describe('provider detection', () => {
  it('maps known hosts', () => {
    expect(providerForHost('GitHub.com')).toBe('github');
    expect(providerForHost('bitbucket.org')).toBe('bitbucket');
    expect(providerForHost('gitlab.com')).toBe('gitlab');
    expect(providerForHost('example.com')).toBe('unknown');
  });

  it('prefers a valid override', () => {
    expect(detectProvider('github.com', 'gitlab')).toBe('gitlab');
    expect(detectProvider('example.com', 'nonsense')).toBe('unknown');
  });

  it('labels providers', () => {
    expect(providerLabel('github')).toBe('GitHub');
    expect(providerLabel('bitbucket')).toBe('Bitbucket');
    expect(providerLabel('gitlab')).toBe('GitLab');
    expect(providerLabel('unknown')).toBe('Unknown');
  });
});
