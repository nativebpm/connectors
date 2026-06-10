//go:build !wasm

package wasman

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileSnapshotStore implements SnapshotStore using the local file system.
type FileSnapshotStore struct {
	Dir string
	mu  sync.RWMutex
}

var _ SnapshotStore = (*FileSnapshotStore)(nil)


// Save writes a full memory snapshot to a file.
func (f *FileSnapshotStore) Save(ctx context.Context, id string, snapshot []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s.bin", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s.bin", f.Dir, tenant, id)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, err := compressData(snapshot)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads a full memory snapshot from a file.
func (f *FileSnapshotStore) Load(ctx context.Context, id string) ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s.bin", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s.bin", f.Dir, tenant, id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decompressData(data)
}

func (f *FileSnapshotStore) SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s_deltas.json", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s_deltas.json", f.Dir, tenant, id)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	current := make(map[int][]byte)
	data, err := os.ReadFile(path)
	if err == nil {
		decompressed, err := decompressData(data)
		if err == nil {
			_ = json.Unmarshal(decompressed, &current)
		}
	}
	for k, v := range deltas {
		current[k] = v
	}
	newData, err := json.Marshal(current)
	if err != nil {
		return err
	}
	newData, err = compressData(newData)
	if err != nil {
		return err
	}
	return os.WriteFile(path, newData, 0644)
}

func (f *FileSnapshotStore) LoadDeltas(ctx context.Context, id string) (map[int][]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s_deltas.json", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s_deltas.json", f.Dir, tenant, id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	decompressed, err := decompressData(data)
	if err != nil {
		return nil, err
	}
	var deltas map[int][]byte
	err = json.Unmarshal(decompressed, &deltas)
	return deltas, err
}

func (f *FileSnapshotStore) TruncateDeltas(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s_deltas.json", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s_deltas.json", f.Dir, tenant, id)
	}
	_ = os.Remove(path)
	return nil
}

func (f *FileSnapshotStore) SaveOplog(ctx context.Context, id string, callIndex int, apiName string, request []byte, response []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s_oplog.json", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s_oplog.json", f.Dir, tenant, id)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	var list []OplogEntry
	data, err := os.ReadFile(path)
	if err == nil {
		decompressed, err := decompressData(data)
		if err == nil {
			_ = json.Unmarshal(decompressed, &list)
		}
	}
	list = append(list, OplogEntry{
		CallIndex:       callIndex,
		ApiName:         apiName,
		RequestPayload:  request,
		ResponsePayload: response,
	})
	newData, err := json.Marshal(list)
	if err != nil {
		return err
	}
	newData, err = compressData(newData)
	if err != nil {
		return err
	}
	return os.WriteFile(path, newData, 0644)
}

func (f *FileSnapshotStore) LoadOplog(ctx context.Context, id string) ([]OplogEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s_oplog.json", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s_oplog.json", f.Dir, tenant, id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	decompressed, err := decompressData(data)
	if err != nil {
		return nil, err
	}
	var list []OplogEntry
	err = json.Unmarshal(decompressed, &list)
	return list, err
}

func (f *FileSnapshotStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s_oplog.json", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s_oplog.json", f.Dir, tenant, id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	decompressed, err := decompressData(data)
	if err != nil {
		return err
	}
	var list []OplogEntry
	if err := json.Unmarshal(decompressed, &list); err != nil {
		return err
	}
	var filtered []OplogEntry
	for _, entry := range list {
		if entry.CallIndex > beforeCallIndex {
			filtered = append(filtered, entry)
		}
	}
	newData, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	newData, err = compressData(newData)
	if err != nil {
		return err
	}
	return os.WriteFile(path, newData, 0644)
}

func (f *FileSnapshotStore) SaveMetadata(ctx context.Context, meta *InstanceMeta) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s_meta.json", tenant, meta.InstanceID)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s_meta.json", f.Dir, tenant, meta.InstanceID)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	var existing InstanceMeta
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &existing)
		if meta.Version == 0 {
			return false, nil
		}
		if existing.Version != meta.Version {
			return false, nil
		}
	} else if !os.IsNotExist(err) {
		return false, err
	} else if meta.Version > 0 {
		return false, nil
	}

	meta.Version++
	newData, err := json.Marshal(meta)
	if err != nil {
		return false, err
	}
	err = os.WriteFile(path, newData, 0644)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (f *FileSnapshotStore) LoadMetadata(ctx context.Context, id string) (*InstanceMeta, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s_meta.json", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s_meta.json", f.Dir, tenant, id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var meta InstanceMeta
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

func (f *FileSnapshotStore) Delete(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	tenant := GetTenantID(ctx)
	path := fmt.Sprintf("%s/%s.bin", tenant, id)
	pathDeltas := fmt.Sprintf("%s/%s_deltas.json", tenant, id)
	pathOplog := fmt.Sprintf("%s/%s_oplog.json", tenant, id)
	pathMeta := fmt.Sprintf("%s/%s_meta.json", tenant, id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s/%s.bin", f.Dir, tenant, id)
		pathDeltas = fmt.Sprintf("%s/%s/%s_deltas.json", f.Dir, tenant, id)
		pathOplog = fmt.Sprintf("%s/%s/%s_oplog.json", f.Dir, tenant, id)
		pathMeta = fmt.Sprintf("%s/%s/%s_meta.json", f.Dir, tenant, id)
	}
	_ = os.Remove(path)
	_ = os.Remove(pathDeltas)
	_ = os.Remove(pathOplog)
	_ = os.Remove(pathMeta)
	return nil
}

func (f *FileSnapshotStore) SaveWasm(ctx context.Context, hash string, wasmBytes []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := fmt.Sprintf("wasm_%s.wasm", hash)
	if f.Dir != "" {
		_ = os.MkdirAll(f.Dir, 0755)
		path = fmt.Sprintf("%s/wasm_%s.wasm", f.Dir, hash)
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	data, err := compressData(wasmBytes)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (f *FileSnapshotStore) LoadWasm(ctx context.Context, hash string) ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	path := fmt.Sprintf("wasm_%s.wasm", hash)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/wasm_%s.wasm", f.Dir, hash)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decompressData(data)
}

// UpdateActiveIndex updates the local index file.
func (f *FileSnapshotStore) UpdateActiveIndex(ctx context.Context, id string, info []byte, completed bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := "active_index.json"
	if f.Dir != "" {
		path = fmt.Sprintf("%s/active_index.json", f.Dir)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)

	var index []map[string]interface{}
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &index)
	}

	var newInfo map[string]interface{}
	if err := json.Unmarshal(info, &newInfo); err != nil {
		return fmt.Errorf("failed to unmarshal index info: %w", err)
	}

	updated := false
	nextIndex := make([]map[string]interface{}, 0)
	for _, entry := range index {
		if entry["instance_id"] == id {
			if !completed {
				nextIndex = append(nextIndex, newInfo)
				updated = true
			}
		} else {
			nextIndex = append(nextIndex, entry)
		}
	}
	if !updated && !completed {
		nextIndex = append(nextIndex, newInfo)
	}

	newData, err := json.Marshal(nextIndex)
	if err != nil {
		return err
	}
	return os.WriteFile(path, newData, 0644)
}

// LoadActiveIndex loads the local active index file.
func (f *FileSnapshotStore) LoadActiveIndex(ctx context.Context) ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := "active_index.json"
	if f.Dir != "" {
		path = fmt.Sprintf("%s/active_index.json", f.Dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte("[]"), nil
		}
		return nil, err
	}
	return data, nil
}
