package dev.pegg.intellij.git

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class BranchNameTest {
    @Test
    fun `slugifies titles`() {
        assertEquals("fix-login-redirect", BranchName.slugify("Fix Login Redirect"))
        assertEquals("fix-login", BranchName.slugify("[FEATURE] Fix login (again)"))
        assertEquals("crash-on-start", BranchName.slugify("[BUG]: Crash on start"))
        assertEquals("foo-bar-baz", BranchName.slugify("  Foo / Bar -- Baz! "))
    }

    @Test
    fun `caps long titles on a word boundary`() {
        val slug = BranchName.slugify("A very long issue title that keeps going and going and going forever")
        assertTrue(slug.length <= 40)
        assertTrue(slug.startsWith("a-very-long-issue-title"))
        assertTrue(!slug.endsWith("-"))
    }

    @Test
    fun `renders the default template`() {
        assertEquals("42-fix-login", BranchName.render("{number}-{slug}", "42", "Fix Login", null, "github"))
        assertEquals("7-mr-flow", BranchName.render("{number}-{slug}", "7", "MR flow", null, "gitlab"))
        assertEquals(
            "github/1-hello",
            BranchName.render("{provider}/{number}-{slug}", "1", "Hello", null, "github"),
        )
        assertEquals("issue/9", BranchName.render("issue/{number}", "9", "Anything", null, null))
    }

    @Test
    fun `omits an unavailable number without leaving separators`() {
        assertEquals("no-issue", BranchName.render("{number}-{slug}", null, "No issue", null, null))
        assertEquals("no-issue", BranchName.render("{number}/{slug}", null, "No issue", null, null))
    }

    @Test
    fun `sanitizes unsafe characters and falls back`() {
        assertEquals("3-fix-login", BranchName.render("{number} {slug}", "3", "Fix: login?", null, null))
        assertEquals("", BranchName.render("{slug}", null, "   ", null, null))
        assertEquals("fallback-title", BranchName.forCard("{number}", KanbanCard(title = "Fallback title")))
        assertEquals("work", BranchName.forCard("{number}", KanbanCard(title = "  ")))
    }
}
