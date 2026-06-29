package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	agentSkillsRoot          = ".agents"
	agentSkillsMaxDepth      = 4
	agentSkillsMaxFiles      = 40
	agentSkillsMaxFileBytes  = 64 * 1024
	agentSkillsMaxTotalBytes = 256 * 1024
)

type agentSkillsEnvelope struct {
	Skills         []agentSkill `json:"skills"`
	Truncated      bool         `json:"truncated"`
	MaxFiles       int          `json:"max_files"`
	MaxFileBytes   int          `json:"max_file_bytes"`
	MaxTotalBytes  int          `json:"max_total_bytes"`
	TotalBytesRead int          `json:"total_bytes_read"`
}

type agentSkill struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Size      int    `json:"size"`
	Truncated bool   `json:"truncated"`
}

type agentSkillSource struct {
	Root        string
	DisplayRoot string
	Label       string
}

type agentSkillFile struct {
	Path        string
	DisplayPath string
}

func listAgentSkills(workspace string) (string, error) {
	sources := []agentSkillSource{}
	workspace = strings.TrimSpace(workspace)
	if workspace != "" {
		root, err := validateWorkspaceRoot(workspace)
		if err != nil {
			return "", err
		}
		sources = append(sources, agentSkillSource{
			Root:        filepath.Join(root, agentSkillsRoot),
			DisplayRoot: ".agents",
			Label:       ".agents",
		})
	}
	if globalRoot := globalAgentSkillsRoot(); globalRoot != "" {
		sources = append(sources, agentSkillSource{
			Root:        globalRoot,
			DisplayRoot: ".agents/global",
			Label:       "global .agents",
		})
	}

	paths := []agentSkillFile{}
	envelope := agentSkillsEnvelope{
		Skills:        []agentSkill{},
		MaxFiles:      agentSkillsMaxFiles,
		MaxFileBytes:  agentSkillsMaxFileBytes,
		MaxTotalBytes: agentSkillsMaxTotalBytes,
	}
	for _, source := range sources {
		if len(paths) >= agentSkillsMaxFiles {
			envelope.Truncated = true
			break
		}
		collected, truncated, err := collectAgentSkillFiles(source, agentSkillsMaxFiles-len(paths))
		if err != nil {
			return "", err
		}
		if truncated {
			envelope.Truncated = true
		}
		paths = append(paths, collected...)
	}
	if len(paths) == 0 {
		return marshalAgentSkillsEnvelope(envelope)
	}
	sort.Slice(paths, func(i, j int) bool {
		return strings.ToLower(paths[i].DisplayPath) < strings.ToLower(paths[j].DisplayPath)
	})

	for _, item := range paths {
		if envelope.TotalBytesRead >= agentSkillsMaxTotalBytes {
			envelope.Truncated = true
			break
		}
		remaining := agentSkillsMaxTotalBytes - envelope.TotalBytesRead
		limit := agentSkillsMaxFileBytes
		if remaining < limit {
			limit = remaining
		}
		file, err := os.Open(item.Path)
		if err != nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(file, int64(limit)+1))
		_ = file.Close()
		if readErr != nil {
			continue
		}
		fileTruncated := len(data) > limit
		if fileTruncated {
			data = data[:limit]
			envelope.Truncated = true
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		envelope.TotalBytesRead += len(data)
		envelope.Skills = append(envelope.Skills, agentSkill{
			ID:        skillIDFromPath(item.DisplayPath),
			Name:      skillNameFromPath(item.DisplayPath),
			Path:      item.DisplayPath,
			Content:   content,
			Size:      len(data),
			Truncated: fileTruncated,
		})
	}
	return marshalAgentSkillsEnvelope(envelope)
}

func collectAgentSkillFiles(source agentSkillSource, remainingFiles int) ([]agentSkillFile, bool, error) {
	if remainingFiles <= 0 {
		return nil, true, nil
	}
	info, err := os.Stat(source.Root)
	if errors.Is(err, os.ErrNotExist) {
		return []agentSkillFile{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.IsDir() {
		return nil, false, fmt.Errorf("%s is not a directory", source.Label)
	}

	paths := []agentSkillFile{}
	truncated := false
	err = filepath.WalkDir(source.Root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(source.Root, path)
		if err != nil || rel == "." {
			return nil
		}
		depth := strings.Count(filepath.ToSlash(rel), "/") + 1
		if entry.IsDir() {
			if depth > agentSkillsMaxDepth || entry.Type()&os.ModeSymlink != 0 {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !isAgentSkillMarkdown(entry.Name()) {
			return nil
		}
		if depth > agentSkillsMaxDepth {
			return nil
		}
		if len(paths) >= remainingFiles {
			truncated = true
			return filepath.SkipAll
		}
		displayPath := strings.TrimRight(source.DisplayRoot, "/") + "/" + filepath.ToSlash(rel)
		paths = append(paths, agentSkillFile{Path: path, DisplayPath: displayPath})
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return paths, truncated, nil
}

func globalAgentSkillsRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, "token-market", agentSkillsRoot)
}

func isAgentSkillMarkdown(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".md")
}

func skillIDFromPath(path string) string {
	id := strings.Trim(strings.ToLower(path), "/")
	id = strings.TrimPrefix(id, ".agents/")
	id = strings.TrimSuffix(id, ".md")
	id = strings.ReplaceAll(id, "/", "__")
	id = strings.ReplaceAll(id, "\\", "__")
	return id
}

func skillNameFromPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) >= 3 && strings.EqualFold(parts[len(parts)-1], "skill.md") {
		return parts[len(parts)-2]
	}
	name := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1]))
	if strings.TrimSpace(name) == "" {
		return path
	}
	return name
}

func marshalAgentSkillsEnvelope(envelope agentSkillsEnvelope) (string, error) {
	if envelope.Skills == nil {
		envelope.Skills = []agentSkill{}
	}
	if envelope.MaxFiles == 0 {
		envelope.MaxFiles = agentSkillsMaxFiles
	}
	if envelope.MaxFileBytes == 0 {
		envelope.MaxFileBytes = agentSkillsMaxFileBytes
	}
	if envelope.MaxTotalBytes == 0 {
		envelope.MaxTotalBytes = agentSkillsMaxTotalBytes
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
