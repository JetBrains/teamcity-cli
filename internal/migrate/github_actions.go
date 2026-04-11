package migrate

import (
	"cmp"
	"fmt"
	"strings"
)

var skippedActions = []struct{ action, note string }{
	{"actions/checkout", "checkout (TeamCity VCS checkout is automatic)"},
	{"actions/checkout-v2", "checkout (TeamCity VCS checkout is automatic)"},
	{"actions/setup-node", "setup-node (use agent tooling or nvm)"},
	{"actions/setup-java", "setup-java (use agent JDK)"},
	{"actions/setup-go", "setup-go (use agent Go installation)"},
	{"actions/setup-python", "setup-python (use agent Python)"},
	{"actions/setup-dotnet", "setup-dotnet (use agent .NET SDK)"},
	{"actions/setup-ruby", "setup-ruby (use agent Ruby or rbenv)"},
	{"actions/setup-php", "setup-php (use agent PHP)"},
	{"shivammathur/setup-php", "setup-php (use agent PHP)"},
	{"julia-actions/setup-julia", "setup-julia (use agent Julia)"},
	{"actions/setup-elixir", "setup-elixir (use agent Elixir)"},
	{"actions/setup-haskell", "setup-haskell (use agent GHC)"},
	{"erlef/setup-beam", "setup-beam (use agent Erlang/Elixir)"},
	{"ruby/setup-ruby", "setup-ruby (use agent Ruby or rbenv)"},
	{"haskell-actions/setup", "setup-haskell (use agent GHC)"},
	{"gradle/actions/setup-gradle", "setup-gradle (Gradle wrapper used directly)"},
	{"gradle/gradle-build-action", "setup-gradle (Gradle wrapper used directly)"},
	{"ATiltedTree/setup-rust", "setup-rust (use agent Rust toolchain)"},
	{"dtolnay/rust-toolchain", "rust-toolchain (use agent Rust toolchain)"},
	{"actions-rust-lang/setup-rust-toolchain", "setup-rust-toolchain (use agent Rust)"},
	{"subosito/flutter-action", "flutter-action (use agent Flutter SDK)"},
	{"dart-lang/setup-dart", "setup-dart (use agent Dart SDK)"},
	{"swift-actions/setup-swift", "setup-swift (use agent Swift toolchain)"},
	{"pnpm/action-setup", "pnpm-setup (use agent pnpm)"},
	{"oven-sh/setup-bun", "setup-bun (use agent Bun)"},
	{"denoland/setup-deno", "setup-deno (use agent Deno)"},
	{"hashicorp/setup-terraform", "setup-terraform (use agent Terraform installation)"},
	{"google-github-actions/setup-gcloud", "setup-gcloud (use agent gcloud installation)"},
	{"docker/setup-buildx-action", "setup-buildx (configure on agent)"},
	{"docker/setup-qemu-action", "setup-qemu (configure on agent for multi-arch builds)"},
	{"dorny/test-reporter", "test-reporter → TeamCity has built-in XML test report processing"},
	{"mikepenz/action-junit-report", "junit-report → TeamCity has built-in JUnit report processing"},
	{"EnricoMi/publish-unit-test-result-action", "publish-unit-test-result → TeamCity has built-in test report processing"},
	{"actions/configure-pages", "configure-pages (GitHub Pages metadata, no-op for TeamCity)"},
	{"actions/deploy-pages", "deploy-pages (GitHub Pages specific)"},
}

var scriptActions = []struct{ action, script, name string }{
	{"codecov/codecov-action", "curl -Os https://cli.codecov.io/latest/linux/codecov && chmod +x codecov && ./codecov", "Codecov upload"},
	{"sonarsource/sonarqube-scan-action", "# TODO: Configure SonarQube connection in TeamCity project settings\nsonar-scanner", "SonarQube scan"},
	{"sonarsource/sonarcloud-github-action", "# TODO: Configure SonarCloud connection in TeamCity project settings\nsonar-scanner", "SonarCloud scan"},
	{"pre-commit/action", "pre-commit run --all-files", "Pre-commit checks"},
	{"super-linter/super-linter", "docker run --rm -v \"$(pwd)\":/tmp/lint ghcr.io/super-linter/super-linter:latest", "Super-Linter"},
	{"helm/kind-action", "kind create cluster", "Create kind cluster"},
	{"peter-evans/create-pull-request", "gh pr create --fill", "Create pull request"},
	{"cypress-io/github-action", "npx cypress run", "Cypress E2E tests"},
	{"coverallsapp/github-action", "npm install -g coveralls && cat coverage/lcov.info | coveralls", "Coveralls upload"},
	{"paambaati/codeclimate-action", "curl -L https://codeclimate.com/downloads/test-reporter/test-reporter-latest-linux-amd64 > cc-test-reporter && chmod +x cc-test-reporter && ./cc-test-reporter after-build", "Code Climate upload"},
	{"snyk/actions", "snyk test", "Snyk security scan"},
	{"wagoid/commitlint-github-action", "npx commitlint --from HEAD~1", "Commitlint"},
	{"treosh/lighthouse-ci-action", "npx @lhci/cli autorun", "Lighthouse CI"},
	{"ad-m/github-push-action", "git push origin HEAD", "Git push"},
	{"stefanzweifel/git-auto-commit-action", "git add -A && git diff --cached --quiet || git commit -m \"Auto-commit\" && git push", "Git auto-commit"},
	{"EndBug/add-and-commit", "git add -A && git diff --cached --quiet || git commit -m \"Auto-commit\" && git push", "Git add and commit"},
}

var unsupportedActions = []struct {
	action, reason string
	manual         []string
}{
	{"github/codeql-action/init", "CodeQL init → use Qodana or third-party SAST in TeamCity",
		[]string{"CodeQL → consider Qodana build feature for static analysis in TeamCity"}},
	{"github/codeql-action/analyze", "CodeQL analyze → use Qodana or third-party SAST in TeamCity", nil},
	{"github/codeql-action/autobuild", "CodeQL autobuild → not needed with Qodana", nil},
	{"actions/labeler", "actions/labeler → GitHub-specific; no TeamCity equivalent", nil},
	{"actions/stale", "actions/stale → GitHub-specific; no TeamCity equivalent", nil},
	{"actions/first-interaction", "actions/first-interaction → GitHub-specific", nil},
	{"ossf/scorecard-action", "scorecard-action → GitHub-specific security scoring", nil},
	{"peter-evans/repository-dispatch", "repository-dispatch → GitHub-specific event system", nil},
	{"hmarr/auto-approve-action", "auto-approve → GitHub PR-specific", nil},
	{"pascalgn/automerge-action", "automerge → GitHub PR-specific", nil},
	{"dessant/lock-threads", "lock-threads → GitHub-specific issue management", nil},
}

func initActionRegistry() map[string]actionTransformer {
	m := map[string]actionTransformer{}

	for _, a := range skippedActions {
		note := a.note
		m[a.action] = func(_, _ string, _ map[string]string) StepResult {
			return StepResult{Status: StatusSimplified, Note: note}
		}
	}
	for _, a := range scriptActions {
		script, name := a.script, a.name
		m[a.action] = func(stepName, _ string, _ map[string]string) StepResult {
			return Converted([]Step{{Name: cmp.Or(stepName, name), ScriptContent: script}})
		}
	}
	for _, a := range unsupportedActions {
		reason, id, manual := a.reason, a.action, a.manual
		m[a.action] = func(_, _ string, _ map[string]string) StepResult {
			return StepResult{Status: StatusUnsupported, Identifier: id, Note: reason, ManualTasks: manual}
		}
	}

	m["actions/cache"] = func(_, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusSimplified, Note: "cache → enable-dependency-cache: true", Features: []string{"enable-dependency-cache"}}
	}

	m["actions/upload-artifact"] = func(_, _ string, inputs map[string]string) StepResult {
		path := cmp.Or(inputs["path"], "**/*")
		return StepResult{Status: StatusSimplified, Note: "upload-artifact → files-publication",
			Artifacts: []FilePublication{{Path: path, ShareWithJobs: true, PublishArtifact: true}}}
	}
	m["actions/download-artifact"] = func(_, _ string, inputs map[string]string) StepResult {
		r := StepResult{Status: StatusSimplified, Note: "download-artifact → files-publication with share-with-jobs"}
		if name := inputs["name"]; name != "" {
			r.ManualTasks = []string{fmt.Sprintf("Artifact download %q → ensure upstream job publishes via files-publication with share-with-jobs: true", name)}
		}
		return r
	}

	m["docker/login-action"] = func(name, _ string, inputs map[string]string) StepResult {
		registry := cmp.Or(inputs["registry"], "Docker Hub")
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Docker login"), ScriptContent: "# Configure Docker registry connection in TeamCity project settings\n# Registry: " + registry}},
			ManualTasks: []string{fmt.Sprintf("Docker registry %s → configure Docker connection in TeamCity project settings", registry)}}
	}
	m["docker/build-push-action"] = transformDockerBuild
	m["docker/metadata-action"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Docker metadata"), ScriptContent: "# TODO: docker/metadata-action generates tags/labels from git context\necho 'Set IMAGE tag from TeamCity build parameters'"}},
			ManualTasks: []string{"docker/metadata-action → use TeamCity build parameters for image tags (%%build.vcs.number%%, %%build.number%%)"}}
	}

	m["JetBrains/qodana-action"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted, Note: "Qodana → use TeamCity native Qodana build feature",
			Steps: []Step{{Name: cmp.Or(name, "Qodana"), ScriptContent: "# TeamCity has native Qodana integration\n# Add Qodana build feature in TeamCity project settings\n# Or run via Docker:\ndocker run --rm -v \"$(pwd)\":/data/project jetbrains/qodana-jvm-community:latest"}}}
	}

	m["aws-actions/configure-aws-credentials"] = func(name, _ string, inputs map[string]string) StepResult {
		region := cmp.Or(inputs["aws-region"], "us-east-1")
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Configure AWS credentials"), ScriptContent: "export AWS_ACCESS_KEY_ID=\"$AWS_ACCESS_KEY_ID\"\nexport AWS_SECRET_ACCESS_KEY=\"$AWS_SECRET_ACCESS_KEY\"\nexport AWS_DEFAULT_REGION=\"" + region + "\""}},
			ManualTasks: []string{"AWS credentials → create TeamCity parameters: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY (type: password)"}}
	}
	m["azure/login"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Azure login"), ScriptContent: "az login --service-principal -u \"$AZURE_CLIENT_ID\" -p \"$AZURE_CLIENT_SECRET\" --tenant \"$AZURE_TENANT_ID\""}},
			ManualTasks: []string{"Azure credentials → create TeamCity parameters: AZURE_CLIENT_ID, AZURE_CLIENT_SECRET (password), AZURE_TENANT_ID"}}
	}
	m["google-github-actions/auth"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Google Cloud auth"), ScriptContent: "gcloud auth activate-service-account --key-file=\"$GOOGLE_APPLICATION_CREDENTIALS\""}},
			ManualTasks: []string{"GCP credentials → configure service account key as TeamCity secure parameter"}}
	}
	m["aws-actions/amazon-ecr-login"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "ECR login"), ScriptContent: "aws ecr get-login-password --region \"$AWS_DEFAULT_REGION\" | docker login --username AWS --password-stdin \"$ECR_REGISTRY\""}},
			ManualTasks: []string{"ECR login → ensure AWS credentials and ECR_REGISTRY parameter are configured"}}
	}
	m["webfactory/ssh-agent"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "SSH agent"), ScriptContent: "eval \"$(ssh-agent -s)\" && echo \"$SSH_PRIVATE_KEY\" | ssh-add -"}},
			ManualTasks: []string{"SSH private key → create TeamCity parameter SSH_PRIVATE_KEY (type: password)"}}
	}
	m["hashicorp/vault-action"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Vault secrets"), ScriptContent: "# TODO: Fetch secrets from HashiCorp Vault\nexport VAULT_ADDR=\"$VAULT_ADDR\"\nvault kv get -format=json secret/data/ci"}},
			ManualTasks: []string{"Vault → configure VAULT_ADDR and VAULT_TOKEN as TeamCity parameters"}}
	}

	m["azure/webapps-deploy"] = func(name, _ string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Azure Web App deploy"), ScriptContent: fmt.Sprintf("az webapp deploy --name %q --src-path \"${PACKAGE:-.}\"", inputs["app-name"])}})
	}
	m["aws-actions/amazon-ecs-deploy-task-definition"] = func(name, _ string, _ map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "ECS deploy"), ScriptContent: "aws ecs update-service --cluster \"$CLUSTER\" --service \"$SERVICE\" --force-new-deployment"}})
	}
	m["azure/k8s-deploy"] = func(name, _ string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Kubernetes deploy"), ScriptContent: fmt.Sprintf("kubectl apply -f %s", cmp.Or(inputs["manifests"], "k8s/"))}})
	}
	m["azure/k8s-set-context"] = func(name, _ string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "K8s set context"), ScriptContent: fmt.Sprintf("az aks get-credentials --resource-group %q --name %q", inputs["resource-group"], inputs["cluster-name"])}})
	}
	m["google-github-actions/deploy-cloudrun"] = func(name, _ string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Cloud Run deploy"), ScriptContent: fmt.Sprintf("gcloud run deploy %q --image %q --region \"${REGION:-us-central1}\"", inputs["service"], inputs["image"])}})
	}
	m["aws-actions/amazon-ecs-render-task-definition"] = func(name, _ string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "ECS render task def"), ScriptContent: fmt.Sprintf("jq '.containerDefinitions[0].image = \"%s\"' %s > new-task-def.json", inputs["image"], cmp.Or(inputs["task-definition"], "task-definition.json"))}})
	}
	m["FirebaseExtended/action-hosting-deploy"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Firebase deploy"), ScriptContent: "firebase deploy --only hosting"}},
			ManualTasks: []string{"Firebase → create TeamCity parameter FIREBASE_TOKEN (type: password)"}}
	}
	m["amondnet/vercel-action"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Vercel deploy"), ScriptContent: "npx vercel --prod --token \"$VERCEL_TOKEN\""}},
			ManualTasks: []string{"Vercel → create TeamCity parameter VERCEL_TOKEN (type: password)"}}
	}
	m["appleboy/scp-action"] = func(name, _ string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "SCP deploy"), ScriptContent: fmt.Sprintf("scp -r %s %s:%s", cmp.Or(inputs["source"], "."), inputs["host"], cmp.Or(inputs["target"], "~/"))}})
	}
	m["SamKirkland/FTP-Deploy-Action"] = func(name, _ string, inputs map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "FTP deploy"), ScriptContent: fmt.Sprintf("lftp -c \"open -u $FTP_USER,$FTP_PASSWORD %s; mirror -R %s %s\"", inputs["server"], cmp.Or(inputs["local-dir"], "./"), cmp.Or(inputs["server-dir"], "/"))}},
			ManualTasks: []string{"FTP credentials → create TeamCity parameters FTP_USER, FTP_PASSWORD (type: password)"}}
	}
	m["ncipollo/release-action"] = func(name, _ string, inputs map[string]string) StepResult {
		tag := cmp.Or(inputs["tag"], "$TAG")
		cmd := fmt.Sprintf("gh release create %q --generate-notes", tag)
		if body := inputs["body"]; body != "" {
			cmd += fmt.Sprintf(" --notes %q", body)
		}
		return Converted([]Step{{Name: cmp.Or(name, "GitHub release"), ScriptContent: cmd}})
	}
	m["anothrNick/github-tag-action"] = func(name, _ string, _ map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Auto tag"), ScriptContent: "# TODO: Determine new version tag from commit messages\ngit tag \"$NEW_TAG\" && git push origin \"$NEW_TAG\""}})
	}

	m["golangci/golangci-lint-action"] = func(name, _ string, inputs map[string]string) StepResult {
		cmd := "golangci-lint run"
		if args := inputs["args"]; args != "" {
			cmd += " " + args
		}
		r := Converted([]Step{{Name: cmp.Or(name, "golangci-lint"), ScriptContent: cmd}})
		if v := inputs["version"]; v != "" {
			r.ManualTasks = []string{fmt.Sprintf("golangci-lint version %s → ensure installed on agent", v)}
		}
		return r
	}

	m["actions/create-release"] = func(name, _ string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Create release"), ScriptContent: fmt.Sprintf("gh release create %q --generate-notes", cmp.Or(inputs["tag_name"], "$TAG"))}})
	}
	m["softprops/action-gh-release"] = func(name, _ string, inputs map[string]string) StepResult {
		cmd := fmt.Sprintf("gh release create %q --generate-notes", cmp.Or(inputs["tag_name"], "$TAG"))
		if files := inputs["files"]; files != "" {
			cmd += " " + files
		}
		return Converted([]Step{{Name: cmp.Or(name, "GitHub release"), ScriptContent: cmd}})
	}
	m["peaceiris/actions-gh-pages"] = func(name, _ string, inputs map[string]string) StepResult {
		dir := cmp.Or(inputs["publish_dir"], "./public")
		return Converted([]Step{{Name: cmp.Or(name, "Deploy to GitHub Pages"), ScriptContent: ghPagesScript("gh-pages", dir)}})
	}
	m["JamesIves/github-pages-deploy-action"] = func(name, _ string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Deploy to GitHub Pages"), ScriptContent: ghPagesScript(cmp.Or(inputs["branch"], "gh-pages"), cmp.Or(inputs["folder"], "."))}})
	}

	m["actions/github-script"] = func(name, _ string, inputs map[string]string) StepResult {
		script := cmp.Or(inputs["script"], "echo 'TODO: convert GitHub Script to shell commands'")
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "GitHub script"), ScriptContent: "# TODO: This was a GitHub Script action using Octokit\n# " + strings.ReplaceAll(script, "\n", "\n# ")}},
			ManualTasks: []string{"actions/github-script → convert Octokit JS to shell/curl commands"}}
	}
	m["dorny/paths-filter"] = func(_, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusUnsupported, Identifier: "dorny/paths-filter", Note: "paths-filter → use TeamCity VCS trigger rules",
			ManualTasks: []string{"dorny/paths-filter → configure VCS trigger rules with path patterns in TeamCity"}}
	}
	m["tj-actions/changed-files"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Get changed files"), ScriptContent: "CHANGED_FILES=$(git diff --name-only HEAD~1)\necho \"$CHANGED_FILES\""}},
			ManualTasks: []string{"tj-actions/changed-files → TeamCity provides %%teamcity.build.changedFiles.file%%"}}
	}

	m["slackapi/slack-github-action"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Slack notification"), ScriptContent: "# TeamCity has built-in Slack integration\n# Configure in: Project Settings → Build Features → Slack Notifier"}},
			ManualTasks: []string{"Slack notification → configure TeamCity Slack Notifier"}}
	}

	m["aquasecurity/trivy-action"] = func(name, _ string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Trivy security scan"), ScriptContent: fmt.Sprintf("trivy %s %s", cmp.Or(inputs["scan-type"], "fs"), cmp.Or(inputs["image-ref"], "."))}})
	}

	m["JS-DevTools/npm-publish"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted, Steps: []Step{{Name: cmp.Or(name, "npm publish"), ScriptContent: "npm publish"}},
			ManualTasks: []string{"npm publish token → create TeamCity parameter NPM_TOKEN (type: password)"}}
	}
	m["pypa/gh-action-pypi-publish"] = func(name, _ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusConverted, Steps: []Step{{Name: cmp.Or(name, "PyPI publish"), ScriptContent: "pip install twine && twine upload dist/*"}},
			ManualTasks: []string{"PyPI credentials → create TeamCity parameters TWINE_USERNAME, TWINE_PASSWORD (type: password)"}}
	}

	return m
}

func transformDockerBuild(name, _ string, inputs map[string]string) StepResult {
	context := cmp.Or(inputs["context"], ".")
	tags := inputs["tags"]
	file := inputs["file"]

	var lines []string
	if file != "" && file != "Dockerfile" {
		lines = append(lines, fmt.Sprintf("DOCKERFILE=%q", file))
	}
	if tags != "" {
		tagList := strings.Split(strings.TrimSpace(tags), "\n")
		for i, tag := range tagList {
			tagList[i] = strings.TrimSpace(tag)
		}
		lines = append(lines, fmt.Sprintf("IMAGE=%q", tagList[0]))
		for _, extra := range tagList[1:] {
			if extra != "" {
				lines = append(lines, fmt.Sprintf("# Additional tag: %s", extra))
			}
		}
	} else {
		lines = append(lines, `IMAGE="${IMAGE:?Set IMAGE variable}"`)
	}

	buildCmd := "docker build"
	if file != "" && file != "Dockerfile" {
		buildCmd += ` -f "$DOCKERFILE"`
	}
	if buildArgs := inputs["build-args"]; buildArgs != "" {
		for arg := range strings.SplitSeq(strings.TrimSpace(buildArgs), "\n") {
			if arg = strings.TrimSpace(arg); arg != "" {
				buildCmd += fmt.Sprintf(" --build-arg %q", arg)
			}
		}
	}
	buildCmd += ` -t "$IMAGE" ` + context
	lines = append(lines, buildCmd)

	if inputs["push"] == "true" {
		lines = append(lines, `docker push "$IMAGE"`)
	}
	return Converted([]Step{{Name: cmp.Or(name, "Docker build and push"), ScriptContent: strings.Join(lines, "\n")}})
}

func ghPagesScript(branch, folder string) string {
	return fmt.Sprintf("git config user.name \"TeamCity\"\ngit config user.email \"teamcity@localhost\"\ngit checkout --orphan %s\ncp -r %s/* .\ngit add .\ngit commit -m \"Deploy\"\ngit push origin %s --force", branch, folder, branch)
}
