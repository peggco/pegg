package dev.pegg.intellij.binary

import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

class BinaryTest {

    @Test
    fun `detects platform targets`() {
        assertEquals("linux", detectTarget("Linux", "amd64").os)
        assertEquals("darwin", detectTarget("Mac OS X", "aarch64").os)
        assertEquals("windows", detectTarget("Windows 11", "x86_64").os)
        assertEquals("pegg_linux_arm64.tar.gz", detectTarget("Linux", "aarch64").asset)
        assertEquals("pegg_windows_amd64.zip", detectTarget("Windows 11", "amd64").asset)
        assertEquals("pegg.exe", detectTarget("Windows 11", "amd64").binary)
    }

    @Test
    fun `parses checksum files`() {
        val hash = "a".repeat(64)
        val parsed = parseChecksums("$hash  pegg_linux_amd64.tar.gz\n${"b".repeat(64)} *pegg_darwin_arm64.tar.gz\n")
        assertEquals(hash, parsed["pegg_linux_amd64.tar.gz"])
        assertEquals("b".repeat(64), parsed["pegg_darwin_arm64.tar.gz"])
    }

    @Test
    fun `verifies a checksum and rejects a mismatch`() {
        val file = Files.createTempFile("pegg-checksum", ".bin").toFile()
        try {
            file.writeText("hello")
            val actual = sha256(file)
            verifyChecksum(file, mapOf("asset" to actual), "asset")
            assertFailsWith<IllegalStateException> {
                verifyChecksum(file, mapOf("asset" to "0".repeat(64)), "asset")
            }
        } finally {
            file.delete()
        }
    }

    @Test
    fun `parses PATH assignments from shell profiles`() {
        val home = Files.createTempDirectory("pegg-home").toFile()
        try {
            File(home, ".zshrc").writeText(
                "export PATH=\"$home/bin:/usr/local/pegg/bin:\$HOME/.local/bin\"\n" +
                    "PATH=\"$home/other/bin:\$PATH\"\n",
            )
            val dirs = profileSearchPaths(home.path)
            assertTrue(dirs.contains(File(home, "bin").path))
            assertTrue(dirs.contains(File(home, "other/bin").path))
            assertTrue(dirs.contains(File(home, ".local/bin").path))
            assertTrue(dirs.contains("/usr/local/pegg/bin"))
        } finally {
            home.deleteRecursively()
        }
    }

    @Test
    fun `search dirs include the home pegg bin`() {
        val home = System.getProperty("user.home")
        val dirs = searchDirs()
        assertTrue(dirs.any { it == File(home, ".pegg/bin").path || it.endsWith(".pegg/bin") })
    }
}
