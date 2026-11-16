package dev.pegg.intellij.git

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class GitRemoteTest {
    @Test
    fun `parses https remotes for all providers`() {
        val github = GitRemote.parse("https://github.com/peggco/pegg.git")
        assertEquals("github.com", github?.host)
        assertEquals("peggco", github?.owner)
        assertEquals("pegg", github?.repo)
        assertEquals("https", github?.protocol)
        assertEquals(GitProvider.GITHUB, github?.provider)

        assertEquals(GitProvider.BITBUCKET, GitRemote.parse("https://bitbucket.org/team/repo.git")?.provider)
        val gitlab = GitRemote.parse("https://gitlab.com/group/sub/repo.git")
        assertEquals("group/sub", gitlab?.owner)
        assertEquals(GitProvider.GITLAB, gitlab?.provider)
    }

    @Test
    fun `parses scp-style ssh remotes`() {
        val info = GitRemote.parse("git@github.com:peggco/pegg.git")
        assertEquals("github.com", info?.host)
        assertEquals("peggco", info?.owner)
        assertEquals("pegg", info?.repo)
        assertEquals("ssh", info?.protocol)
        assertEquals(GitProvider.GITHUB, info?.provider)
        assertEquals(GitProvider.BITBUCKET, GitRemote.parse("git@bitbucket.org:team/repo.git")?.provider)
        assertEquals(GitProvider.GITLAB, GitRemote.parse("git@gitlab.com:group/repo.git")?.provider)
    }

    @Test
    fun `parses ssh urls with ports`() {
        val info = GitRemote.parse("ssh://git@gitlab.example.com:2222/group/repo.git")
        assertEquals("gitlab.example.com", info?.host)
        assertEquals("ssh", info?.protocol)
        assertEquals(GitProvider.UNKNOWN, info?.provider)
    }

    @Test
    fun `strips credentials and trailing slashes`() {
        val info = GitRemote.parse("https://user:token@github.com/peggco/pegg/")
        assertEquals("github.com", info?.host)
        assertEquals("pegg", info?.repo)
        assertEquals(GitProvider.GITHUB, info?.provider)
    }

    @Test
    fun `applies a provider override for self-hosted hosts`() {
        assertEquals(GitProvider.GITLAB, GitRemote.parse("https://git.company.dev/team/repo.git", "gitlab")?.provider)
        assertEquals(GitProvider.BITBUCKET, GitRemote.parse("git@git.company.dev:team/repo.git", "bitbucket")?.provider)
    }

    @Test
    fun `handles local paths and invalid input`() {
        val local = GitRemote.parse("/tmp/repo.git")
        assertEquals("tmp", local?.owner)
        assertEquals("repo", local?.repo)
        assertEquals("file", local?.protocol)
        assertNull(GitRemote.parse("not a url"))
        assertNull(GitRemote.parse("   "))
    }

    @Test
    fun `detects providers and labels them`() {
        assertEquals(GitProvider.GITHUB, GitRemote.providerForHost("GitHub.com"))
        assertEquals(GitProvider.UNKNOWN, GitRemote.providerForHost("example.com"))
        assertEquals(GitProvider.GITLAB, GitRemote.detectProvider("github.com", "gitlab"))
        assertEquals(GitProvider.UNKNOWN, GitRemote.detectProvider("example.com", "nonsense"))
        assertEquals("GitHub", GitProvider.GITHUB.label)
        assertEquals("Bitbucket", GitProvider.BITBUCKET.label)
        assertEquals("GitLab", GitProvider.GITLAB.label)
        assertTrue(GitProvider.UNKNOWN.label.isNotEmpty())
    }
}
