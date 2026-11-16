package dev.pegg.intellij.git

enum class GitProvider(val key: String) {
    GITHUB("github"),
    BITBUCKET("bitbucket"),
    GITLAB("gitlab"),
    UNKNOWN("unknown"),
    ;

    val label: String
        get() = when (this) {
            GITHUB -> "GitHub"
            BITBUCKET -> "Bitbucket"
            GITLAB -> "GitLab"
            UNKNOWN -> "Unknown"
        }

    companion object {
        fun from(value: String?): GitProvider {
            val wanted = value?.trim()?.lowercase().orEmpty()
            return entries.firstOrNull { it.key == wanted } ?: UNKNOWN
        }
    }
}

data class RemoteInfo(
    val host: String,
    val owner: String,
    val repo: String,
    val protocol: String,
    val provider: GitProvider,
)

object GitRemote {
    private val knownHosts = mapOf(
        "github.com" to GitProvider.GITHUB,
        "www.github.com" to GitProvider.GITHUB,
        "bitbucket.org" to GitProvider.BITBUCKET,
        "www.bitbucket.org" to GitProvider.BITBUCKET,
        "gitlab.com" to GitProvider.GITLAB,
        "www.gitlab.com" to GitProvider.GITLAB,
    )

    private val scpPattern = Regex("^(?:[^@/]+@)?([^:/]+):(?!//)(.+)$")

    fun providerForHost(host: String): GitProvider =
        knownHosts[host.trim().lowercase()] ?: GitProvider.UNKNOWN

    fun detectProvider(host: String, override: String?): GitProvider {
        val forced = GitProvider.from(override)
        return if (forced != GitProvider.UNKNOWN) forced else providerForHost(host)
    }

    private fun splitPath(path: String): Pair<String, String>? {
        val cleaned = path
            .replace(Regex("^/+"), "")
            .replace(Regex("\\.git$", RegexOption.IGNORE_CASE), "")
            .replace(Regex("/+$"), "")
        val parts = cleaned.split('/').filter { it.isNotBlank() }
        if (parts.size < 2) return null
        return parts.dropLast(1).joinToString("/") to parts.last()
    }

    fun parse(url: String, override: String? = null): RemoteInfo? {
        val value = url.trim()
        if (value.isEmpty()) return null

        if (!value.contains("://")) {
            val scp = scpPattern.matchEntire(value)
            if (scp != null) {
                val path = splitPath(scp.groupValues[2]) ?: return null
                val host = scp.groupValues[1].lowercase()
                return RemoteInfo(host, path.first, path.second, "ssh", detectProvider(host, override))
            }
            if (value.startsWith("/")) {
                val path = splitPath(value) ?: return null
                return RemoteInfo("localhost", path.first, path.second, "file", detectProvider("localhost", override))
            }
            return null
        }

        val uri = runCatching { java.net.URI(value) }.getOrNull() ?: return null
        if (uri.scheme == "file") {
            val path = splitPath(uri.path ?: "") ?: return null
            return RemoteInfo("localhost", path.first, path.second, "file", detectProvider("localhost", override))
        }
        val host = uri.host?.lowercase() ?: return null
        val path = splitPath(uri.path ?: "") ?: return null
        val protocol = when (uri.scheme?.lowercase()) {
            "http", "https" -> "https"
            "ssh", "git+ssh" -> "ssh"
            else -> "unknown"
        }
        return RemoteInfo(host, path.first, path.second, protocol, detectProvider(host, override))
    }
}
