import jobs.Test
import jetbrains.buildServer.configs.kotlin.*

version = "2025.11"

project {
    description = "A command-line interface for TeamCity that lets you manage builds, jobs, and projects without leaving your terminal"
    buildType(Test)
}
