package dev.pegg.intellij.settings

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import java.util.concurrent.CopyOnWriteArrayList

@Service(Service.Level.APP)
@State(name = "PeggSettings", storages = [Storage("pegg.xml")])
class PeggSettings : PersistentStateComponent<PeggSettings.State> {

    data class State(
        var executablePath: String = "",
        var autoDownload: Boolean = true,
        var allowWorkspaceBinary: Boolean = true,
        var binaryVersion: String = "match-extension",
        var downloadBaseUrl: String = "https://github.com/peggco/pegg/releases/download",
        var provider: String = "",
        var model: String = "",
        var defaultMode: String = "auto",
        var includeActiveFile: Boolean = true,
        var includeSelection: Boolean = true,
        var maxFileBytes: Int = 262144,
        var sendOnEnter: String = "enter",
        var notifyOnComplete: Boolean = true,
        var env: MutableMap<String, String> = linkedMapOf(),
        var logLevel: String = "info",
    )

    private var myState = State()
    private val listeners = CopyOnWriteArrayList<() -> Unit>()

    @Synchronized
    override fun getState(): State = myState

    @Synchronized
    override fun loadState(state: State) {
        myState = state
        if (myState.maxFileBytes < 1024) myState.maxFileBytes = 1024
        if (myState.logLevel !in setOf("error", "warn", "info", "debug")) myState.logLevel = "info"
        if (myState.sendOnEnter !in setOf("enter", "ctrl+enter")) myState.sendOnEnter = "enter"
        if (myState.defaultMode !in setOf("auto", "plan", "build")) myState.defaultMode = "auto"
    }

    fun addListener(listener: () -> Unit) = listeners.addIfAbsent(listener)

    fun removeListener(listener: () -> Unit) = listeners.remove(listener)

    fun notifyChanged() {
        listeners.forEach { runCatching { it() } }
    }

    fun mode(): String = myState.defaultMode

    fun env(): Map<String, String> = myState.env

    fun logLevel(): String = myState.logLevel

    companion object {
        fun getInstance(): PeggSettings =
            ApplicationManager.getApplication().getService(PeggSettings::class.java)
    }
}
