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

		// Version mismatch - remove folder and recreate from scratch
		err = versionInfoFile.Close()
		if err != nil {
			return err
		}

		versionInfoFile = nil

		err = os.RemoveAll(path)
		if err != nil {
			return err
		}

		err = os.MkdirAll(path, 0750)
		if err != nil {
			return err
		}

		return recursiveLoopFolders(path, fsRoot, fs, extractEmbedFsFile)
	}

	// version_info.txt is missing (e.g. first extraction, or an older layout) - instead of
	// wiping the whole folder, sync files one by one, only overwriting the ones that differ
	return recursiveLoopFolders(path, fsRoot, fs, syncEmbedFsFile)
}

func recursiveLoopFolders(to, from string, fs embed.FS, fn func(to, from string, fs embed.FS) error) error {
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
			err := recursiveLoopFolders(to, from, fs, fn)
			if err != nil {
				return err
			}
		} else {
			// Sync file
			err := fn(to, from, fs)
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

func syncEmbedFsFile(to, from string, fs embed.FS) error {
	// Read expected contents
	expected, err := fs.ReadFile(from)
	if err != nil {
		return err
	}

	// Read existing contents (if any) and compare, skipping the write if they already match
	found, err := os.ReadFile(to)

	if err == nil && slices.Equal(found, expected) {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.WriteFile(to, expected, 0640)
}
