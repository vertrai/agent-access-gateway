package starthermes

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed skills
var embeddedSkills embed.FS

// InstallSkills refreshes every embedded Hub Gateway skill in the Hermes
// private state directory. Files absent from the new embedded version are
// removed so upgrades cannot leave stale helpers behind.
func InstallSkills(home string) error {
	destinationRoot := filepath.Join(home, ".hermes", "skills")
	entries, err := embeddedSkills.ReadDir("skills")
	if err != nil {
		return fmt.Errorf("list embedded skills: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		source := "skills/" + entry.Name()
		destination := filepath.Join(destinationRoot, entry.Name())
		if err := os.MkdirAll(destinationRoot, 0o755); err != nil {
			return err
		}
		temporary, err := os.MkdirTemp(destinationRoot, "."+entry.Name()+"-*")
		if err != nil {
			return err
		}
		if err := extractSkill(source, temporary); err != nil {
			os.RemoveAll(temporary)
			return fmt.Errorf("install skill %s: %w", entry.Name(), err)
		}
		backup := destination + ".previous"
		_ = os.RemoveAll(backup)
		if _, err := os.Stat(destination); err == nil {
			if err := os.Rename(destination, backup); err != nil {
				os.RemoveAll(temporary)
				return err
			}
		}
		if err := os.Rename(temporary, destination); err != nil {
			_ = os.Rename(backup, destination)
			return err
		}
		_ = os.RemoveAll(backup)
	}
	return nil
}

func extractSkill(sourceRoot, destinationRoot string) error {
	return fs.WalkDir(embeddedSkills, sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative := strings.TrimPrefix(path, sourceRoot)
		relative = strings.TrimPrefix(relative, "/")
		destination := filepath.Join(destinationRoot, filepath.FromSlash(relative))
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		content, err := embeddedSkills.ReadFile(path)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(path, ".py") || strings.HasSuffix(path, ".sh") {
			mode = 0o755
		}
		return os.WriteFile(destination, content, mode)
	})
}
