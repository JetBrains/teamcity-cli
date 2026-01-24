package extensions

import jetbrains.buildServer.configs.kotlin.BuildSteps
import jetbrains.buildServer.configs.kotlin.buildSteps.ScriptBuildStep
import jetbrains.buildServer.configs.kotlin.buildSteps.script

private object Docker {
    const val GOLANGCI_LINT_IMAGE = "golangci/golangci-lint:latest"
}

fun BuildSteps.goLint() {
    script {
        id = "goLint"
        scriptContent = "go fmt ./... && golangci-lint run --tests=false ./..."
        dockerImage = Docker.GOLANGCI_LINT_IMAGE
        dockerImagePlatform = ScriptBuildStep.ImagePlatform.Linux
    }
}
