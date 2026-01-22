# TeamCity CLI Feature Gap Analysis

**Date:** January 2026  
**Purpose:** Identify features available in GitHub CLI and TeamCity API that could enhance teamcity-cli

## Executive Summary

This analysis compares teamcity-cli with:
1. **GitHub CLI (gh)** - As a reference for modern CLI UX patterns
2. **TeamCity REST API** - To identify unutilized TeamCity capabilities

**Note:** Agent-related features are excluded per project requirements.

---

## Current Implementation Overview

### ✅ What teamcity-cli Does Well

**Authentication & Configuration:**
- ✅ Token-based auth with multi-server support
- ✅ Environment variable overrides for CI/CD
- ✅ Interactive login flow

**Build/Run Management:**
- ✅ List, view, start, cancel, restart builds
- ✅ Real-time build watching
- ✅ Log streaming with interactive viewer
- ✅ Artifact downloads
- ✅ Build pinning, tagging, commenting
- ✅ VCS changes view
- ✅ Test results display

**Build Configuration (Jobs):**
- ✅ List, view, pause/resume
- ✅ Parameter management

**Projects:**
- ✅ List, view
- ✅ Parameter management
- ✅ Secure token management

**Queue Management:**
- ✅ List, remove, reorder, approve

**Developer Experience:**
- ✅ JSON output for scripting
- ✅ Plain text mode for parsing
- ✅ Color-coded output
- ✅ Shell completion
- ✅ Raw API access

---

## Gap Analysis: Missing High-Value Features

### 1. 🔴 **Build Investigation & Muting** (HIGH PRIORITY)
**Inspired by:** GitHub issue/PR assignment and triage features  
**TeamCity API:** `/app/rest/investigations`, `/app/rest/mutes`

**Missing capabilities:**
- ❌ Investigate build failures (assign to user)
- ❌ Mute/unmute build problems
- ❌ Mute/unmute failing tests
- ❌ View current investigations
- ❌ Resolve investigations

**Why this matters:**
- Critical for team collaboration on build failures
- Reduces noise from known issues
- Helps track ownership of build problems
- Common workflow in CI/CD teams

**Proposed commands:**
```bash
tc run investigate <build-id> --user <username> --comment "Looking into memory leak"
tc run uninvestigate <build-id>
tc problem mute <problem-id> --scope project:<id>
tc problem unmute <problem-id>
tc test mute <test-id> --scope buildType:<id>
tc investigation list --status active
```

---

### 2. 🟡 **Build Comparison & Diff** (MEDIUM PRIORITY)
**Inspired by:** `gh pr diff`, `gh release compare`  
**TeamCity API:** Build comparison endpoints

**Missing capabilities:**
- ❌ Compare two builds (changes, tests, artifacts)
- ❌ Show diff between builds
- ❌ Compare test results across builds

**Why this matters:**
- Helps identify what changed between builds
- Useful for debugging regressions
- Common debugging workflow

**Proposed commands:**
```bash
tc run compare <build1> <build2>
tc run diff <build1> <build2> --changes
tc run diff <build1> <build2> --tests
```

---

### 3. 🟡 **Build Dependencies & Artifact Dependencies** (MEDIUM PRIORITY)
**Inspired by:** GitHub Actions workflow dependencies  
**TeamCity API:** Build dependency chains, artifact dependencies

**Missing capabilities:**
- ❌ View build dependency chain
- ❌ View artifact dependencies
- ❌ Trigger dependent builds
- ❌ Visualize build graph

**Why this matters:**
- Complex projects have multi-stage build pipelines
- Need to understand dependency relationships
- Helps troubleshoot pipeline issues

**Proposed commands:**
```bash
tc run deps <build-id>  # Show dependency tree
tc run deps <build-id> --graph  # ASCII dependency graph
tc job deps <job-id>  # Show job dependencies
```

---

### 4. 🟡 **Enhanced Search & Filtering** (MEDIUM PRIORITY)
**Inspired by:** `gh search`, GitHub's powerful search syntax  
**TeamCity API:** Advanced locator syntax

**Missing capabilities:**
- ❌ Search builds by multiple criteria
- ❌ Search across projects
- ❌ Advanced filtering with complex queries
- ❌ Save search filters/queries

**Current limitation:** `tc run list` has basic filters but limited composability

**Why this matters:**
- Large TeamCity instances have thousands of builds
- Need better discovery and filtering
- Power users want advanced queries

**Proposed enhancements:**
```bash
tc search builds "branch:main AND status:failure AND user:alice"
tc search builds --failed --since 7d --project MyProject
tc run list --query "tag:release,branch:main,state:finished"
```

---

### 5. 🟢 **Build Statistics & Metrics** (LOW PRIORITY)
**Inspired by:** GitHub Insights, `gh run view` with timing info  
**TeamCity API:** `/app/rest/builds/{id}/statistics`

**Missing capabilities:**
- ❌ Build duration statistics
- ❌ Build step timing breakdown
- ❌ Historical performance trends
- ❌ Build success rate metrics

**Why this matters:**
- Helps identify slow builds
- Performance optimization
- Team metrics and reporting

**Proposed commands:**
```bash
tc run stats <build-id>  # Detailed timing breakdown
tc job stats <job-id> --since 30d  # Historical trends
tc run view <build-id> --stats  # Include stats in view
```

---

### 6. 🟢 **Build Configuration Templates** (LOW PRIORITY)
**Inspired by:** GitHub workflow templates  
**TeamCity API:** Template management

**Missing capabilities:**
- ❌ List available templates
- ❌ View template details
- ❌ Create job from template

**Why this matters:**
- Templates are key TeamCity feature
- Helps standardize build configs
- Useful for large organizations

**Proposed commands:**
```bash
tc template list
tc template view <template-id>
tc job create --from-template <template-id>
```

---

### 7. 🟡 **VCS Root Management** (MEDIUM PRIORITY)
**Inspired by:** `gh repo` commands  
**TeamCity API:** VCS roots endpoints

**Missing capabilities:**
- ❌ List VCS roots
- ❌ View VCS root details
- ❌ Check VCS connectivity
- ❌ Trigger VCS check

**Why this matters:**
- VCS issues are common build failures
- Need to diagnose connection problems
- Useful for repository migrations

**Proposed commands:**
```bash
tc vcs list
tc vcs view <vcs-root-id>
tc vcs check <vcs-root-id>
tc project vcs-roots <project-id>
```

---

### 8. 🟢 **User & Permission Management** (LOW PRIORITY)
**Inspired by:** `gh org` commands  
**TeamCity API:** Users, groups, roles

**Missing capabilities:**
- ❌ List users
- ❌ View user details
- ❌ Manage user roles
- ❌ List groups

**Why this matters:**
- Admins need user management
- Useful for onboarding/offboarding
- Permission auditing

**Proposed commands:**
```bash
tc user list
tc user view <username>
tc user role add <username> <role> --project <id>
tc group list
```

---

### 9. 🔴 **Build Problem Details** (HIGH PRIORITY)
**Inspired by:** `gh run view` showing detailed failure info  
**TeamCity API:** Build problems endpoint

**Missing capabilities:**
- ❌ List build problems separately from logs
- ❌ View problem details
- ❌ Filter builds by problem type
- ❌ Show problem history

**Why this matters:**
- Problems are first-class entities in TeamCity
- Better than parsing logs
- Structured error information

**Proposed commands:**
```bash
tc run problems <build-id>
tc run problems <build-id> --new  # Only new problems
tc problem view <problem-id>
tc problem history <problem-id>
```

---

### 10. 🟢 **Cleanup Rules Management** (LOW PRIORITY)
**Inspired by:** Repository settings management  
**TeamCity API:** Cleanup rules

**Missing capabilities:**
- ❌ View cleanup rules
- ❌ Configure cleanup policies

**Why this matters:**
- Disk space management
- Artifact retention policies
- Compliance requirements

**Proposed commands:**
```bash
tc project cleanup-rules <project-id>
tc job cleanup-rules <job-id>
```

---

### 11. 🟡 **Branch Management & Default Branch** (MEDIUM PRIORITY)
**Inspired by:** `gh repo edit --default-branch`  
**TeamCity API:** Branch specification, default branch

**Missing capabilities:**
- ❌ List tracked branches
- ❌ View branch specifications
- ❌ Set default branch for job

**Why this matters:**
- Branch-based development workflows
- Branch cleanup and archiving
- Understanding which branches TeamCity tracks

**Proposed commands:**
```bash
tc job branches <job-id>
tc job branch-spec <job-id>
tc job default-branch <job-id> [new-branch]
```

---

### 12. 🟢 **Build Triggers Management** (LOW PRIORITY)
**Inspired by:** GitHub Actions workflow triggers  
**TeamCity API:** Triggers configuration

**Missing capabilities:**
- ❌ List build triggers
- ❌ View trigger details
- ❌ Enable/disable triggers
- ❌ Test trigger conditions

**Why this matters:**
- Understanding why builds start
- Debugging unexpected builds
- Managing trigger configuration

**Proposed commands:**
```bash
tc job triggers <job-id>
tc job trigger view <trigger-id>
tc job trigger disable <trigger-id>
tc job trigger test <trigger-id>
```

---

### 13. 🟡 **Build Steps & Runner Details** (MEDIUM PRIORITY)
**Inspired by:** GitHub Actions job steps view  
**TeamCity API:** Build steps, runner configuration

**Missing capabilities:**
- ❌ List build steps for a configuration
- ❌ View individual step configuration
- ❌ Show step execution time in build

**Why this matters:**
- Understanding build process
- Debugging step failures
- Performance optimization

**Proposed commands:**
```bash
tc job steps <job-id>
tc job step view <job-id> <step-id>
tc run steps <build-id>  # Show executed steps with timing
```

---

### 14. 🟢 **Server & License Information** (LOW PRIORITY)
**Inspired by:** `gh api /meta`, system information commands  
**TeamCity API:** Server info, license

**Missing capabilities:**
- ❌ View server version
- ❌ Check license information
- ❌ View server plugins
- ❌ Server health check

**Why this matters:**
- Troubleshooting and support
- Compatibility checking
- License compliance

**Proposed commands:**
```bash
tc server info
tc server license
tc server plugins
tc server health
```

---

### 15. 🔴 **Interactive Build Selection** (HIGH PRIORITY)
**Inspired by:** `gh pr list` with interactive selection, `fzf` integration  
**Current gap:** No interactive pickers

**Missing capabilities:**
- ❌ Interactive build picker
- ❌ Interactive job picker
- ❌ Interactive project picker
- ❌ Fuzzy search in lists

**Why this matters:**
- Dramatically improves UX
- Reduces need to remember IDs
- Common pattern in modern CLIs (`gh`, `glab`, `az`)

**Proposed enhancement:**
```bash
tc run list --interactive  # Opens interactive picker
tc run start --interactive  # Pick job interactively
tc run log  # If no build ID, show picker
```

**Implementation note:** Could use libraries like `bubbletea` (used by many Go CLIs) or `promptui`

---

### 16. 🟡 **Favorites & Recent Items** (MEDIUM PRIORITY)
**Inspired by:** Browser history, shell history patterns  
**Current gap:** No concept of "recent" or "favorite" items

**Missing capabilities:**
- ❌ Remember recently viewed builds
- ❌ Save favorite jobs
- ❌ Quick access to common projects
- ❌ History of commands run

**Why this matters:**
- Reduces typing for common operations
- Improves productivity for power users
- Natural workflow for developers

**Proposed commands:**
```bash
tc run log --last  # Last build you viewed
tc job list --favorites
tc job favorite add <job-id>
tc history  # Show recent commands
```

---

### 17. 🟢 **Build Artifact Browsing** (LOW PRIORITY)
**Inspired by:** `gh release view` with asset listing  
**TeamCity API:** Artifact browsing

**Missing capabilities:**
- ❌ Browse artifact directory tree
- ❌ View artifact metadata (size, timestamp)
- ❌ Compare artifacts between builds

**Current implementation:** `tc run download` works but limited browsing

**Why this matters:**
- Large builds have many artifacts
- Need to find specific files
- Useful for verification before download

**Proposed enhancement:**
```bash
tc run artifacts <build-id>  # List all artifacts with details
tc run artifacts <build-id> --tree  # Tree view
tc run artifacts <build-id> --filter "*.jar"
```

---

### 18. 🟡 **Pending Changes** (MEDIUM PRIORITY)
**Inspired by:** Git/VCS status commands  
**TeamCity API:** Pending changes endpoint

**Missing capabilities:**
- ❌ View pending VCS changes not yet built
- ❌ See what commits are waiting
- ❌ Trigger builds for pending changes

**Why this matters:**
- Understand what's queued to build
- Manual trigger for pending changes
- Useful in branch-based workflows

**Proposed commands:**
```bash
tc changes pending --job <job-id>
tc changes pending --branch <branch>
tc run start <job-id> --pending  # Build pending changes
```

---

### 19. 🟢 **Build Metadata & Custom Fields** (LOW PRIORITY)
**Inspired by:** GitHub metadata, labels, custom properties  
**TeamCity API:** Build attributes

**Missing capabilities:**
- ❌ View build metadata/attributes
- ❌ Set custom build attributes
- ❌ Filter by custom metadata

**Why this matters:**
- Advanced use cases and integrations
- Custom tracking and reporting
- Extensibility

**Proposed commands:**
```bash
tc run metadata <build-id>
tc run metadata set <build-id> <key> <value>
```

---

### 20. 🟡 **Build Promotion/Labeling Workflow** (MEDIUM PRIORITY)
**Inspired by:** Release promotion workflows, `gh release` patterns  
**Current implementation:** Tags exist but no promotion workflow

**Missing capabilities:**
- ❌ Promote build through stages (dev → staging → prod)
- ❌ Label builds with environment targets
- ❌ Track deployment history
- ❌ Approve builds for promotion

**Why this matters:**
- CD pipelines need promotion workflows
- Audit trail for deployments
- Multi-environment deployments

**Proposed commands:**
```bash
tc run promote <build-id> --to production
tc run promote <build-id> --to staging --require-approval
tc run promotions <build-id>  # Show promotion history
```

**Note:** This might be better as enhanced tagging + metadata rather than new feature

---

## Comparison Matrix: GitHub CLI Patterns

| GitHub CLI Feature | TeamCity Equivalent | Implemented? | Priority |
|-------------------|---------------------|--------------|----------|
| `gh run watch` | `tc run watch` | ✅ Yes | - |
| `gh run list` | `tc run list` | ✅ Yes | - |
| `gh run view` | `tc run view` | ✅ Yes | - |
| `gh run download` | `tc run download` | ✅ Yes | - |
| `gh run cancel` | `tc run cancel` | ✅ Yes | - |
| `gh run rerun` | `tc run restart` | ✅ Yes | - |
| `gh workflow enable/disable` | `tc job pause/resume` | ✅ Yes | - |
| `gh pr checks` | Build status view | ✅ Partial | 🟡 Medium |
| `gh pr diff` | Build comparison | ❌ No | 🟡 Medium |
| `gh search` | Advanced search | ❌ No | 🟡 Medium |
| `gh pr comment` | `tc run comment` | ✅ Yes | - |
| `gh release view` | Build artifacts | ✅ Partial | 🟢 Low |
| `gh repo view` | `tc project view` | ✅ Yes | - |
| Issue assignment | Build investigation | ❌ No | 🔴 High |
| Interactive pickers | Interactive mode | ❌ No | 🔴 High |
| `gh pr review` | Build approval | ✅ Partial (queue approve) | - |

---

## Recommended Implementation Priority

### Phase 1: Critical UX Improvements (1-2 weeks)
1. **Build Investigation & Muting** - Essential team collaboration feature
2. **Build Problem Details** - Better error visibility
3. **Interactive Selection** - Massive UX improvement

### Phase 2: Common Workflows (2-3 weeks)
4. **Build Comparison/Diff** - Debugging tool
5. **VCS Root Management** - Common admin task
6. **Branch Management** - Branch-based workflows
7. **Enhanced Search** - Better discovery
8. **Build Steps & Timing** - Performance insights

### Phase 3: Advanced Features (3-4 weeks)
9. **Build Dependencies** - Complex pipeline support
10. **Pending Changes** - VCS integration
11. **Favorites/Recent** - Power user features
12. **Triggers Management** - Advanced configuration

### Phase 4: Nice-to-Have (Future)
13. **User Management** - Admin features
14. **Statistics/Metrics** - Reporting
15. **Templates** - Configuration management
16. **Cleanup Rules** - Admin features
17. **Server Info** - System commands

---

## Implementation Considerations

### Technical Debt to Address
1. **API Client:** Current implementation mixes HTTP calls with business logic
   - Consider creating dedicated API client layer
   - Better error handling and retries

2. **Output Formatting:** Some commands have inconsistent formatting
   - Standardize table output
   - Consistent JSON structure

3. **Testing:** Limited test coverage
   - Add integration tests for new features
   - Mock TeamCity API for unit tests

### UX Patterns to Adopt from `gh`
1. **Interactive Prompts:** Use for common operations when params missing
2. **Smart Defaults:** Latest build, current project, etc.
3. **Contextual Help:** Better help text with examples
4. **Progress Indicators:** For long operations
5. **Confirmation Prompts:** For destructive operations (with `--force` flag)

### Dependencies to Consider
- `bubbletea` or `promptui` - Interactive TUI components
- `survey` - Interactive prompts and selections
- `tablewriter` - Better table formatting
- `go-pretty` - Enhanced terminal output

---

## Features NOT Recommended

### Excluded Features (with reasons)
1. ❌ **Agent Management** - Explicitly out of scope
2. ❌ **Build Script Editing** - Too complex for CLI, use UI
3. ❌ **Full Configuration DSL** - TeamCity Kotlin DSL exists
4. ❌ **Plugin Management** - Admin feature, use UI
5. ❌ **Audit Log Access** - Security concern, use UI
6. ❌ **Cloud Profile Management** - Complex, use UI
7. ❌ **LDAP/Auth Config** - Security concern, use UI

---

## Metrics for Success

### User Adoption Metrics
- Command usage frequency
- User retention (return users)
- CLI vs UI usage ratio

### Developer Experience Metrics
- Time to complete common tasks
- Number of command invocations per task
- Error rate and retry frequency

### Feature-Specific Metrics
- Investigation assignment rate
- Mute usage for noise reduction
- Interactive mode adoption
- Search query complexity

---

## Next Steps

### Immediate Actions
1. ✅ Complete this gap analysis
2. ⬜ Share with stakeholders for feedback
3. ⬜ Prioritize Phase 1 features
4. ⬜ Create detailed design docs for top 3 features
5. ⬜ Set up feature flags for gradual rollout

### Long-term Planning
1. Create roadmap based on priorities
2. Establish contribution guidelines
3. Build community around the CLI
4. Consider plugin/extension system
5. Explore TeamCity Cloud support

---

## Appendix: Research Sources

### GitHub CLI Analysis
- Explored `gh` command structure via source code
- Analyzed user workflows and common patterns
- Reviewed community feedback and feature requests

### TeamCity API Research
- TeamCity REST API documentation
- Existing teamcity-cli implementation
- Common TeamCity user workflows
- Enterprise CI/CD patterns

### Similar Tools Analyzed
- GitLab CLI (`glab`)
- Jenkins CLI
- CircleCI CLI
- Buildkite CLI

---

**End of Analysis**
