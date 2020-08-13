package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/gohugoio/hugo/watcher"
	"github.com/pkg/errors"
	"log"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type Dispatcher func([]fsnotify.Event)

type FileWatcher struct {
	w                 *watcher.Batcher
	folderDispatchers map[string]Dispatcher
	fileDispatchers   map[string]Dispatcher
	lock              sync.RWMutex
}

func NewFileWatcher(interval time.Duration) (*FileWatcher, error) {
	b, err := watcher.New(interval)
	if err != nil {
		return nil, err
	}

	w := FileWatcher{
		w:                 b,
		folderDispatchers: make(map[string]Dispatcher),
		fileDispatchers:   make(map[string]Dispatcher),
	}
	go func() {
		for {
			select {
			case e := <-b.Events:
				w.handleEvents(e)
			case err := <-b.Errors:
				if err != nil {
					log.Print("Error while watching: ", err)
				}
			}
		}
	}()
	return &w, err
}

type DispatcherEvents struct {
	Dispatcher Dispatcher
	Events     []fsnotify.Event
}

func (w *FileWatcher) handleEvents(e []fsnotify.Event) {
	w.lock.RLock()
	defer w.lock.RUnlock()
	m := make(map[string]*DispatcherEvents)
	for _, event := range e {
		k := event.Name
		d, ok := w.fileDispatchers[k]
		if !ok {
			k = filepath.Dir(event.Name)
			d, ok = w.folderDispatchers[k]
		}
		if !ok {
			log.Print("Unregistered listener for ", strconv.Quote(event.Name))
			continue
		}
		de, ok := m[k]
		if ok {
			de.Events = append(de.Events, event)
		} else {
			m[k] = &DispatcherEvents{
				Dispatcher: d,
				Events:     []fsnotify.Event{event},
			}
		}
	}
	for _, events := range m {
		events.Dispatcher(events.Events)
	}
}

func (w *FileWatcher) add(file string, dispatcher Dispatcher, m map[string]Dispatcher) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	_, ok := m[file]
	if ok {
		return errors.New("watcher for this directory already exists")
	}

	m[file] = dispatcher
	if err := w.w.Add(file); err != nil {
		return err
	}

	return nil
}

func (w *FileWatcher) Add(folder string, dispatcher Dispatcher) error {
	return w.add(folder, dispatcher, w.folderDispatchers)
}

func (w *FileWatcher) AddFileWatch(file string, dispatcher Dispatcher) error {
	return w.add(file, dispatcher, w.fileDispatchers)
}

func (w *FileWatcher) Stop() {
	w.w.Close()
}

func FirstNonChmodIn(events []fsnotify.Event) *fsnotify.Event {
	for _, event := range events {
		if event.Op&fsnotify.Chmod == fsnotify.Chmod {
			continue
		}
		return &event
	}
	return nil
}
