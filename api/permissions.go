package api

// KnownPermissions maps TeamCity permission enum names to their server-provided descriptions.
// Keys are the permissions the CLI requests (see fallbackScopes in pkce.go); values are verbatim
// from server-model/.../Permission.java. Keep in sync with fallbackScopes when either changes.
var KnownPermissions = map[string]string{
	"VIEW_PROJECT":                       "View project and all parent projects",
	"VIEW_BUILD_CONFIGURATION_SETTINGS":  "View build configuration settings",
	"VIEW_BUILD_RUNTIME_DATA":            "View build runtime parameters and data",
	"VIEW_AGENT_DETAILS":                 "View agent details",
	"VIEW_AGENT_DETAILS_FOR_PROJECT":     "View project agents details",
	"VIEW_AGENT_CLOUDS":                  "View cloud images and instances",
	"RUN_BUILD":                          "Run build",
	"CANCEL_BUILD":                       "Stop build / remove from queue",
	"TAG_BUILD":                          "Tag build",
	"COMMENT_BUILD":                      "Comment build",
	"ASSIGN_INVESTIGATION":               "Assign / unassign investigation",
	"MANAGE_BUILD_PROBLEMS":              "Mute / unmute problems and tests in project",
	"PIN_UNPIN_BUILD":                    "Pin / unpin build",
	"PATCH_BUILD_SOURCES":                "Change build source code with a custom patch",
	"CUSTOMIZE_BUILD_PARAMETERS":         "Customize build parameters",
	"CUSTOMIZE_BUILD_REVISIONS":          "Customize build revisions",
	"REORDER_BUILD_QUEUE":                "Reorder builds in queue",
	"PAUSE_ACTIVATE_BUILD_CONFIGURATION": "Pause / activate build configuration",
	"EDIT_PROJECT":                       "Edit project",
	"CREATE_SUB_PROJECT":                 "Create subproject",
	"CREATE_DELETE_VCS_ROOT":             "Create / delete VCS root",
	"CONNECT_TO_AGENT":                   "Invoke interactive agent terminals",
	"ENABLE_DISABLE_AGENT":               "Enable / disable agent",
	"AUTHORIZE_AGENT":                    "Authorize agent",
	"ADMINISTER_AGENT":                   "Administer build agent machines (e.g. reboot, view agent logs, etc.)",
	"MANAGE_AGENT_POOLS":                 "Manage agent pools",
	"START_STOP_CLOUD_AGENT":             "Start / Stop cloud agent",
}

var permissionByDescription = func() map[string]string {
	out := make(map[string]string, len(KnownPermissions))
	for name, desc := range KnownPermissions {
		out[desc] = name
	}
	return out
}()

// PermissionEnum returns the enum name for a server-provided permission description, or "" if unknown.
func PermissionEnum(description string) string {
	return permissionByDescription[description]
}

// PermissionEditProject is the locator-form value for `userPermission:(permission:<name>,...)` (lowercase).
const PermissionEditProject = "edit_project"
