package dev.pegg.intellij.git

import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class CardCheckoutTest {
    private data class Fixture(val root: File, val repo: File, val remote: File)

    private fun withRepo(block: (Fixture) -> Unit) {
        val root = Files.createTempDirectory("pegg-card-").toFile()
        try {
            val remote = File(root, "remote.git")
            val repo = File(root, "repo")
            run(root, "init", "--bare", remote.path)
            run(remote, "symbolic-ref", "HEAD", "refs/heads/main")
            Files.createDirectories(repo.toPath())
            run(repo, "init")
            run(repo, "symbolic-ref", "HEAD", "refs/heads/main")
            File(repo, "README.md").writeText("hello\n")
            run(repo, "add", ".")
            run(repo, "commit", "-m", "initial")
            run(repo, "remote", "add", "origin", remote.path)
            run(repo, "push", "-u", "origin", "main")
            block(Fixture(root, repo, remote))
        } finally {
            root.deleteRecursively()
        }
    }

    private fun run(cwd: File, vararg args: String): String {
        val process = ProcessBuilder(listOf("git") + args)
            .directory(cwd)
            .redirectErrorStream(true)
            .apply {
                environment()["GIT_AUTHOR_NAME"] = "Pegg Test"
                environment()["GIT_AUTHOR_EMAIL"] = "test@example.com"
                environment()["GIT_COMMITTER_NAME"] = "Pegg Test"
                environment()["GIT_COMMITTER_EMAIL"] = "test@example.com"
                environment()["GIT_TERMINAL_PROMPT"] = "0"
            }
            .start()
        val output = process.inputStream.bufferedReader().use { it.readText() }.trim()
        check(process.waitFor() == 0) { "git ${args.joinToString(" ")} failed: $output" }
        return output
    }

    private fun options(
        repo: File,
        providerOverride: String = "github",
        autoPush: Boolean = false,
        remote: String = "origin",
        confirmDirty: (List<String>) -> DirtyChoice = { DirtyChoice.CANCEL },
    ) = CheckoutOptions(
        cwd = repo.path,
        remote = remote,
        providerOverride = providerOverride,
        branchTemplate = "{number}-{slug}",
        autoPush = autoPush,
        confirmDirty = confirmDirty,
    )

    @Test
    fun `prefers the card branch and normalizes prefixes`() {
        val opts = options(File("."))
        assertEquals(
            "pegg/fix-login",
            CardCheckout.cardBranch(
                KanbanCard(title = "Fix login", issue = CardIssue(number = 42), context = CardContext(branch = "pegg/fix-login")),
                opts,
            ),
        )
        assertEquals(
            "feature/x",
            CardCheckout.cardBranch(KanbanCard(title = "Fix login", branch = "refs/heads/feature/x"), opts),
        )
        assertEquals(
            "feature/x",
            CardCheckout.cardBranch(KanbanCard(title = "Fix login", branch = "origin/feature/x"), opts),
        )
        assertEquals(
            "42-fix-login-redirect",
            CardCheckout.cardBranch(
                KanbanCard(title = "[BUG] Fix login redirect", issue = CardIssue(number = 42)),
                opts,
            ),
        )
        assertEquals(
            "issue/9-fix-login",
            CardCheckout.cardBranch(
                KanbanCard(title = "Fix login", issue = CardIssue(iid = 9)),
                opts.copy(branchTemplate = "issue/{number}-{slug}"),
            ),
        )
    }

    @Test
    fun `loads and sorts board cards`() {
        withRepo { fx ->
            assertTrue(CardCheckout.loadCards(fx.root).isEmpty())
            val board = File(File(File(fx.root, ".pegg"), "kanban"), "board.json")
            board.parentFile.mkdirs()
            board.writeText(
                """
                {"cards":[
                  {"id":"b","title":"Second","issue":{"number":2}},
                  {"id":"a","title":"First","issue":{"number":1},"context":{"branch":"pegg/first"}},
                  {"id":"c","title":"No issue"}
                ]}
                """.trimIndent(),
            )
            val cards = CardCheckout.loadCards(fx.root)
            assertEquals(listOf("First", "Second", "No issue"), cards.map { it.title })
            assertEquals("pegg/first", cards[0].context?.branch)
        }
    }

    @Test
    fun `reports malformed boards`() {
        withRepo { fx ->
            val board = File(File(File(fx.root, ".pegg"), "kanban"), "board.json")
            board.parentFile.mkdirs()
            board.writeText("{not json")
            assertFailsWith<CardCheckoutException> { CardCheckout.loadCards(fx.root) }
        }
    }

    @Test
    fun `checks out an existing remote branch with tracking`() {
        withRepo { fx ->
            run(fx.repo, "branch", "42-remote")
            run(fx.repo, "push", "origin", "42-remote")
            run(fx.repo, "branch", "-D", "42-remote")

            val result = CardCheckout.checkout(KanbanCard(title = "Remote", issue = CardIssue(number = 42)), options(fx.repo))

            assertEquals("42-remote", result.branch)
            assertFalse(result.created)
            assertEquals(GitProvider.GITHUB, result.provider)
            assertEquals("42-remote", run(fx.repo, "rev-parse", "--abbrev-ref", "HEAD"))
            assertEquals("origin/42-remote", run(fx.repo, "rev-parse", "--abbrev-ref", "42-remote@{upstream}"))
        }
    }

    @Test
    fun `checks out a local-only branch`() {
        withRepo { fx ->
            run(fx.repo, "branch", "43-local")
            val result = CardCheckout.checkout(KanbanCard(title = "Local", issue = CardIssue(number = 43)), options(fx.repo))

            assertFalse(result.created)
            assertEquals("43-local", run(fx.repo, "rev-parse", "--abbrev-ref", "HEAD"))
            assertFailsWith<IllegalStateException> { run(fx.remote, "rev-parse", "--verify", "refs/heads/43-local") }
        }
    }

    @Test
    fun `creates a missing branch from the remote default`() {
        withRepo { fx ->
            val result = CardCheckout.checkout(
                KanbanCard(title = "Brand new thing", issue = CardIssue(number = 44)),
                options(fx.repo),
            )

            assertEquals("44-brand-new-thing", result.branch)
            assertEquals("main", result.base)
            assertTrue(result.created)
            assertEquals("44-brand-new-thing", run(fx.repo, "rev-parse", "--abbrev-ref", "HEAD"))
            assertEquals(run(fx.repo, "rev-parse", "origin/main"), run(fx.repo, "rev-parse", "HEAD"))
        }
    }

    @Test
    fun `pushes a new branch when auto-push is enabled`() {
        withRepo { fx ->
            val result = CardCheckout.checkout(
                KanbanCard(title = "Push me", issue = CardIssue(number = 45)),
                options(fx.repo, autoPush = true),
            )

            assertTrue(result.created)
            assertTrue(result.pushed)
            run(fx.remote, "rev-parse", "--verify", "refs/heads/45-push-me")
        }
    }

    @Test
    fun `is a no-op when already on the branch`() {
        withRepo { fx ->
            val result = CardCheckout.checkout(
                KanbanCard(title = "Already here", issue = CardIssue(number = 46), context = CardContext(branch = "main")),
                options(fx.repo),
            )

            assertTrue(result.alreadyOnBranch)
            assertEquals("main", run(fx.repo, "rev-parse", "--abbrev-ref", "HEAD"))
        }
    }

    @Test
    fun `refuses dirty worktrees when the user cancels`() {
        withRepo { fx ->
            File(fx.repo, "README.md").writeText("changed\n")
            val error = assertFailsWith<CardCheckoutException> {
                CardCheckout.checkout(
                    KanbanCard(title = "Dirty", issue = CardIssue(number = 47)),
                    options(fx.repo) { DirtyChoice.CANCEL },
                )
            }
            assertTrue(error.message.orEmpty().contains("uncommitted changes", ignoreCase = true))
            assertTrue(run(fx.repo, "status", "--porcelain").isNotEmpty())
        }
    }

    @Test
    fun `stashes a dirty worktree when asked`() {
        withRepo { fx ->
            File(fx.repo, "README.md").writeText("changed\n")
            val result = CardCheckout.checkout(
                KanbanCard(title = "Dirty", issue = CardIssue(number = 48)),
                options(fx.repo) { DirtyChoice.STASH },
            )

            assertTrue(result.stashed)
            assertEquals("", run(fx.repo, "status", "--porcelain"))
            assertTrue(run(fx.repo, "stash", "list").contains("pegg: changes before checking out 48-dirty"))
        }
    }

    @Test
    fun `commits a dirty worktree when asked`() {
        withRepo { fx ->
            File(fx.repo, "README.md").writeText("changed\n")
            val result = CardCheckout.checkout(
                KanbanCard(title = "Dirty", issue = CardIssue(number = 49)),
                options(fx.repo) { DirtyChoice.COMMIT },
            )

            assertTrue(result.committed)
            assertEquals("", run(fx.repo, "status", "--porcelain"))
            assertTrue(run(fx.repo, "log", "-1", "--pretty=%s").contains("wip: changes before checking out 49-dirty"))
        }
    }

    @Test
    fun `rejects an unknown provider without an override`() {
        withRepo { fx ->
            val error = assertFailsWith<CardCheckoutException> {
                CardCheckout.checkout(
                    KanbanCard(title = "No provider", issue = CardIssue(number = 50)),
                    options(fx.repo, providerOverride = ""),
                )
            }
            assertTrue(error.message.orEmpty().contains("Could not detect the hosting provider"))
        }
    }

    @Test
    fun `reports a missing remote actionably`() {
        withRepo { fx ->
            val error = assertFailsWith<CardCheckoutException> {
                CardCheckout.checkout(
                    KanbanCard(title = "No remote", issue = CardIssue(number = 51)),
                    options(fx.repo, remote = "upstream"),
                )
            }
            assertTrue(error.message.orEmpty().contains("No git remote named 'upstream'"))
        }
    }

    @Test
    fun `rejects invalid branch names`() {
        withRepo { fx ->
            assertFailsWith<CardCheckoutException> {
                CardCheckout.checkout(
                    KanbanCard(title = "Bad name", issue = CardIssue(number = 52), context = CardContext(branch = "bad..name")),
                    options(fx.repo),
                )
            }
        }
    }
}
