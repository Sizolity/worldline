package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sizolity/worldline/internal/world/model"
)

type FileStore struct {
	root string
}

func NewFileStore(workspace string) *FileStore {
	return &FileStore{root: filepath.Join(workspace, "worlds")}
}

func (s *FileStore) SaveWorld(_ context.Context, world model.World) error {
	if err := world.Validate(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.worldDir(string(world.ID)), "world.json"), world)
}

func (s *FileStore) LoadWorld(_ context.Context, worldID string) (model.World, error) {
	if err := model.ValidateID(worldID); err != nil {
		return model.World{}, err
	}
	var world model.World
	if err := readJSON(filepath.Join(s.worldDir(worldID), "world.json"), &world); err != nil {
		return model.World{}, err
	}
	if string(world.ID) != worldID {
		return model.World{}, fmt.Errorf("world id %q does not match path id %q", world.ID, worldID)
	}
	if err := validateSnapshot(world); err != nil {
		return model.World{}, err
	}
	return world, nil
}

func (s *FileStore) SaveSnapshot(ctx context.Context, world model.World) error {
	if err := validateSnapshot(world); err != nil {
		return err
	}
	if err := s.SaveWorld(ctx, world); err != nil {
		return err
	}
	worldID := string(world.ID)
	if err := os.RemoveAll(filepath.Join(s.worldDir(worldID), "entities")); err != nil {
		return err
	}
	for _, entity := range world.Entities {
		if err := s.SaveEntity(ctx, worldID, entity); err != nil {
			return err
		}
	}
	if err := s.SaveRelations(ctx, worldID, world.Relations); err != nil {
		return err
	}
	if err := s.SaveFacts(ctx, worldID, world.Facts); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(s.worldDir(worldID), "events.jsonl"), world.EventLog); err != nil {
		return err
	}
	if err := s.SaveMemories(ctx, worldID, world.Memory); err != nil {
		return err
	}
	return s.SaveThreads(ctx, worldID, world.Threads)
}

func validateSnapshot(world model.World) error {
	if err := world.Validate(); err != nil {
		return err
	}
	for id, entity := range world.Entities {
		if err := entity.Validate(); err != nil {
			return fmt.Errorf("entities[%s]: %w", id, err)
		}
	}
	for i, relation := range world.Relations {
		if err := model.ValidateID(string(relation.ID)); err != nil {
			return fmt.Errorf("relations[%d].id: %w", i, err)
		}
	}
	for i, fact := range world.Facts {
		if err := model.ValidateID(string(fact.ID)); err != nil {
			return fmt.Errorf("facts[%d].id: %w", i, err)
		}
	}
	for i, event := range world.EventLog {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("events[%d]: %w", i, err)
		}
	}
	for i, event := range world.EventQueue {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("event_queue[%d]: %w", i, err)
		}
	}
	for i, memory := range world.Memory {
		if err := memory.Validate(); err != nil {
			return fmt.Errorf("memories[%d]: %w", i, err)
		}
	}
	for i, thread := range world.Threads {
		if err := thread.Validate(); err != nil {
			return fmt.Errorf("threads[%d]: %w", i, err)
		}
	}
	return nil
}

func (s *FileStore) LoadSnapshot(ctx context.Context, worldID string) (model.World, error) {
	world, err := s.LoadWorld(ctx, worldID)
	if err != nil {
		return model.World{}, err
	}
	entities, err := s.loadEntities(ctx, worldID)
	if err != nil {
		return model.World{}, err
	}
	world.Entities = entities
	if world.Relations, err = loadOptionalJSON[[]model.Relation](filepath.Join(s.worldDir(worldID), "relations.json")); err != nil {
		return model.World{}, err
	}
	if world.Facts, err = loadOptionalJSON[[]model.Fact](filepath.Join(s.worldDir(worldID), "facts.json")); err != nil {
		return model.World{}, err
	}
	if world.EventLog, err = s.ListEvents(ctx, worldID); err != nil {
		return model.World{}, err
	}
	if world.Memory, err = loadOptionalJSON[[]model.MemoryRecord](filepath.Join(s.worldDir(worldID), "memories.json")); err != nil {
		return model.World{}, err
	}
	if world.Threads, err = loadOptionalJSON[[]model.WorldThread](filepath.Join(s.worldDir(worldID), "threads.json")); err != nil {
		return model.World{}, err
	}
	if err := validateSnapshot(world); err != nil {
		return model.World{}, err
	}
	return world, nil
}

func (s *FileStore) SaveEntity(_ context.Context, worldID string, entity model.Entity) error {
	if err := model.ValidateID(worldID); err != nil {
		return err
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.worldDir(worldID), "entities", string(entity.ID)+".json"), entity)
}

func (s *FileStore) LoadEntity(_ context.Context, worldID, entityID string) (model.Entity, error) {
	if err := model.ValidateID(worldID); err != nil {
		return model.Entity{}, err
	}
	if err := model.ValidateID(entityID); err != nil {
		return model.Entity{}, err
	}
	var entity model.Entity
	if err := readJSON(filepath.Join(s.worldDir(worldID), "entities", entityID+".json"), &entity); err != nil {
		return model.Entity{}, err
	}
	if string(entity.ID) != entityID {
		return model.Entity{}, fmt.Errorf("entity id %q does not match path id %q", entity.ID, entityID)
	}
	return entity, entity.Validate()
}

func (s *FileStore) loadEntities(_ context.Context, worldID string) (map[model.EntityID]model.Entity, error) {
	if err := model.ValidateID(worldID); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.worldDir(worldID), "entities")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entities := map[model.EntityID]model.Entity{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		entityID := entry.Name()[:len(entry.Name())-len(".json")]
		entity, err := s.LoadEntity(context.Background(), worldID, entityID)
		if err != nil {
			return nil, err
		}
		entities[entity.ID] = entity
	}
	return entities, nil
}

func (s *FileStore) SaveRelations(_ context.Context, worldID string, relations []model.Relation) error {
	if err := model.ValidateID(worldID); err != nil {
		return err
	}
	for i, relation := range relations {
		if err := model.ValidateID(string(relation.ID)); err != nil {
			return fmt.Errorf("relations[%d].id: %w", i, err)
		}
	}
	return writeJSON(filepath.Join(s.worldDir(worldID), "relations.json"), relations)
}

func (s *FileStore) LoadRelations(_ context.Context, worldID string) ([]model.Relation, error) {
	if err := model.ValidateID(worldID); err != nil {
		return nil, err
	}
	var relations []model.Relation
	if err := readJSON(filepath.Join(s.worldDir(worldID), "relations.json"), &relations); err != nil {
		return nil, err
	}
	return relations, nil
}

func (s *FileStore) SaveFacts(_ context.Context, worldID string, facts []model.Fact) error {
	if err := model.ValidateID(worldID); err != nil {
		return err
	}
	for i, fact := range facts {
		if err := model.ValidateID(string(fact.ID)); err != nil {
			return fmt.Errorf("facts[%d].id: %w", i, err)
		}
	}
	return writeJSON(filepath.Join(s.worldDir(worldID), "facts.json"), facts)
}

func (s *FileStore) LoadFacts(_ context.Context, worldID string) ([]model.Fact, error) {
	if err := model.ValidateID(worldID); err != nil {
		return nil, err
	}
	var facts []model.Fact
	if err := readJSON(filepath.Join(s.worldDir(worldID), "facts.json"), &facts); err != nil {
		return nil, err
	}
	return facts, nil
}

func (s *FileStore) AppendEvent(_ context.Context, worldID string, event model.WorldEvent) error {
	if err := model.ValidateID(worldID); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	return appendJSONL(filepath.Join(s.worldDir(worldID), "events.jsonl"), event)
}

func (s *FileStore) ListEvents(_ context.Context, worldID string) ([]model.WorldEvent, error) {
	if err := model.ValidateID(worldID); err != nil {
		return nil, err
	}
	events, err := readJSONL[model.WorldEvent](filepath.Join(s.worldDir(worldID), "events.jsonl"))
	if err != nil {
		return nil, err
	}
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("events[%d]: %w", i, err)
		}
	}
	return events, nil
}

func (s *FileStore) SaveMemories(_ context.Context, worldID string, memories []model.MemoryRecord) error {
	if err := model.ValidateID(worldID); err != nil {
		return err
	}
	for i, memory := range memories {
		if err := memory.Validate(); err != nil {
			return fmt.Errorf("memories[%d]: %w", i, err)
		}
	}
	return writeJSON(filepath.Join(s.worldDir(worldID), "memories.json"), memories)
}

func (s *FileStore) LoadMemories(_ context.Context, worldID string) ([]model.MemoryRecord, error) {
	if err := model.ValidateID(worldID); err != nil {
		return nil, err
	}
	var memories []model.MemoryRecord
	if err := readJSON(filepath.Join(s.worldDir(worldID), "memories.json"), &memories); err != nil {
		return nil, err
	}
	for i, memory := range memories {
		if err := memory.Validate(); err != nil {
			return nil, fmt.Errorf("memories[%d]: %w", i, err)
		}
	}
	return memories, nil
}

func (s *FileStore) SaveThreads(_ context.Context, worldID string, threads []model.WorldThread) error {
	if err := model.ValidateID(worldID); err != nil {
		return err
	}
	for i, thread := range threads {
		if err := thread.Validate(); err != nil {
			return fmt.Errorf("threads[%d]: %w", i, err)
		}
	}
	return writeJSON(filepath.Join(s.worldDir(worldID), "threads.json"), threads)
}

func (s *FileStore) LoadThreads(_ context.Context, worldID string) ([]model.WorldThread, error) {
	if err := model.ValidateID(worldID); err != nil {
		return nil, err
	}
	var threads []model.WorldThread
	if err := readJSON(filepath.Join(s.worldDir(worldID), "threads.json"), &threads); err != nil {
		return nil, err
	}
	for i, thread := range threads {
		if err := thread.Validate(); err != nil {
			return nil, fmt.Errorf("threads[%d]: %w", i, err)
		}
	}
	return threads, nil
}

func (s *FileStore) SaveCheckpoint(ctx context.Context, worldID string) (int64, error) {
	world, err := s.LoadSnapshot(ctx, worldID)
	if err != nil {
		return 0, fmt.Errorf("load snapshot for checkpoint: %w", err)
	}
	seq := world.Clock.Sequence
	cpDir := s.checkpointDir(worldID, seq)
	if err := os.MkdirAll(cpDir, 0o755); err != nil {
		return 0, err
	}
	cpStore := &FileStore{root: filepath.Join(cpDir)}
	if err := cpStore.SaveSnapshot(ctx, world); err != nil {
		return 0, fmt.Errorf("save checkpoint: %w", err)
	}
	return seq, nil
}

func (s *FileStore) LoadCheckpoint(ctx context.Context, worldID string, sequence int64) (model.World, error) {
	cpDir := s.checkpointDir(worldID, sequence)
	cpStore := &FileStore{root: filepath.Join(cpDir)}
	world, err := cpStore.LoadSnapshot(ctx, worldID)
	if err != nil {
		return model.World{}, fmt.Errorf("load checkpoint %d: %w", sequence, err)
	}
	return world, nil
}

func (s *FileStore) ListCheckpoints(_ context.Context, worldID string) ([]int64, error) {
	if err := model.ValidateID(worldID); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.worldDir(worldID), "checkpoints")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []int64{}, nil
		}
		return nil, err
	}
	var seqs []int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var seq int64
		if _, err := fmt.Sscanf(entry.Name(), "%d", &seq); err == nil {
			seqs = append(seqs, seq)
		}
	}
	return seqs, nil
}

func (s *FileStore) checkpointDir(worldID string, sequence int64) string {
	return filepath.Join(s.worldDir(worldID), "checkpoints", fmt.Sprintf("%d", sequence))
}

// ForkWorld creates a new world by copying the source world's state.
// If atSequence > 0, it forks from the checkpoint at that sequence.
// If atSequence <= 0, it forks from the current snapshot.
// The new world gets a ForkInfo recording its parent lineage.
func (s *FileStore) ForkWorld(ctx context.Context, sourceWorldID, newWorldID string, atSequence int64) (model.World, error) {
	if err := model.ValidateID(sourceWorldID); err != nil {
		return model.World{}, fmt.Errorf("source world id: %w", err)
	}
	if err := model.ValidateID(newWorldID); err != nil {
		return model.World{}, fmt.Errorf("new world id: %w", err)
	}
	if sourceWorldID == newWorldID {
		return model.World{}, fmt.Errorf("source and new world IDs must differ")
	}

	var world model.World
	var err error
	if atSequence > 0 {
		world, err = s.LoadCheckpoint(ctx, sourceWorldID, atSequence)
	} else {
		world, err = s.LoadSnapshot(ctx, sourceWorldID)
		atSequence = world.Clock.Sequence
	}
	if err != nil {
		return model.World{}, fmt.Errorf("load source for fork: %w", err)
	}

	world.ID = model.WorldID(newWorldID)
	world.Metadata.Fork = &model.ForkInfo{
		ParentWorldID: model.WorldID(sourceWorldID),
		ForkSequence:  atSequence,
	}
	world.EventQueue = nil

	if err := s.SaveSnapshot(ctx, world); err != nil {
		return model.World{}, fmt.Errorf("save forked world: %w", err)
	}
	return world, nil
}

// ListWorlds returns the IDs of all worlds in the store by scanning the
// worlds directory. Only directories containing a valid world.json are included.
func (s *FileStore) ListWorlds(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		worldFile := filepath.Join(s.root, entry.Name(), "world.json")
		if _, err := os.Stat(worldFile); err == nil {
			ids = append(ids, entry.Name())
		}
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

func (s *FileStore) worldDir(worldID string) string {
	return filepath.Join(s.root, worldID)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func appendJSONL(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func writeJSONL(path string, events []model.WorldEvent) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := event.Validate(); err != nil {
			_ = f.Close()
			return err
		}
		data, err := json.Marshal(event)
		if err != nil {
			_ = f.Close()
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			_ = f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadOptionalJSON[T any](path string) (T, error) {
	var out T
	if err := readJSON(path, &out); err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	return out, nil
}

func readJSONL[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []T{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []T
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var item T
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
