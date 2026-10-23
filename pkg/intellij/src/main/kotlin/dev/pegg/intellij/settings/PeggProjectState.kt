package dev.pegg.intellij.settings

import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import com.intellij.openapi.project.Project

@Service(Service.Level.PROJECT)
@State(name = "PeggProjectState", storages = [Storage("pegg-session.xml")])
class PeggProjectState : PersistentStateComponent<PeggProjectState.State> {

    data class State(
        var acpSessionId: String? = null,
        var peggSessionId: String? = null,
    )

    private var myState = State()

    @Synchronized
    override fun getState(): State = myState

    @Synchronized
    override fun loadState(state: State) {
        myState = state
    }

    @Synchronized
    fun save(acpSessionId: String?, peggSessionId: String?) {
        myState.acpSessionId = acpSessionId
        myState.peggSessionId = peggSessionId
    }

    companion object {
        fun getInstance(project: Project): PeggProjectState =
            project.getService(PeggProjectState::class.java)
    }
}
