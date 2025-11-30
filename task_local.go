package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// RunLocal processes .js.map files from a local directory
func (c *Collector) RunLocal() error {
	// Find all .js.map files in the directory
	mapFiles, err := findMapFiles(c.InputDir)
	if err != nil {
		return fmt.Errorf("find map files: %v", err)
	}

	if len(mapFiles) == 0 {
		c.Logger.Warn("no .js.map files found in directory", zap.String("dir", c.InputDir))
		return nil
	}

	c.Logger.Info("found source map files", zap.Int("count", len(mapFiles)))

	for _, mapFile := range mapFiles {
		c.Logger.Debug("processing source map", zap.String("file", mapFile))
		err := c.processLocalMap(mapFile)
		if err != nil {
			c.Logger.Error("failed to process source map", zap.String("file", mapFile), zap.Error(err))
		}
	}

	return nil
}

// findMapFiles recursively finds all .js.map files in the given directory
func findMapFiles(dir string) ([]string, error) {
	var mapFiles []string

	err := filepath.Walk(dir, func(fpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(fpath, ".js.map") {
			mapFiles = append(mapFiles, fpath)
		}
		return nil
	})

	return mapFiles, err
}

// processLocalMap reads a local .js.map file and extracts source files
func (c *Collector) processLocalMap(mapFilePath string) error {
	data, err := os.ReadFile(mapFilePath)
	if err != nil {
		return fmt.Errorf("read source map file: %v", err)
	}

	var m SourceMap
	err = json.Unmarshal(data, &m)
	if err != nil {
		return fmt.Errorf("parse JSON: %v", err)
	}

	// Get the base name from the map file path for organizing output
	baseName := filepath.Base(filepath.Dir(mapFilePath))
	if baseName == "." || baseName == "" {
		baseName = "local"
	}

	for i, fname := range m.FileNames {
		// Sanitize the filename to prevent directory traversal attacks
		fname = strings.ReplaceAll(fname, "../", "")
		fname = strings.ReplaceAll(fname, "..\\", "")
		fname = strings.ReplaceAll(fname, "webpack://", "")
		fname = strings.ReplaceAll(fname, "://", "")
		
		// Normalize path separators for the current OS
		fname = filepath.FromSlash(fname)
		
		// Clean and validate the path to ensure it stays within output directory
		fname = filepath.Clean(fname)
		if filepath.IsAbs(fname) {
			fname = strings.TrimLeft(fname, "/\\")
		}
		
		fname = filepath.Join(c.Output, baseName, fname)

		if i >= len(m.Contents) {
			return errors.New("sources array is longer than sourcesContent array")
		}
		if strings.HasPrefix(fname, "external ") {
			c.Logger.Warn("skipping external source", zap.String("file", fname))
			continue
		}

		parent := filepath.Dir(fname)
		err = os.MkdirAll(parent, 0770)
		if err != nil {
			return fmt.Errorf("create dir: %v", err)
		}

		err = os.WriteFile(fname, []byte(m.Contents[i]), 0660)
		if err != nil {
			return fmt.Errorf("write file: %v", err)
		}

		c.Logger.Debug("extracted source file", zap.String("file", fname))
	}

	return nil
}
