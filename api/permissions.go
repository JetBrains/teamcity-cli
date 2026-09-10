package api

// KnownPermissions lists PKCE-selectable permissions from TeamCity's Permission.java, excluding prohibited CHANGE_OWN_PROFILE.
var KnownPermissions = map[string]string{
	"ADMINISTER_AGENT":                                  "Administer build agent machines (e.g. reboot, view agent logs, etc.)",
	"ADMINISTER_AGENT_FOR_PROJECT":                      "Administer project agent machines (e.g. reboot, view agent logs, etc.)",
	"ARCHIVE_PROJECT":                                   "Archive / dearchive project",
	"ASSIGN_INVESTIGATION":                              "Assign / unassign investigation",
	"ASSIGN_USERS_ADD_SUBGROUPS":                        "Assign / unassign users to groups or change groups hierarchy",
	"AUTHORIZE_AGENT":                                   "Authorize agent",
	"AUTHORIZE_AGENT_FOR_PROJECT":                       "Authorize project agent",
	"BACKUP":                                            "Change backup settings and control backup process",
	"CANCEL_ANY_PERSONAL_BUILD":                         "Stop / remove from queue any personal build",
	"CANCEL_BUILD":                                      "Stop build / remove from queue",
	"CHANGE_AGENT_RUN_CONFIGURATION_POLICY":             "Change agent run configuration policy",
	"CHANGE_AGENT_RUN_CONFIGURATION_POLICY_FOR_PROJECT": "Change agent run configuration policy for project",
	"CHANGE_CLEANUP_RULES":                              "Change cleanup rules",
	"CHANGE_HTTPS_SETTINGS":                             "Change HTTPS settings",
	"CHANGE_SERVER_SETTINGS":                            "Change server settings",
	"CHANGE_USER":                                       "Modify user profile and roles",
	"CHANGE_USER_NOTIFICATIONS":                         "Change user / group notification rules",
	"CHANGE_USER_NOTIFICATIONS_IN_PROJECT":              "Change user / group notification rules in project",
	"CHANGE_USER_ROLES_IN_PROJECT":                      "Change user roles in project",
	"CHANGE_USERGROUP":                                  "Modify user group (name, description and roles)",
	"CHANGE_VCS_USERNAME_IN_PROJECT":                    "Change VCS usernames in project",
	"CLEAN_AGENT_SOURCES":                               "Clean sources on agent",
	"CLEAN_BUILD_CONFIGURATION_SOURCES":                 "Clean build configuration sources",
	"COMMENT_BUILD":                                     "Comment build",
	"CONFIGURE_SERVER_DATA_CLEANUP":                     "Configure server data cleanup",
	"CONNECT_TO_AGENT":                                  "Invoke interactive agent terminals",
	"CREATE_DELETE_VCS_ROOT":                            "Create / delete VCS root",
	"CREATE_SUB_PROJECT":                                "Create subproject",
	"CREATE_USER":                                       "Create user account",
	"CREATE_USERGROUP":                                  "Create user group",
	"CUSTOMIZE_BUILD_PARAMETERS":                        "Customize build parameters",
	"CUSTOMIZE_BUILD_REVISIONS":                         "Customize build revisions",
	"DELETE_SUB_PROJECT":                                "Delete subproject",
	"DELETE_USER":                                       "Delete user account",
	"DELETE_USERGROUP":                                  "Delete user group",
	"EDIT_ENFORCED_SETTINGS":                            "Enable / disable enforced settings in project",
	"EDIT_PROJECT":                                      "Edit project",
	"EDIT_VCS_MODIFICATION":                             "Edit VCS change description",
	"EDIT_VERSIONED_SETTINGS":                           "Enable / disable versioned settings",
	"ENABLE_DISABLE_AGENT":                              "Enable / disable agent",
	"ENABLE_DISABLE_AGENT_FOR_PROJECT":                  "Enable / disable agents associated with project",
	"IMPORT_PROJECTS":                                   "Import projects",
	"LABEL_BUILD":                                       "Manually label / merge build sources",
	"MANAGE_AGENT_CLOUDS":                               "Manage project's agent cloud profiles",
	"MANAGE_AGENT_POOLS":                                "Manage agent pools",
	"MANAGE_AGENT_POOLS_FOR_PROJECT":                    "Change agent pools associated with project",
	"MANAGE_AUTHENTICATION_SETTINGS":                    "Manage authentication settings",
	"MANAGE_BUILD_PROBLEM_INSTANCES":                    "Change build status",
	"MANAGE_BUILD_PROBLEMS":                             "Mute / unmute problems and tests in project",
	"MANAGE_CUSTOM_SSL_CERTIFICATES":                    "Manage custom SSL/HTTPS certificates",
	"MANAGE_EXPERIMENTAL_FEATURES":                      "Manage experimental features",
	"MANAGE_ROLES":                                      "Manage roles (create, delete, change permissions)",
	"MANAGE_SERVER_INSTALLATION":                        "Manage server installation: view logs, restart, etc.",
	"MANAGE_SERVER_LICENSES":                            "Manage server licenses",
	"PATCH_BUILD_SOURCES":                               "Change build source code with a custom patch",
	"PAUSE_ACTIVATE_BUILD_CONFIGURATION":                "Pause / activate build configuration",
	"PIN_UNPIN_BUILD":                                   "Pin / unpin build",
	"REMOVE_AGENT":                                      "Remove agent",
	"REMOVE_AGENT_FOR_PROJECT":                          "Remove project agent",
	"REMOVE_BUILD":                                      "Remove finished build",
	"REORDER_BUILD_QUEUE":                               "Reorder builds in queue",
	"RUN_BUILD":                                         "Run build",
	"START_STOP_CLOUD_AGENT":                            "Start / Stop cloud agent",
	"SUBMIT_SUPPORT_REQUEST_WITH_DIAGNOSTIC_DATA":       "Submit support requests with diagnostic data to JetBrains Support",
	"TAG_BUILD":                                         "Tag build",
	"VIEW_AGENT_CLOUDS":                                 "View cloud images and instances",
	"VIEW_AGENT_DETAILS":                                "View agent details",
	"VIEW_AGENT_DETAILS_FOR_PROJECT":                    "View project agents details",
	"VIEW_AGENT_USAGE_STATISTICS":                       "View agent usage statistics",
	"VIEW_ALL_USERS":                                    "View all registered users",
	"VIEW_AUDIT_LOG":                                    "View audit log",
	"VIEW_BUILD_CONFIGURATION_SETTINGS":                 "View build configuration settings",
	"VIEW_BUILD_RUNTIME_DATA":                           "View build runtime parameters and data",
	"VIEW_FILE_CONTENT":                                 "View VCS file content",
	"VIEW_PROJECT":                                      "View project and all parent projects",
	"VIEW_SERVER_ERRORS":                                "View server errors",
	"VIEW_SERVER_SETTINGS":                              "View server settings",
	"VIEW_USAGE_STATISTICS":                             "View usage statistics",
	"VIEW_USER_PROFILE":                                 "View user profile",
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
