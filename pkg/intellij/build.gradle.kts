
plugins {
    id("java")
    id("org.jetbrains.kotlin.jvm") version "2.1.21"
    id("org.jetbrains.kotlin.plugin.serialization") version "2.1.21"
    id("org.jetbrains.intellij.platform")
}

group = providers.gradleProperty("pluginGroup").get()
version = providers.gradleProperty("pluginVersion").get()

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    intellijPlatform {
        intellijIdea(providers.gradleProperty("platformVersion").get())
        bundledPlugin("com.intellij.modules.jcef")
        pluginVerifier()
        zipSigner()
    }

    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.8.0")
    implementation("org.apache.commons:commons-compress:1.27.1")

    testImplementation(kotlin("test"))
    testImplementation("org.junit.jupiter:junit-jupiter:5.11.4")
    testImplementation("junit:junit:4.13.2")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher:1.11.4")
}

kotlin {
    jvmToolchain(21)
}

sourceSets {
    main {
        resources.srcDir(layout.buildDirectory.dir("webview"))
    }
}

intellijPlatform {
    pluginConfiguration {
        id = "dev.pegg.intellij"
        name = "Pegg"
        version = providers.gradleProperty("pluginVersion").get()
        description = """
            <p>Pegg brings the Pegg AI coding agent into every JetBrains IDE.</p>
            <p>Chat, tools, sessions, subagents, plans, permissions, attachments, voice input,
            and editor actions are driven by the local Pegg binary over the Agent Client Protocol.</p>
        """.trimIndent()
        vendor {
            name = "Pegg"
            url = "https://pegg.dev"
        }
        ideaVersion {
            sinceBuild = "253.1"
            untilBuild = provider { null }
        }
    }
}

val buildWebview by tasks.registering(Exec::class) {
    workingDir = projectDir
    commandLine("node", "build-webview.mjs")
    inputs.dir(layout.projectDirectory.dir("webview"))
    inputs.dir(layout.projectDirectory.dir("../chat-ui/src"))
    inputs.file(layout.projectDirectory.file("build-webview.mjs"))
    outputs.dir(layout.buildDirectory.dir("webview"))
}

tasks.named<ProcessResources>("processResources") {
    dependsOn(buildWebview)
}

tasks.test {
    useJUnitPlatform()
    systemProperty("java.awt.headless", "true")
}

tasks.named<JavaExec>("runIde") {
    jvmArgs("-Didea.auto.reload.plugins=false")
}
