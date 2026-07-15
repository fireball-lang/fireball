package lsp

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/fireball-lang/protocol"
	"github.com/fsnotify/fsnotify"
	"go.lsp.dev/uri"
)

type NativeWatcher struct {
	watcher *fsnotify.Watcher
	log     *slog.Logger

	Create func(context.Context, *protocol.CreateFilesParams) error
	Delete func(context.Context, *protocol.DeleteFilesParams) error
}

func NewNativeWatcher(log *slog.Logger) *NativeWatcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err.Error())
	}

	n := &NativeWatcher{
		watcher: watcher,
		log:     log,
	}

	go n.process()

	return n
}

func (n *NativeWatcher) process() {
	for {
		select {
		case ev, ok := <-n.watcher.Events:
			if !ok {
				return
			}

			if ev.Op.Has(fsnotify.Create) {
				info, err := os.Stat(ev.Name)
				if err != nil {
					n.log.Error("fsnotify: os.Stat() failed on Create", "error", err.Error())
					continue
				}

				if info.IsDir() {
					n.AddRecursive(ev.Name)

					if n.Create != nil {
						files := make([]protocol.FileCreate, 0)

						_ = filepath.WalkDir(ev.Name, func(path string, d os.DirEntry, walkErr error) error {
							if walkErr != nil {
								return walkErr
							}

							if d.IsDir() {
								return nil
							}

							files = append(files, protocol.FileCreate{URI: string(uri.File(path))})

							return nil
						})

						if len(files) > 0 {
							_ = n.Create(context.Background(), &protocol.CreateFilesParams{Files: files})
						}
					}
				} else {
					if n.Create != nil {
						_ = n.Create(context.Background(), &protocol.CreateFilesParams{Files: []protocol.FileCreate{{URI: string(uri.File(ev.Name))}}})
					}
				}
			} else if ev.Op.Has(fsnotify.Remove) || ev.Op.Has(fsnotify.Rename) {
				n.Remove(ev.Name)

				if n.Delete != nil {
					_ = n.Delete(context.Background(), &protocol.DeleteFilesParams{Files: []protocol.FileDelete{{URI: string(uri.File(ev.Name))}}})
				}
			}

		case err, ok := <-n.watcher.Errors:
			if !ok {
				return
			}

			if n.log != nil {
				n.log.Error("fsnotify: error", "error", err.Error())
			}
		}
	}
}

func (n *NativeWatcher) Add(path string) {
	err := n.watcher.Add(path)
	if err != nil {
		n.log.Error("fsnotify: failed to add watch", "path", path, "error", err.Error())
	}
}

func (n *NativeWatcher) AddRecursive(root string) {
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			n.Add(path)
		}

		return nil
	})
}

func (n *NativeWatcher) Remove(path string) bool {
	err := n.watcher.Remove(path)

	if err != nil {
		if errors.Is(err, fsnotify.ErrNonExistentWatch) {
			return false
		}

		panic(err.Error())
	}

	return true
}

func (n *NativeWatcher) Close() {
	err := n.watcher.Close()
	if err != nil {
		panic(err.Error())
	}
}
