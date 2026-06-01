package store

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/sizolity/worldline/internal/world/model"
)

// ExportWorld writes a world snapshot as a portable tar.gz archive to w.
// The archive layout mirrors the file store structure:
//
//	world.json
//	entities/<id>.json
//	relations.json
//	facts.json
//	events.json
//	memories.json
//	threads.json
//
// Events use a single JSON array (not JSONL) for portability.
func ExportWorld(w model.World, out io.Writer) error {
	if err := w.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	if err := addJSON(tw, "world.json", w); err != nil {
		return err
	}
	for _, entity := range w.Entities {
		name := fmt.Sprintf("entities/%s.json", entity.ID)
		if err := addJSON(tw, name, entity); err != nil {
			return err
		}
	}
	if err := addJSON(tw, "relations.json", nonNil(w.Relations)); err != nil {
		return err
	}
	if err := addJSON(tw, "facts.json", nonNil(w.Facts)); err != nil {
		return err
	}
	if err := addJSON(tw, "events.json", nonNil(w.EventLog)); err != nil {
		return err
	}
	if err := addJSON(tw, "memories.json", nonNil(w.Memory)); err != nil {
		return err
	}
	return addJSON(tw, "threads.json", nonNil(w.Threads))
}

// ImportWorld reads a tar.gz archive produced by ExportWorld and returns
// the reconstructed world. If newID is non-empty, the world ID is replaced.
func ImportWorld(r io.Reader, newID string) (model.World, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return model.World{}, fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var w model.World
	entities := map[model.EntityID]model.Entity{}
	var relations []model.Relation
	var facts []model.Fact
	var events []model.WorldEvent
	var memories []model.MemoryRecord
	var threads []model.WorldThread

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return model.World{}, fmt.Errorf("tar read: %w", err)
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return model.World{}, fmt.Errorf("read %s: %w", hdr.Name, err)
		}

		switch {
		case hdr.Name == "world.json":
			if err := json.Unmarshal(data, &w); err != nil {
				return model.World{}, fmt.Errorf("parse world.json: %w", err)
			}
		case len(hdr.Name) > len("entities/") && hdr.Name[:len("entities/")] == "entities/":
			var e model.Entity
			if err := json.Unmarshal(data, &e); err != nil {
				return model.World{}, fmt.Errorf("parse %s: %w", hdr.Name, err)
			}
			entities[e.ID] = e
		case hdr.Name == "relations.json":
			if err := json.Unmarshal(data, &relations); err != nil {
				return model.World{}, fmt.Errorf("parse relations.json: %w", err)
			}
		case hdr.Name == "facts.json":
			if err := json.Unmarshal(data, &facts); err != nil {
				return model.World{}, fmt.Errorf("parse facts.json: %w", err)
			}
		case hdr.Name == "events.json":
			if err := json.Unmarshal(data, &events); err != nil {
				return model.World{}, fmt.Errorf("parse events.json: %w", err)
			}
		case hdr.Name == "memories.json":
			if err := json.Unmarshal(data, &memories); err != nil {
				return model.World{}, fmt.Errorf("parse memories.json: %w", err)
			}
		case hdr.Name == "threads.json":
			if err := json.Unmarshal(data, &threads); err != nil {
				return model.World{}, fmt.Errorf("parse threads.json: %w", err)
			}
		}
	}

	if w.ID == "" {
		return model.World{}, fmt.Errorf("archive missing world.json")
	}

	w.Entities = entities
	w.Relations = relations
	w.Facts = facts
	w.EventLog = events
	w.Memory = memories
	w.Threads = threads

	if newID != "" {
		w.ID = model.WorldID(newID)
	}

	if err := w.Validate(); err != nil {
		return model.World{}, fmt.Errorf("imported world invalid: %w", err)
	}
	return w, nil
}

// ExportToFileStore is a convenience that loads, exports, and writes to a store.
func ExportToFileStore(ctx context.Context, fs *FileStore, worldID string, out io.Writer) error {
	w, err := fs.LoadSnapshot(ctx, worldID)
	if err != nil {
		return err
	}
	return ExportWorld(w, out)
}

// ImportToFileStore reads an archive and saves it as a snapshot in the store.
func ImportToFileStore(ctx context.Context, fs *FileStore, r io.Reader, newID string) (model.World, error) {
	w, err := ImportWorld(r, newID)
	if err != nil {
		return model.World{}, err
	}
	if err := fs.SaveSnapshot(ctx, w); err != nil {
		return model.World{}, fmt.Errorf("save imported world: %w", err)
	}
	return w, nil
}

func addJSON(tw *tar.Writer, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write header %s: %w", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
