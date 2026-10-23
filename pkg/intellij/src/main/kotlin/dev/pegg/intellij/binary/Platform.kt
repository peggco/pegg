package dev.pegg.intellij.binary

data class Target(
    val os: String,
    val arch: String,
    val asset: String,
    val binary: String,
)

fun detectTarget(
    osName: String = System.getProperty("os.name"),
    archName: String = System.getProperty("os.arch"),
): Target {
    val os = when {
        osName.startsWith("Mac", ignoreCase = true) || osName.startsWith("Darwin", ignoreCase = true) -> "darwin"
        osName.startsWith("Windows", ignoreCase = true) -> "windows"
        else -> "linux"
    }
    val arch = when (archName.lowercase()) {
        "aarch64", "arm64" -> "arm64"
        "amd64", "x86_64", "x64" -> "amd64"
        else -> throw IllegalStateException("Unsupported architecture: $archName")
    }
    val ext = if (os == "windows") "zip" else "tar.gz"
    return Target(
        os = os,
        arch = arch,
        asset = "pegg_${os}_${arch}.$ext",
        binary = if (os == "windows") "pegg.exe" else "pegg",
    )
}
