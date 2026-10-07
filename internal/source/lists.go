package source

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nbyoung/tablotui/internal/view/list"
)

// ListRequest names one list-shaped view and the parameters that focus it.
// It names no ref and no person: the source holds them. It names the viewer
// and the role, as Request does, so that a source resolves the level a view
// opens at.
type ListRequest struct {
	View   string // "task", "queue", "blockage" or "history"
	Task   string // the task in focus; "" on the queue
	Level  string // "detail" or "provenance"; "" is the level the viewer's role opens the view at
	Viewer string // the email of the person at the keyboard; "" is nobody
	Role   string // the role the viewer reads as; "" is the viewer's own
}

// Lists loads the list-shaped views. A call blocks; a pane runs it in a command.
type Lists interface {
	Task(ctx context.Context, r ListRequest) (list.Task, error)
	Queue(ctx context.Context, r ListRequest) (list.Queue, error)
	Blockage(ctx context.Context, r ListRequest) (list.Blockage, error)
	History(ctx context.Context, r ListRequest) (list.History, error)
}

// ErrNoPerson is the error Queue returns when the viewer has no email.
var ErrNoPerson = errors.New("the work queue needs a person, and the viewer states none")

// ListName returns the file name that serves r: the view, then "-<task>"
// when Task is set, then "-provenance" when Level is "provenance", then ".json".
func (f File) ListName(r ListRequest) string {
	name := r.View
	if r.Task != "" {
		name += "-" + r.Task
	}
	if r.Level == "provenance" {
		name += "-provenance"
	}
	return name + ".json"
}

// decodeList reads the file that serves r into v, which View names.
func (f File) decodeList(ctx context.Context, r ListRequest, view string, v any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.View = view
	name := f.ListName(r)
	file, err := os.Open(filepath.Join(f.Dir, name))
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := list.Decode(file, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// Task implements Lists.
func (f File) Task(ctx context.Context, r ListRequest) (list.Task, error) {
	var v list.Task
	err := f.decodeList(ctx, r, "task", &v)
	return v, err
}

// Queue implements Lists.
func (f File) Queue(ctx context.Context, r ListRequest) (list.Queue, error) {
	var v list.Queue
	err := f.decodeList(ctx, r, "queue", &v)
	return v, err
}

// Blockage implements Lists.
func (f File) Blockage(ctx context.Context, r ListRequest) (list.Blockage, error) {
	var v list.Blockage
	err := f.decodeList(ctx, r, "blockage", &v)
	return v, err
}

// History implements Lists.
func (f File) History(ctx context.Context, r ListRequest) (list.History, error) {
	var v list.History
	err := f.decodeList(ctx, r, "history", &v)
	return v, err
}
