package config

import (
	"fmt"
	"maps"
)

func GetAlias(name string) (string, bool) {
	if cfg == nil || cfg.Aliases == nil {
		return "", false
	}
	exp, ok := cfg.Aliases[name]
	return exp, ok
}

func GetAllAliases() map[string]string {
	if cfg == nil || cfg.Aliases == nil {
		return nil
	}
	return maps.Clone(cfg.Aliases)
}

func IsShellAlias(name string) bool {
	exp, ok := GetAlias(name)
	return ok && len(exp) > 0 && exp[0] == '!'
}

func AddAlias(name, expansion string) error {
	if cfg.Aliases == nil {
		cfg.Aliases = make(map[string]string)
	}
	cfg.Aliases[name] = expansion
	return writeConfig()
}

func DeleteAlias(name string) error {
	if cfg.Aliases == nil {
		return fmt.Errorf("no such alias %q", name)
	}
	if _, ok := cfg.Aliases[name]; !ok {
		return fmt.Errorf("no such alias %q", name)
	}
	delete(cfg.Aliases, name)
	return writeConfig()
}
