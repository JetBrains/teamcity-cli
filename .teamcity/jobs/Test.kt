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
    private val scriptContent: String
) : BuildType() {
    init {
        id("Test_$os")
        name = "Test $os"

        params {
            param("env.TC_INSECURE_SKIP_WARN", "1")
        }

        requirements {
            contains("teamcity.agent.name", this@TestBuild.agentName)
        }

        vcs {
            root(DslContext.settingsRoot)
        }

        steps {
            script {
                this.scriptContent = this@TestBuild.scriptContent
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
    agentName = "Windows",
    scriptContent = "go test -v ./... -timeout 15m -coverpkg=./... -coverprofile=coverage.out"
)

object TestLinux : TestBuild(
    os = "Linux",
    agentName = "Ubuntu",
    scriptContent = """
        sudo add-apt-repository ppa:longsleep/golang-backports
        sudo apt update
        sudo apt install -y golang-go
        export GOPROXY=https://proxy.golang.org,direct
        export GOROOT=/usr/lib/go-1.25
        /usr/lib/go-1.25/bin/go test -v ./... -timeout 15m -coverpkg=./... -coverprofile=coverage.out
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
