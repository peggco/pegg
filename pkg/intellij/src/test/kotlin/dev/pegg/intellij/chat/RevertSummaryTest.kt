package dev.pegg.intellij.chat

import dev.pegg.intellij.acp.ACP_JSON
import dev.pegg.intellij.acp.RevertFileChange
import dev.pegg.intellij.acp.RevertSummary
import dev.pegg.intellij.acp.SessionRevertResult
import kotlin.test.Test
import kotlin.test.assertEquals

class RevertSummaryTest {

    @Test
    fun `decodes the revert result with per-file changes`() {
        val json = """
            {"removed":2,"snapshotId":"snap-1","files":[
              {"path":"src/a.go","additions":3,"deletions":1,"status":"reverted"},
              {"path":"src/b.ts","additions":1,"deletions":0,"status":"conflict"}
            ],"totalAdditions":4,"totalDeletions":1}
        """.trimIndent()

        val result = ACP_JSON.decodeFromString(SessionRevertResult.serializer(), json)

        assertEquals(2, result.removed)
        assertEquals("snap-1", result.snapshotId)
        assertEquals(2, result.files.size)
        assertEquals(RevertFileChange("src/a.go", 3, 1, "reverted"), result.files[0])
        assertEquals("conflict", result.files[1].status)
        assertEquals(4, result.totalAdditions)
        assertEquals(1, result.totalDeletions)
    }

    @Test
    fun `round trips the summary posted to the webview`() {
        val summary = RevertSummary(
            removed = 1,
            snapshotId = "snap-2",
            files = listOf(RevertFileChange("a.kt", 2, 3, "reverted")),
            totalAdditions = 2,
            totalDeletions = 3,
            status = "reverted",
        )

        val encoded = ACP_JSON.encodeToString(RevertSummary.serializer(), summary)
        val decoded = ACP_JSON.decodeFromString(RevertSummary.serializer(), encoded)

        assertEquals(summary, decoded)
    }
}
