package vault

import (
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// SettingsFile is the hidden per-vault settings file. It travels with the vault
// (unlike the user config), so it holds state and options that belong to the
// vault itself.
const SettingsFile = ".task-cli.yaml"

// Settings are the per-vault settings. Unknown keys are ignored so older
// binaries keep working with vaults written by newer ones.
type Settings struct {
	NextID int64 `yaml:"next_id"`
}

func (s *Store) settingsPath() string { return filepath.Join(s.Root, SettingsFile) }

// Settings loads the vault settings; a missing file yields zero values.
func (s *Store) Settings() (Settings, error) {
	var settings Settings
	err := decodeYAMLFile(s.settingsPath(), &settings)
	return settings, err
}

// setNextID updates only next_id, keeping comments and other keys intact.
func (s *Store) setNextID(next int64) error {
	return editYAMLFile(s.settingsPath(), "task-cli vault settings", func(m *yaml.Node) {
		setMappingValue(m, "next_id", strconv.FormatInt(next, 10), "!!int")
	})
}
