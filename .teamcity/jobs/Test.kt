package jobs

import extensions.gitHubIntegration
import jetbrains.buildServer.configs.kotlin.*
import jetbrains.buildServer.configs.kotlin.buildFeatures.golang
import jetbrains.buildServer.configs.kotlin.buildFeatures.perfmon
import jetbrains.buildServer.configs.kotlin.buildFeatures.swabra
import jetbrains.buildServer.configs.kotlin.buildSteps.script
import jetbrains.buildServer.configs.kotlin.triggers.vcs

abstract class TestBuild(
    private val os: String,
    private val agentName: String,
    private val setupScript: String? = null
) : BuildType() {
    init {
        id("Test_$os")
        name = "Test $os"

        params {
            param("env.TC_INSECURE_SKIP_WARN", "1")
            param("env.GOPROXY", "https://proxy.golang.org,direct")
        }

        requirements {
            contains("teamcity.agent.name", this@TestBuild.agentName)
        }

        vcs {
            root(DslContext.settingsRoot)
        }

        steps {
            this@TestBuild.setupScript?.let {
                script {
                    id = "setup"
                    scriptContent = it
                }
            }
            script {
                id = "goTest"
                scriptContent = "go test -v ./... -timeout 15m -coverpkg=./... -coverprofile=coverage.out"
            }
        }

        features {
            perfmon {}
            golang { testFormat = "json" }
            gitHubIntegration()
            swabra {}
        }
    }
}

object TestWindows : TestBuild(
    os = "Windows",
    agentName = "Windows"
)

object TestLinux : TestBuild(
    os = "Linux",
    agentName = "Ubuntu",
    setupScript = """
        sudo add-apt-repository ppa:longsleep/golang-backports
        sudo apt update
        sudo apt install -y golang-go
    """.trimIndent()
)

object Test : BuildType({
    id("Test")
    name = "Test"
    type = Type.COMPOSITE

    vcs {
        root(DslContext.settingsRoot)
        showDependenciesChanges = true
    }

    triggers {
        vcs {
            branchFilter = """
                +:<default>
                +:pull/*
            """.trimIndent()
        }
    }

    features {
        gitHubIntegration()
    }

    dependencies {
        snapshot(TestWindows) {
            onDependencyFailure = FailureAction.FAIL_TO_START
        }
        snapshot(TestLinux) {
            onDependencyFailure = FailureAction.FAIL_TO_START
        }
    }
})
