package skill

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Installer struct {
	skillsDir string
}

func NewInstaller(skillsDir string) *Installer {
	return &Installer{skillsDir: skillsDir}
}

func (i *Installer) Install(url string) (*SkillInfo, error) {
	// Extract name from URL
	name := extractNameFromURL(url)
	if name == "" {
		return nil, fmt.Errorf("could not determine skill name from URL: %s", url)
	}

	destDir := filepath.Join(i.skillsDir, name)
	if _, err := os.Stat(destDir); err == nil {
		return nil, fmt.Errorf("skill %q already exists", name)
	}

	// Clone
	cmd := exec.Command("git", "clone", "--depth=1", url, destDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git clone failed: %s: %w", string(output), err)
	}

	// Validate
	skillMD := filepath.Join(destDir, "SKILL.md")
	if _, err := os.Stat(skillMD); os.IsNotExist(err) {
		os.RemoveAll(destDir)
		return nil, fmt.Errorf("invalid skill: SKILL.md not found")
	}

	data, err := os.ReadFile(skillMD)
	if err != nil {
		return nil, err
	}

	info := &SkillInfo{Name: name, Dir: destDir}
	parseSkillMD(string(data), info)
	return info, nil
}

func (i *Installer) Remove(name string) error {
	dir := filepath.Join(i.skillsDir, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("skill %q not found", name)
	}
	return os.RemoveAll(dir)
}

func extractNameFromURL(url string) string {
	// Handle github.com/user/repo or https://github.com/user/repo.git
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
