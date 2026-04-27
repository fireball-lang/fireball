package core

import (
	"embed"
	"errors"
	"io"
	"os"
	path2 "path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

func IsFilepathInside(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	return err == nil && !strings.HasPrefix(rel, "..")
}

func IsNil(v any) bool {
	if v == nil {
		return true
	}

	switch reflect.TypeOf(v).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Slice, reflect.Func:
		return reflect.ValueOf(v).IsNil()
	default:
		return false
	}
}

func ExtractVersionedEmbedFs(path, fsRoot string, fs embed.FS) error {
	defer Scope()()

	err := os.MkdirAll(path, 0750)
	if err != nil {
		return err
	}

	// Check version
	versionInfoFile, err := os.Open(filepath.Join(path, "version_info.txt"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	defer func() {
		if versionInfoFile != nil {
			_ = versionInfoFile.Close()
		}
	}()

	if err == nil {
		found, err := io.ReadAll(versionInfoFile)
		if err != nil {
			return err
		}

		expected, err := fs.ReadFile(path2.Join(fsRoot, "version_info.txt"))
		if err != nil {
			return err
		}

		if slices.Equal(found, expected) {
			return nil
		}
	}

	// Remove folder
	err = os.RemoveAll(path)
	if err != nil {
		return err
	}

	// Create folder
	err = os.MkdirAll(path, 0750)
	if err != nil {
		return err
	}

	// Copy files
	return extractEmbedFsFolder(path, fsRoot, fs)
}

func extractEmbedFsFolder(to, from string, fs embed.FS) error {
	// Create folder
	err := os.MkdirAll(to, 0750)
	if err != nil {
		return err
	}

	// Read entries
	entries, err := fs.ReadDir(from)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		to := filepath.Join(to, entry.Name())
		from := path2.Join(from, entry.Name())

		if entry.IsDir() {
			// Recurse folder
			err := extractEmbedFsFolder(to, from, fs)
			if err != nil {
				return err
			}
		} else {
			// Copy file
			err := extractEmbedFsFile(to, from, fs)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func extractEmbedFsFile(to, from string, fs embed.FS) error {
	// Create file
	toFile, err := os.Create(to)
	if err != nil {
		return err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer toFile.Close()

	// Open file
	fromFile, err := fs.Open(from)
	if err != nil {
		return err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer fromFile.Close()

	// Copy
	_, err = io.Copy(toFile, fromFile)
	return err
}
