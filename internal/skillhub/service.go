package skillhub

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

func (c *Client) Add(slug, version string, global, skipConfirm bool) error {
	fmt.Printf("%s %s\n", color.New(color.Bold).Sprint("📦 Installing skill:"), color.CyanString(slug))

	detail, err := c.GetSkillDetail(slug)
	if err != nil {
		return err
	}

	installVersion := version
	if installVersion == "" {
		installVersion = detail.LatestVersion.Version
	}

	fmt.Printf("%s %s v%s\n",
		color.New(color.Bold).Sprint("▸"),
		color.New(color.Bold).Sprint(detail.Skill.DisplayName),
		color.GreenString(installVersion),
	)
	fmt.Printf("  %s\n\n", detail.Skill.Description)

	fmt.Println(color.New(color.Bold).Sprint("⬇ Downloading..."))
	zipData, err := c.DownloadSkill(slug, installVersion)
	if err != nil {
		return fmt.Errorf("Download skill failed: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(zipData))
	fmt.Printf("  %s\n", color.New(color.FgHiBlack).Sprintf("SHA256: %s", hash))

	// Detect all installed agents
	agents := DetectAllInstalledAgents()
	if len(agents) > 0 {
		fmt.Printf("  %s ",
			color.New(color.FgHiBlack).Sprint("Detected agents:"),
		)
		var names []string
		for _, a := range agents {
			names = append(names, a.DisplayName)
		}
		fmt.Printf("%s\n", color.CyanString(strings.Join(names, ", ")))
	}

	// Install to global repository
	home, _ := os.UserHomeDir()
	globalRepo := filepath.Join(home, ".agents", "skills", slug)

	if _, err := os.Stat(globalRepo); err == nil {
		if !skipConfirm {
			fmt.Printf("Skill %s already exists. Overwrite? [y/N] ", color.CyanString(slug))
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(answer) != "y" {
				fmt.Println(color.YellowString("Installation cancelled."))
				return nil
			}
		}
		_ = os.RemoveAll(globalRepo)
	}

	fmt.Println(color.New(color.Bold).Sprint("📂 Extracting to global repository..."))
	if err := extractZip(zipData, globalRepo); err != nil {
		return fmt.Errorf("Extract failed: %w", err)
	}

	// Create symlinks (Unix/macOS) or copies (Windows) for all detected agents
	if runtime.GOOS == "windows" {
		fmt.Println(color.New(color.Bold).Sprint("📋 Copying to agent directories..."))
	} else {
		fmt.Println(color.New(color.Bold).Sprint("🔗 Creating symlinks..."))
	}

	linkedCount := 0
	for _, agent := range agents {
		linkPath := filepath.Join(agent.GlobalSkillsDir, slug)

		// Skip if the link path is the same as global repo (e.g., Cline uses ~/.agents/skills)
		if linkPath == globalRepo {
			continue
		}

		// Ensure the skills directory exists
		if err := os.MkdirAll(agent.GlobalSkillsDir, 0755); err != nil {
			fmt.Printf("  %s Failed to create directory for %s: %v\n",
				color.YellowString("⚠"),
				agent.DisplayName,
				err,
			)
			continue
		}

		// Remove old symlink/directory if exists
		_ = os.RemoveAll(linkPath)

		// Create symlink (Unix/macOS) or copy directory (Windows)
		if err := linkOrCopyDir(globalRepo, linkPath); err != nil {
			fmt.Printf("  %s Failed to link to %s: %v\n",
				color.YellowString("⚠"),
				agent.DisplayName,
				err,
			)
		} else {
			fmt.Printf("  %s %s -> %s\n",
				color.GreenString("✓"),
				color.CyanString(agent.DisplayName),
				color.New(color.FgHiBlack).Sprint(linkPath),
			)
			linkedCount++
		}
	}

	fmt.Println()
	fmt.Println(color.New(color.Bold, color.FgGreen).Sprint("✅ Installation complete!"))
	fmt.Printf("  %s %s\n",
		color.New(color.FgHiBlack).Sprint("Global repository:"),
		color.CyanString(globalRepo),
	)
	if linkedCount > 0 {
		linkLabel := "Symlinks:"
		if runtime.GOOS == "windows" {
			linkLabel = "Copies:"
		}
		fmt.Printf("  %s Linked to %d agent(s)\n",
			color.New(color.FgHiBlack).Sprint(linkLabel),
			linkedCount,
		)
	}
	fmt.Println()

	fmt.Println(color.New(color.Bold).Sprint("Usage:"))
	fmt.Println("  The skill is now available in all your agent sessions.")
	fmt.Println("  Restart your agent if needed to load the new skill.")

	return nil
}

func extractZip(zipData []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return err
	}

	topLevelDirs := make(map[string]bool)
	for _, file := range reader.File {
		parts := strings.Split(file.Name, "/")
		if len(parts) > 0 && parts[0] != "" {
			topLevelDirs[parts[0]] = true
		}
	}

	var stripPrefix string
	if len(topLevelDirs) == 1 {
		for dir := range topLevelDirs {
			stripPrefix = dir + "/"
		}
	}

	for _, file := range reader.File {
		fileName := file.Name
		if stripPrefix != "" {
			fileName = strings.TrimPrefix(fileName, stripPrefix)
		}
		if fileName == "" {
			continue
		}

		targetPath := filepath.Join(targetDir, fileName)
		if file.FileInfo().IsDir() {
			_ = os.MkdirAll(targetPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}

		outFile, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
