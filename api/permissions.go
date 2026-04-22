package api

// KnownPermissions maps TeamCity permission enum names to their server-provided descriptions.
// Keys mirror ALLOWED_PERMISSIONS in oauth-server/.../OAuthServerConstants.java; values are
// verbatim from server-model/.../Permission.java. Keep in sync when either list changes.
var KnownPermissions = map[string]string{
	"VIEW_PROJECT":                       "View project and all parent projects",
	"VIEW_BUILD_CONFIGURATION_SETTINGS":  "View build configuration settings",
	"VIEW_AGENT_DETAILS":                 "View agent details",
	"RUN_BUILD":                          "Run build",
	"CANCEL_BUILD":                       "Stop build / remove from queue",
	"TAG_BUILD":                          "Tag build",
	"COMMENT_BUILD":                      "Comment build",
	"PIN_UNPIN_BUILD":                    "Pin / unpin build",
	"PATCH_BUILD_SOURCES":                "Change build source code with a custom patch",
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
