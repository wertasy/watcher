package watcher

import (
	"strings"
	"syscall"
	"unsafe"
)

const (
	IN_ACCESS    = syscall.IN_ACCESS
	IN_ALL_EVENT = syscall.IN_ALL_EVENTS
	IN_ATTRIB    = syscall.IN_ATTRIB
	IN_CLASSA_HO = syscall.IN_CLASSA_HOST
	IN_CLASSA_MA = syscall.IN_CLASSA_MAX
	IN_CLASSA_NE = syscall.IN_CLASSA_NET
	IN_CLASSA_NS = syscall.IN_CLASSA_NSHIFT
	IN_CLASSB_HO = syscall.IN_CLASSB_HOST
	IN_CLASSB_MA = syscall.IN_CLASSB_MAX
	IN_CLASSB_NE = syscall.IN_CLASSB_NET
	IN_CLASSB_NS = syscall.IN_CLASSB_NSHIFT
	IN_CLASSC_HO = syscall.IN_CLASSC_HOST
	IN_CLASSC_NE = syscall.IN_CLASSC_NET
	IN_CLASSC_NS = syscall.IN_CLASSC_NSHIFT
	IN_CLOEXEC   = syscall.IN_CLOEXEC
	IN_CLOSE     = syscall.IN_CLOSE
	IN_CLOSE_NOW = syscall.IN_CLOSE_NOWRITE
	IN_CLOSE_WRI = syscall.IN_CLOSE_WRITE
	IN_CREATE    = syscall.IN_CREATE
	IN_DELETE    = syscall.IN_DELETE
	IN_DELETE_SE = syscall.IN_DELETE_SELF
	IN_DONT_FOLL = syscall.IN_DONT_FOLLOW
	IN_EXCL_UNLI = syscall.IN_EXCL_UNLINK
	IN_IGNORED   = syscall.IN_IGNORED
	IN_ISDIR     = syscall.IN_ISDIR
	IN_LOOPBACKN = syscall.IN_LOOPBACKNET
	IN_MASK_ADD  = syscall.IN_MASK_ADD
	IN_MODIFY    = syscall.IN_MODIFY
	IN_MOVE      = syscall.IN_MOVE
	IN_MOVED_FRO = syscall.IN_MOVED_FROM
	IN_MOVED_TO  = syscall.IN_MOVED_TO
	IN_MOVE_SELF = syscall.IN_MOVE_SELF
	IN_NONBLOCK  = syscall.IN_NONBLOCK
	IN_ONESHOT   = syscall.IN_ONESHOT
	IN_ONLYDIR   = syscall.IN_ONLYDIR
	IN_OPEN      = syscall.IN_OPEN
)

type Event struct {
	Wd     int32
	Mask   uint32
	Cookie uint32
	Name   string
	Path   string
}

func (e *Event) IsCreate() bool {
	return e.Mask&IN_CREATE == IN_CREATE
}

type InodeWatcher struct {
	fd   int
	wd   map[string]int
	path map[int]string
	ch   chan *Event
}

func NewInodeWatcher() (*InodeWatcher, error) {
	fd, err := syscall.InotifyInit()
	if err != nil {
		return nil, err
	}

	w := &InodeWatcher{
		fd:   fd,
		wd:   make(map[string]int),
		path: make(map[int]string),
		ch:   make(chan *Event),
	}

	go w.readLoop()

	return w, nil
}

func (w *InodeWatcher) Add(path string, mask uint32) error {
	path = strings.TrimSuffix(path, "/")
	if _, ok := w.wd[path]; ok {
		return nil
	}

	wd, err := syscall.InotifyAddWatch(w.fd, path, mask)
	if err != nil {
		return err
	}

	w.wd[path] = wd
	w.path[wd] = path

	return nil
}

func (w *InodeWatcher) Remove(path string) error {
	path = strings.TrimSuffix(path, "/")
	wd, ok := w.wd[path]
	if !ok {
		return nil
	}

	_, err := syscall.InotifyRmWatch(w.fd, uint32(wd))
	if err != nil {
		return err
	}

	delete(w.path, wd)
	delete(w.wd, path)

	return nil
}

func (w *InodeWatcher) Close() {
	for path := range w.wd {
		w.Remove(path)
	}
	syscall.Close(w.fd)
}

func (w *InodeWatcher) readLoop() {
	buf := make([]byte, syscall.SizeofInotifyEvent*4096)

	for {
		n, err := syscall.Read(w.fd, buf)
		if n == 0 || err != nil {
			continue
		}

		for offset := 0; offset <= n-syscall.SizeofInotifyEvent; {
			row := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			event := &Event{
				Wd:     row.Wd,
				Mask:   row.Mask,
				Cookie: row.Cookie,
				Path:   w.path[int(row.Wd)],
			}

			offset += syscall.SizeofInotifyEvent
			if row.Len > 0 {
				name := (*[syscall.PathMax]byte)(unsafe.Pointer(&buf[offset]))
				event.Name = toString(name[:row.Len])
				offset += int(row.Len)
			}

			w.ch <- event
		}
	}
}

func toString(bytes []byte) string {
	for i, b := range bytes {
		if b == 0 {
			return string(bytes[:i])
		}
	}
	return string(bytes)
}

func (w *InodeWatcher) Wait() <-chan *Event {
	return w.ch
}
