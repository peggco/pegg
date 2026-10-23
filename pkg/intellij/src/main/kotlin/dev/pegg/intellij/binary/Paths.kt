package dev.pegg.intellij.binary

import java.io.File

private val PROFILE_FILES = listOf(".zshenv", ".zprofile", ".zshrc", ".bash_profile", ".bashrc", ".profile")
private val PATH_ASSIGNMENT = Regex(
    """(?:^|[\s;])(?:export\s+)?(?:PATH|path)\s*=\s*("([^"]*)"|'([^']*)'|([^\s;#]+))""",
)

fun searchDirs(): List<String> {
    val dirs = LinkedHashSet<String>()
    System.getenv("PATH")?.split(File.pathSeparatorChar)?.forEach { if (it.isNotBlank()) dirs.add(it) }
    val home = System.getProperty("user.home")
    val os = System.getProperty("os.name").lowercase()
    if (!os.startsWith("windows")) {
        if (os.startsWith("mac")) {
            dirs.add("/opt/homebrew/bin")
            dirs.add("/usr/local/bin")
            dirs.add("/opt/local/bin")
        } else {
            dirs.add("/usr/local/bin")
            dirs.add("/snap/bin")
        }
        dirs.add("$home/.local/bin")
        dirs.add("$home/bin")
        dirs.add("$home/.pegg/bin")
        dirs.addAll(profileSearchPaths(home))
    }
    return dirs.filter { it.isNotBlank() }
}

fun profileSearchPaths(home: String): List<String> {
    val dirs = LinkedHashSet<String>()
    for (profile in PROFILE_FILES) {
        val file = File(home, profile)
        if (!file.isFile) continue
        val content = runCatching { file.readText() }.getOrNull() ?: continue
        for (match in PATH_ASSIGNMENT.findAll(content)) {
            val value = match.groupValues[2].ifEmpty {
                match.groupValues[3].ifEmpty { match.groupValues[4] }
            }
            if (value.isBlank()) continue
            for (part in value.split(':')) {
                val expanded = expandHome(part.trim(), home) ?: continue
                if (expanded.isNotBlank()) dirs.add(expanded)
            }
        }
    }
    return dirs.toList()
}

private fun expandHome(dir: String, home: String): String? {
    if (dir.isBlank()) return null
    var out = dir
    if (out.startsWith("~")) out = File(home, out.removePrefix("~").removePrefix("/")).path
    out = out.replace("\${HOME}", home).replace("\$HOME", home)
    if (out.contains('$')) return null
    return out
}
