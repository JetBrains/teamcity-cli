package auth

import (
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

// restrictedTokenLifetime is how long the token from an onboarding exchange lives; `teamcity auth login` gets a new one.
const restrictedTokenLifetime = 90 * 24 * time.Hour

// onboardingTokenPermissions are the global permissions of the token from an onboarding exchange (TW-105780).
// They are the permissions a TeamCity Cloud free account can have (its teamcity.security.globallyAllowedPermissions),
// without CREATE_OWN_ACCESS_TOKEN and CHANGE_OWN_PROFILE: with them the token could create more tokens, and the server
// rejects them in a restriction anyway. Every listed permission must be granted to the user, or the server rejects the token.
var onboardingTokenPermissions = []string{
	"RUN_BUILD", "CANCEL_BUILD", "CANCEL_ANY_PERSONAL_BUILD", "PIN_UNPIN_BUILD", "TAG_BUILD", "REMOVE_BUILD",
	"PAUSE_ACTIVATE_BUILD_CONFIGURATION", "COMMENT_BUILD", "CLEAN_BUILD_CONFIGURATION_SOURCES", "ASSIGN_INVESTIGATION",
	"VIEW_PROJECT", "VIEW_BUILD_CONFIGURATION_SETTINGS", "VIEW_FILE_CONTENT", "EDIT_VCS_MODIFICATION", "EDIT_PROJECT",
	"CHANGE_CLEANUP_RULES", "IMPORT_PROJECTS", "CUSTOMIZE_BUILD_PARAMETERS", "CUSTOMIZE_BUILD_REVISIONS",
	"CREATE_SUB_PROJECT", "DELETE_SUB_PROJECT", "CREATE_DELETE_VCS_ROOT", "CHANGE_USER_ROLES_IN_PROJECT", "LABEL_BUILD",
	"ARCHIVE_PROJECT", "MANAGE_BUILD_PROBLEMS", "MANAGE_BUILD_PROBLEM_INSTANCES", "VIEW_BUILD_RUNTIME_DATA",
	"MANAGE_AGENT_POOLS_FOR_PROJECT", "EDIT_VERSIONED_SETTINGS", "EDIT_ENFORCED_SETTINGS", "ENABLE_DISABLE_AGENT_FOR_PROJECT",
	"CHANGE_AGENT_RUN_CONFIGURATION_POLICY_FOR_PROJECT", "ADMINISTER_AGENT_FOR_PROJECT", "REMOVE_AGENT_FOR_PROJECT",
	"AUTHORIZE_AGENT_FOR_PROJECT", "VIEW_AGENT_DETAILS_FOR_PROJECT", "CREATE_USER", "DELETE_USER",
	"CHANGE_USER_NOTIFICATIONS_IN_PROJECT", "CHANGE_USER_NOTIFICATIONS", "CHANGE_AGENT_RUN_CONFIGURATION_POLICY",
	"CLEAN_AGENT_SOURCES", "ENABLE_DISABLE_AGENT", "AUTHORIZE_AGENT", "VIEW_AGENT_DETAILS", "VIEW_AGENT_USAGE_STATISTICS",
	"REMOVE_AGENT", "CREATE_USERGROUP", "DELETE_USERGROUP", "CHANGE_USERGROUP", "ASSIGN_USERS_ADD_SUBGROUPS", "MANAGE_ROLES",
	"ADMINISTER_AGENT", "VIEW_AUDIT_LOG", "REORDER_BUILD_QUEUE", "VIEW_SERVER_ERRORS", "VIEW_USAGE_STATISTICS",
	"MANAGE_AGENT_POOLS", "VIEW_USER_PROFILE", "VIEW_ALL_USERS", "VIEW_AGENT_CLOUDS", "MANAGE_AUTHENTICATION_SETTINGS",
	"MANAGE_CUSTOM_SSL_CERTIFICATES", "PATCH_BUILD_SOURCES", "MANAGE_AGENT_CLOUDS", "CHANGE_VCS_USERNAME_IN_PROJECT",
	"MANAGE_EXPERIMENTAL_FEATURES", "CONNECT_TO_AGENT",
}

// restrictToken trades exchanged, the unrestricted token from the one-time token exchange, for a token named name
// that has only onboardingTokenPermissions and expires at expires, then deletes exchanged.
// It returns the value of the token to use; exchanged.Value when the restricted token can't be created,
// for example on a server where the user lacks a listed permission, so onboarding still works.
func restrictToken(p *output.Printer, client *api.Client, exchanged *api.Token, name string, expires time.Time) string {
	restricted, err := client.CreateRestrictedToken(name, onboardingTokenPermissions, expires)
	if err != nil {
		p.Debug("Couldn't create a restricted token, keeping %s: %v", exchanged.Name, err)
		return exchanged.Value
	}
	if err := client.DeleteAPIToken(exchanged.Name); err != nil {
		p.Debug("Couldn't delete the exchange token %s: %v", exchanged.Name, err)
	}
	return restricted.Value
}
