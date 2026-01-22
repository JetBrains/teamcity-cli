package jobs

import extensions.gitHubIntegration
import extensions.goLint
import jetbrains.buildServer.configs.kotlin.BuildType
import jetbrains.buildServer.configs.kotlin.DslContext
import jetbrains.buildServer.configs.kotlin.buildFeatures.perfmon
import jetbrains.buildServer.configs.kotlin.buildFeatures.swabra
import jetbrains.buildServer.configs.kotlin.triggers.vcs

object Lint : BuildType({
    name = "Lint"

    vcs {
        root(DslContext.settingsRoot)
    }

    steps {
        goLint()
    }

    triggers {
        vcs {
            branchFilter = "+:<default>"
        }
    }

    features {
        perfmon {}
        gitHubIntegration()
        swabra {}
    }
})
