package extensions

import jetbrains.buildServer.configs.kotlin.BuildFeatures
import jetbrains.buildServer.configs.kotlin.DslContext
import jetbrains.buildServer.configs.kotlin.buildFeatures.PullRequests
import jetbrains.buildServer.configs.kotlin.buildFeatures.commitStatusPublisher
import jetbrains.buildServer.configs.kotlin.buildFeatures.pullRequests

private const val GITHUB_API_URL = "https://api.github.com"

fun BuildFeatures.gitHubIntegration() {
    commitStatusPublisher {
        publisher = github {
            githubUrl = GITHUB_API_URL
            authType = vcsRoot()
        }
    }
    pullRequests {
        vcsRootExtId = "${DslContext.settingsRoot.id}"
        provider = github {
            authType = vcsRoot()
            filterAuthorRole = PullRequests.GitHubRoleFilter.MEMBER
            ignoreDrafts = true
        }
    }
}
