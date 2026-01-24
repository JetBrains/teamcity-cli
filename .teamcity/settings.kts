import jobs.Lint
import jobs.Test
import jobs.TestLinux
import jobs.TestWindows
import jetbrains.buildServer.configs.kotlin.*

version = "2025.11"

project {
    description = "A command-line interface for TeamCity that lets you manage builds, jobs, and projects without leaving your terminal"
    buildType(Lint)
    buildType(TestWindows)
    buildType(TestLinux)
    buildType(Test)
}
