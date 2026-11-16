package dev.pegg.intellij.git

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.util.concurrent.TimeUnit

@Serializable
data class CardIssue(
    val provider: String? = null,
    val repo: String? = null,
    val number: Int? = null,
    val iid: Int? = null,
    val url: String? = null,
)

@Serializable
data class CardContext(
    val branch: String? = null,
    val base: String? = null,
)

@Serializable
data class KanbanCard(
    val id: String? = null,
    val title: String = "",
    val column: String? = null,
    val issue: CardIssue? = null,
    val context: CardContext? = null,
    val branch: String? = null,
    val base: String? = null,
)

enum class DirtyChoice { STASH, COMMIT, CANCEL }

data class CheckoutOptions(
    val cwd: String,
    val remote: String = "origin",
    val providerOverride: String = "",
    val branchTemplate: String = "{number}-{slug}",
    val autoPush: Boolean = false,
    val confirmDirty: (List<String>) -> DirtyChoice = { DirtyChoice.CANCEL },
    val onProgress: (String) -> Unit = {},
)

data class CheckoutResult(
    val branch: String,
    val provider: GitProvider,
    val remote: String,
    val base: String?,
    val created: Boolean,
    val pushed: Boolean,
    val stashed: Boolean,
    val committed: Boolean,
    val alreadyOnBranch: Boolean,
)

class CardCheckoutException(message: String, val exitCode: Int? = null) : RuntimeException(message)

object CardCheckout {
    private const val GIT_TIMEOUT_SECONDS = 60L

    private val json = Json {
        ignoreUnknownKeys = true
        isLenient = true
    }

    @Serializable
    private data class BoardFile(
        val cards: List<KanbanCard> = emptyList(),
        val columns: List<BoardColumn> = emptyList(),
    )

    @Serializable
    private data class BoardColumn(val cards: List<KanbanCard> = emptyList())

    fun loadCards(root: File): List<KanbanCard> {
        val file = File(File(File(root, ".pegg"), "kanban"), "board.json")
        if (!file.isFile) return emptyList()
        val board = runCatching { json.decodeFromString<BoardFile>(file.readText()) }
            .getOrElse { throw CardCheckoutException("Could not parse ${file.path}. Fix the board file and retry.") }
        return (board.cards + board.columns.flatMap { it.cards })
            .filter { it.title.isNotBlank() }
            .sortedWith(compareBy({ it.issue?.number ?: it.issue?.iid ?: Int.MAX_VALUE }, { it.title }))
    }

    fun cardBranch(card: KanbanCard, options: CheckoutOptions): String {
        val remote = options.remote.ifBlank { "origin" }
        val explicit = card.context?.branch ?: card.branch
        if (!explicit.isNullOrBlank()) return normalizeBranch(explicit, remote)
        return BranchName.forCard(options.branchTemplate, card)
    }

    fun checkout(card: KanbanCard, options: CheckoutOptions): CheckoutResult {
        val hint = options.onProgress
        val remote = options.remote.ifBlank { "origin" }
        val cwd = options.cwd

        val inside = git(cwd, listOf("rev-parse", "--is-inside-work-tree"), GitProvider.UNKNOWN)
        if (inside != "true") throw CardCheckoutException("The workspace is not a git repository.")

        val remoteUrl = try {
            git(cwd, listOf("remote", "get-url", remote), GitProvider.UNKNOWN)
        } catch (_: CardCheckoutException) {
            throw CardCheckoutException(
                "No git remote named '$remote' in this repository. Set the Pegg git remote option or add the remote.",
            )
        }
        val parsed = GitRemote.parse(remoteUrl, options.providerOverride)
        val provider = parsed?.provider ?: GitProvider.from(options.providerOverride)
        if (provider == GitProvider.UNKNOWN) {
            val host = parsed?.let { " (${it.host})" }.orEmpty()
            throw CardCheckoutException(
                "Could not detect the hosting provider$host. Set the Pegg git provider to github, bitbucket, or gitlab.",
            )
        }

        val branch = cardBranch(card, options)
        if (branch.isBlank()) throw CardCheckoutException("Could not derive a branch name for this card.")
        hint("Checking branch $branch")
        git(cwd, listOf("check-ref-format", "--branch", branch), provider)

        val current = runCatching { git(cwd, listOf("rev-parse", "--abbrev-ref", "HEAD"), provider) }
            .getOrDefault("")
        if (current == branch) {
            hint("Already on $branch")
            return CheckoutResult(
                branch = branch,
                provider = provider,
                remote = remote,
                base = card.context?.base ?: card.base,
                created = false,
                pushed = false,
                stashed = false,
                committed = false,
                alreadyOnBranch = true,
            )
        }

        var stashed = false
        var committed = false
        val dirty = dirtyFiles(cwd, provider)
        if (dirty.isNotEmpty()) {
            when (options.confirmDirty(dirty)) {
                DirtyChoice.CANCEL -> throw CardCheckoutException(
                    "Checkout cancelled because the working tree has uncommitted changes.",
                )
                DirtyChoice.STASH -> {
                    hint("Stashing local changes")
                    git(
                        cwd,
                        listOf("stash", "push", "--include-untracked", "-m", "pegg: changes before checking out $branch"),
                        provider,
                    )
                    stashed = true
                }
                DirtyChoice.COMMIT -> {
                    hint("Committing local changes")
                    git(cwd, listOf("add", "-A"), provider)
                    git(cwd, listOf("commit", "-m", "wip: changes before checking out $branch"), provider)
                    committed = true
                }
            }
        }

        if (refExists(cwd, "refs/heads/$branch", provider)) {
            hint("Checking out $branch")
            git(cwd, listOf("checkout", branch), provider)
            return CheckoutResult(
                branch, provider, remote, card.context?.base ?: card.base,
                created = false, pushed = false, stashed = stashed, committed = committed,
                alreadyOnBranch = false,
            )
        }

        if (remoteBranchExists(cwd, remote, branch, provider)) {
            hint("Fetching $remote/$branch")
            git(cwd, listOf("fetch", remote, "refs/heads/$branch:refs/remotes/$remote/$branch"), provider)
            hint("Checking out $branch")
            git(cwd, listOf("checkout", "-b", branch, "--track", "$remote/$branch"), provider)
            return CheckoutResult(
                branch, provider, remote, card.context?.base ?: card.base,
                created = false, pushed = false, stashed = stashed, committed = committed,
                alreadyOnBranch = false,
            )
        }

        val base = remoteDefaultBranch(cwd, remote, provider)
        val startPoint = if (committed) "HEAD" else "$remote/$base"
        hint("Creating $branch from $startPoint")
        if (!committed && !refExists(cwd, "refs/remotes/$remote/$base", provider)) {
            git(cwd, listOf("fetch", remote, "refs/heads/$base:refs/remotes/$remote/$base"), provider)
        }
        git(cwd, listOf("checkout", "-b", branch, startPoint), provider)

        var pushed = false
        if (options.autoPush) {
            hint("Pushing $branch to $remote")
            git(cwd, listOf("push", "-u", remote, branch), provider)
            pushed = true
        }
        return CheckoutResult(
            branch, provider, remote, base,
            created = true, pushed = pushed, stashed = stashed, committed = committed,
            alreadyOnBranch = false,
        )
    }

    private fun normalizeBranch(branch: String, remote: String): String {
        val withoutRefs = branch.trim().removePrefix("refs/heads/")
        return if (withoutRefs.startsWith("$remote/")) withoutRefs.removePrefix("$remote/") else withoutRefs
    }

    private fun firstLine(text: String): String =
        text.lines().map { it.trim() }.filter { it.isNotEmpty() }.takeLast(3).joinToString(" ")

    private fun actionable(message: String, provider: GitProvider): String {
        val text = message.ifBlank { "git command failed." }
        return when {
            Regex(
                "could not resolve host|unable to access|connection timed out|failed to connect|network is unreachable",
                RegexOption.IGNORE_CASE,
            ).containsMatchIn(text) ->
                "Could not reach the git remote. Check your network connection and retry."
            Regex(
                "authentication failed|could not read username|permission denied \\(publickey\\)|invalid username or password|terminal prompts disabled|returned error: 40[13]",
                RegexOption.IGNORE_CASE,
            ).containsMatchIn(text) ->
                "The ${provider.label} remote rejected authentication. Sign in and retry."
            Regex("would be overwritten by checkout|local changes", RegexOption.IGNORE_CASE).containsMatchIn(text) ->
                "Local changes would be overwritten by the checkout. Stash or commit them first."
            Regex("already exists", RegexOption.IGNORE_CASE).containsMatchIn(text) ->
                "A local branch with that name already exists. Pick another branch name."
            Regex("not a git repository", RegexOption.IGNORE_CASE).containsMatchIn(text) ->
                "The workspace is not a git repository."
            else -> text
        }
    }

    private fun git(cwd: String, args: List<String>, provider: GitProvider): String {
        val process = runCatching {
            ProcessBuilder(listOf("git") + args)
                .directory(File(cwd))
                .redirectErrorStream(true)
                .apply { environment()["GIT_TERMINAL_PROMPT"] = "0" }
                .start()
        }.getOrElse {
            throw CardCheckoutException("Could not run git. Make sure it is installed and on PATH.")
        }
        val output = StringBuilder()
        val reader = Thread {
            process.inputStream.bufferedReader(Charsets.UTF_8).use { output.append(it.readText()) }
        }.apply {
            isDaemon = true
            start()
        }
        if (!process.waitFor(GIT_TIMEOUT_SECONDS, TimeUnit.SECONDS)) {
            process.destroyForcibly()
            throw CardCheckoutException("git ${args.firstOrNull().orEmpty()} timed out.")
        }
        reader.join(1000)
        val text = output.toString().trim()
        if (process.exitValue() != 0) {
            throw CardCheckoutException(actionable(firstLine(text), provider), process.exitValue())
        }
        return text
    }

    private fun refExists(cwd: String, ref: String, provider: GitProvider): Boolean =
        try {
            git(cwd, listOf("show-ref", "--verify", "--quiet", ref), provider)
            true
        } catch (e: CardCheckoutException) {
            if (e.exitCode == 1 || e.exitCode == 128) false else throw e
        }

    private fun remoteBranchExists(cwd: String, remote: String, branch: String, provider: GitProvider): Boolean =
        try {
            git(cwd, listOf("ls-remote", "--exit-code", "--heads", remote, "refs/heads/$branch"), provider)
            true
        } catch (e: CardCheckoutException) {
            if (e.exitCode == 2) false else throw e
        }

    private fun remoteDefaultBranch(cwd: String, remote: String, provider: GitProvider): String {
        val symbolic = runCatching {
            git(cwd, listOf("symbolic-ref", "--short", "refs/remotes/$remote/HEAD"), provider)
        }.getOrNull()
        if (!symbolic.isNullOrBlank()) return normalizeBranch(symbolic, remote)
        val remoteHead = runCatching {
            git(cwd, listOf("ls-remote", "--symref", remote, "HEAD"), provider)
        }.getOrNull()
        val match = remoteHead?.let { Regex("^ref:\\s+refs/heads/(\\S+)\\s+HEAD", RegexOption.MULTILINE).find(it) }
        return match?.groupValues?.getOrNull(1) ?: "main"
    }

    private fun dirtyFiles(cwd: String, provider: GitProvider): List<String> =
        git(cwd, listOf("status", "--porcelain"), provider)
            .lines()
            .map { it.drop(3).trim() }
            .filter { it.isNotEmpty() }
}
