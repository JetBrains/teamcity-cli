package extensions

import jetbrains.buildServer.configs.kotlin.BuildSteps
import jetbrains.buildServer.configs.kotlin.buildSteps.ScriptBuildStep
import jetbrains.buildServer.configs.kotlin.buildSteps.script

private object Docker {
    const val DIND_IMAGE = "docker:dind"
    const val PRIVILEGED_HOST_NETWORK = "--network host -v /var/run/docker.sock:/var/run/docker.sock --privileged"
}

fun BuildSteps.goTest() {
    script {
        id = "goTest"
        scriptContent = """
            apk add --no-cache go just
            export PATH=${'$'}PATH:/root/go/bin
            just test-ci
        """.trimIndent()
        dockerImage = Docker.DIND_IMAGE
        dockerImagePlatform = ScriptBuildStep.ImagePlatform.Linux
        dockerRunParameters = Docker.PRIVILEGED_HOST_NETWORK
    }
}
