# TeamCity CLI Enhancement Recommendations

**Executive Summary for Quick Review**

---

## 🎯 Top 3 High-Impact Features

Based on the comprehensive gap analysis, these three features would provide the most value to users:

### 1. 🥇 Build Investigation & Muting
**What:** Allow users to investigate build failures and mute known problems  
**Why:** Critical for team collaboration and noise reduction  
**Complexity:** Medium  
**API Support:** ✅ Full support via `/app/rest/investigations` and `/app/rest/mutes`

```bash
# Proposed commands
tc run investigate <build-id> --user alice
tc problem mute <problem-id> --scope project:MyProject
tc test mute <test-id> --until 2026-02-01
tc investigation list --active
```

**Use cases:**
- Team lead assigns build failure to developer
- Mute known flaky test while fixing
- Track who's investigating what
- Reduce notification noise

---

### 2. 🥈 Interactive Build/Job Selection
**What:** Interactive pickers for builds, jobs, and projects (like `gh` CLI)  
**Why:** Dramatically improves UX, reduces need to remember IDs  
**Complexity:** Medium  
**API Support:** ✅ Uses existing list endpoints

```bash
# Proposed UX
$ tc run log
? Select a build:
> [12345] MyProject_Build #42 - SUCCESS - main - 2min ago
  [12344] MyProject_Build #41 - FAILED - feature/test - 5min ago
  [12343] MyProject_Test #15 - SUCCESS - main - 10min ago
```

**Implementation:**
- Use `bubbletea` or `promptui` library
- Fuzzy search/filtering
- Show recent builds first
- Remember last selection

**Use cases:**
- Quickly view logs without looking up build ID
- Start builds without memorizing job names
- Natural workflow for developers

---

### 3. 🥉 Build Problem Details
**What:** View structured build problems separately from logs  
**Why:** Better error visibility, problems are first-class in TeamCity  
**Complexity:** Low  
**API Support:** ✅ Full support via build problems endpoint

```bash
# Proposed commands
tc run problems <build-id>
tc run problems <build-id> --new     # Only new problems
tc problem view <problem-id>         # Detailed problem info
tc problem history <problem-id>      # Problem history across builds
```

**Output example:**
```
Build #12345 - 3 Problems

ID      TYPE              DESCRIPTION                    NEW
─────   ───────────────   ────────────────────────────   ───
1234    Compilation       src/main.go:42 syntax error    ✓
1235    Test Failed       TestLogin timed out            
1236    Exit Code         Process exit code 1            ✓

Run 'tc problem view 1234' for details
```

**Use cases:**
- Quickly identify what went wrong
- Track problem patterns
- Better than grepping logs

---

## 📊 Features by Priority

### 🔴 High Priority (Implement First)
1. **Build Investigation & Muting** - Team collaboration essential
2. **Build Problem Details** - Better debugging experience  
3. **Interactive Selection** - Major UX improvement

### 🟡 Medium Priority (Implement Next)
4. **Build Comparison/Diff** - Compare changes between builds
5. **VCS Root Management** - List and manage VCS roots
6. **Branch Management** - View/manage tracked branches
7. **Enhanced Search** - Better filtering and discovery
8. **Build Steps & Timing** - Performance insights
9. **Pending Changes** - View uncommitted changes waiting to build
10. **Build Promotion Workflow** - Track deployment stages

### 🟢 Low Priority (Nice to Have)
11. **Build Dependencies** - Show dependency chains
12. **Favorites/Recent** - Quick access to common items
13. **Build Statistics** - Historical metrics
14. **Templates** - Template management
15. **Triggers Management** - Enable/disable triggers
16. **User Management** - Admin features
17. **Server Info** - System information
18. **Cleanup Rules** - Retention policies

---

## 🚀 Recommended Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
**Goal:** Core improvements that benefit all users

- [ ] Implement build problem details view
- [ ] Add investigation command structure
- [ ] Create interactive picker framework
- [ ] Improve error handling and messaging

**Deliverables:**
- `tc run problems <id>`
- `tc run investigate <id>`
- Basic interactive mode for `tc run log`

---

### Phase 2: Team Collaboration (Week 3-4)
**Goal:** Enable team workflows

- [ ] Complete investigation features (list, assign, resolve)
- [ ] Implement muting (problems and tests)
- [ ] Add investigation status to `tc run view`
- [ ] Enhanced `tc run list` with problem indicators

**Deliverables:**
- Full investigation workflow
- Muting capability
- Team visibility into assigned issues

---

### Phase 3: Developer Experience (Week 5-6)
**Goal:** Make CLI more pleasant to use

- [ ] Expand interactive pickers to all commands
- [ ] Add fuzzy search/filtering
- [ ] Implement favorites/recent items
- [ ] Better progress indicators for long operations

**Deliverables:**
- Interactive mode across all commands
- Faster common workflows
- Better visual feedback

---

### Phase 4: Advanced Features (Week 7-8)
**Goal:** Power user and debugging tools

- [ ] Build comparison and diff
- [ ] VCS root management
- [ ] Branch management
- [ ] Enhanced search with saved queries
- [ ] Build steps and timing details

**Deliverables:**
- Advanced debugging tools
- Better pipeline visibility
- Configuration management

---

## 📐 Design Principles

### Inspired by GitHub CLI
1. **Interactive by default** - Prompt for missing required args
2. **Smart defaults** - Latest build, current project
3. **Web escape hatch** - `--web` flag to open in browser
4. **JSON always available** - `--json` for scripting
5. **Consistent flags** - Same flags mean same thing everywhere

### TeamCity-Specific
1. **Build-centric** - Builds are the primary object
2. **Server-aware** - Handle multiple TeamCity servers
3. **CI/CD friendly** - Environment variables for automation
4. **Non-interactive mode** - `--no-input` for scripts

---

## 🔧 Technical Implementation Notes

### New Dependencies to Consider

**For Interactive UI:**
```go
github.com/charmbracelet/bubbletea  // TUI framework (used by gh, glab)
github.com/charmbracelet/bubbles    // TUI components
github.com/charmbracelet/lipgloss   // Styling
```

**For Better Output:**
```go
github.com/olekukonko/tablewriter   // Better tables
github.com/fatih/color             // Already used, keep
```

**For Fuzzy Search:**
```go
github.com/sahilm/fuzzy            // Fuzzy matching
```

### API Client Refactoring

Current structure mixes API calls with command logic. Consider:

```
internal/
  api/
    client.go           # Base HTTP client
    builds.go           # Build operations
    investigations.go   # NEW: Investigation operations
    problems.go         # NEW: Problem operations
    vcs.go             # NEW: VCS operations
  cmd/
    run/               # Commands
    investigation/     # NEW: Investigation commands
    problem/           # NEW: Problem commands
```

### Testing Strategy
1. **Unit tests** - Mock API responses
2. **Integration tests** - Against test TeamCity server
3. **E2E tests** - Real workflows
4. **Snapshot tests** - Output formatting

---

## 📈 Success Metrics

### Adoption Metrics
- **Command usage:** Track most-used commands
- **Interactive mode adoption:** % of commands using pickers
- **Multi-user teams:** Teams using investigation features

### Quality Metrics
- **Error rate:** Failed API calls, invalid inputs
- **Time to task:** How long common workflows take
- **Support tickets:** CLI-related issues

### Feature-Specific Metrics
- **Investigations:** How many build failures get investigated
- **Mutes:** Reduction in notification noise
- **Interactive mode:** Adoption rate vs typed IDs

---

## 💡 Quick Wins (Can Implement Today)

These are small improvements that don't require new API endpoints:

### 1. Better Error Messages
**Current:** Generic HTTP error codes  
**Improved:** Helpful error messages with suggestions

```bash
# Before
Error: 404 Not Found

# After  
Error: Build 12345 not found
Suggestions:
  - Check the build ID with 'tc run list'
  - The build may have been deleted
  - You may not have permission to view this build
```

### 2. Progress Indicators
**Current:** Silent operations  
**Improved:** Show progress for long operations

```bash
$ tc run download 12345
Downloading artifacts... ⠸
[========>    ] 3 of 5 files (60%)
```

### 3. Smart Defaults
**Current:** All arguments required  
**Improved:** Default to latest/current

```bash
# Before
tc run log 12345

# After (defaults to latest build)
tc run log
# Or defaults to latest for specific job
tc run log --job MyBuild
```

### 4. Helpful Prompts
**Current:** Error if argument missing  
**Improved:** Prompt for required info

```bash
$ tc run cancel
? Enter build ID to cancel: _
```

### 5. Better Table Formatting
**Current:** Basic spacing  
**Improved:** Aligned columns, better borders

```bash
# Before
ID    STATUS  BRANCH
12345 SUCCESS main
12344 FAILED  feature

# After
╭────────┬─────────┬─────────────╮
│ ID     │ STATUS  │ BRANCH      │
├────────┼─────────┼─────────────┤
│ 12345  │ ✓ SUCCESS│ main       │
│ 12344  │ ✗ FAILED │ feature    │
╰────────┴─────────┴─────────────╯
```

---

## ❌ What NOT to Build

Features that are out of scope or better handled elsewhere:

1. **Agent Management** - Explicitly excluded per requirements
2. **Full Build Configuration Editing** - Too complex, use UI or Kotlin DSL
3. **Plugin Development/Management** - Admin task, use UI
4. **Security/Auth Configuration** - Sensitive, use UI
5. **Audit Log Access** - Compliance/security concern
6. **Cloud Infrastructure Management** - Too complex for CLI
7. **Database Administration** - Server-side task

---

## 🤝 Community & Contribution

### Contribution Opportunities
1. **Good First Issues:**
   - Better error messages
   - Additional output formats
   - Documentation improvements

2. **Feature Development:**
   - Investigation commands
   - Interactive pickers
   - Build comparison

3. **Testing:**
   - Integration tests
   - E2E test scenarios
   - Performance testing

### Documentation Needs
- [ ] API usage examples for each command
- [ ] Common workflow tutorials
- [ ] Troubleshooting guide
- [ ] Video demos of key features
- [ ] Migration guide from other tools

---

## 📚 Additional Resources

### Related Tools to Study
- **gh (GitHub CLI)** - Best-in-class CLI UX
- **glab (GitLab CLI)** - Similar domain, good patterns
- **jenkins-cli** - Legacy CI CLI for comparison
- **CircleCI CLI** - Modern CI CLI approach

### TeamCity Resources
- [REST API Documentation](https://www.jetbrains.com/help/teamcity/rest/teamcity-rest-api-documentation.html)
- [TeamCity Plugin Development](https://plugins.jetbrains.com/docs/teamcity/developing-teamcity-plugins.html)
- [TeamCity Community Forum](https://teamcity-support.jetbrains.com/hc/en-us/community/topics)

### Go Libraries
- [Cobra](https://github.com/spf13/cobra) - CLI framework (already used)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Survey](https://github.com/AlecAivazis/survey) - Interactive prompts
- [Go-pretty](https://github.com/jedib0t/go-pretty) - Terminal output

---

## Summary

The teamcity-cli is already a solid foundation with good coverage of core TeamCity operations. The main gaps are:

**Critical Missing Features:**
1. ⚠️ **Build Investigation & Muting** - Essential for teams
2. ⚠️ **Interactive Selection** - Major UX improvement  
3. ⚠️ **Build Problems** - Better error visibility

**Recommended First Steps:**
1. Implement build problem details (easiest, high value)
2. Add investigation commands (medium complexity, high value)
3. Create interactive picker framework (medium complexity, very high value)

This analysis provides a clear roadmap for enhancing teamcity-cli to match and exceed the capabilities of comparable tools while maintaining focus on TeamCity-specific workflows.

---

**Questions or Feedback?**
This is a living document. Please contribute additional insights or prioritization feedback.
