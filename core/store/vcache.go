package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

type embedPayload struct {
	V string `json:"v"`
	D int    `json:"d"`
	M string `json:"m"`
}

func embedKey(model string, dim int, text string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s", model, dim, text)))
	return internalPrefix + "embed:" + hex.EncodeToString(h[:])
}

func (d *db) getVector(model string, dim int, text string) ([]float32, bool) {
	key := embedKey(model, dim, text)
	e, ok := d.data[key]
	if !ok {
		return nil, false
	}

	var payload embedPayload
	if err := json.Unmarshal([]byte(e.Value()), &payload); err != nil {
		return nil, false
	}
	if payload.D != dim {
		return nil, false
	}
	vec, err := decodeVector(payload.V)
	if err != nil {
		return nil, false
	}
	return vec, true
}

func (d *db) putVector(model string, dim int, text string, vec []float32) error {
	if err := d.init(); err != nil {
		return err
	}

	key := embedKey(model, dim, text)
	payload, err := json.Marshal(embedPayload{
		V: encodeVector(vec),
		D: dim,
		M: model,
	})
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	value := string(payload)

	now := time.Now().Unix()
	if existing, ok := d.data[key]; ok {
		existing.setValue(value)
		existing.Type = TypeJSON
		existing.UpdatedAt = &now
		existing.parseAndCache()
	} else {
		entry := &Entry{
			Key:       key,
			Type:      TypeJSON,
			CreatedAt: now,
		}
		entry.setValue(value)
		entry.parseAndCache()
		d.data[key] = entry
	}

	raw, err := d.data[key].JSON()
	if err != nil {
		return fmt.Errorf("d.data[key].JSON: %w", err)
	}
	if err := utils.WriteFile(d.filePath(key), raw, 0644); err != nil {
		return err
	}

	return d.addToAOF("SET", key, value, nil)
}
