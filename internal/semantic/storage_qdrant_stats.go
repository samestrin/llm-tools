package semantic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// NewValueFloat is a helper to create a qdrant.Value from a float32/float64
func NewValueFloat(v float64) *qdrant.Value {
	return &qdrant.Value{
		Kind: &qdrant.Value_DoubleValue{
			DoubleValue: v,
		},
	}
}

// Ensure QdrantStorage implements MemoryStatsTracker
var _ MemoryStatsTracker = (*QdrantStorage)(nil)

// retrievalLogID generates a unique ID for a retrieval log entry
func retrievalLogID() string {
	// Use timestamp + random UUID for uniqueness
	return stringToUUID(fmt.Sprintf("log:%d", time.Now().UnixNano()))
}

// TrackMemoryRetrieval records a single memory retrieval event.
func (s *QdrantStorage) TrackMemoryRetrieval(ctx context.Context, memoryID string, query string, score float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	// 1. Get current stats from memory point
	// We need to read the current payload to update average score and count
	points, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collectionName,
		Ids:            []*qdrant.PointId{qdrant.NewID(memoryPointID(memoryID))},
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return fmt.Errorf("failed to get memory point for tracking: %w", err)
	}

	if len(points) == 0 {
		return fmt.Errorf("memory point not found: %s", memoryID)
	}

	point := points[0]

	// Extract current stats
	var currentCount int64 = 0
	var currentAvg float64 = 0.0

	if v, ok := point.Payload["retrieval_count"]; ok {
		currentCount = v.GetIntegerValue()
	}
	if v, ok := point.Payload["avg_score"]; ok {
		currentAvg = v.GetDoubleValue()
	}

	// Calculate new stats
	newCount := currentCount + 1
	// iterative mean: new_avg = old_avg + (new_value - old_avg) / new_count
	newAvg := currentAvg + (float64(score)-currentAvg)/float64(newCount)
	now := time.Now().Format(time.RFC3339)

	// 2. Update memory point payload
	payloadUpdate := map[string]*qdrant.Value{
		"retrieval_count": qdrant.NewValueInt(newCount),
		"avg_score":       NewValueFloat(newAvg),
		"last_retrieved":  qdrant.NewValueString(now),
	}

	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collectionName,
		Payload:        payloadUpdate,
		PointsSelector: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: []*qdrant.PointId{point.Id},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update memory stats: %w", err)
	}

	// 3. Create retrieval log entry
	// Use a zero vector since we don't usually search logs by vector
	dummyVector := make([]float32, s.embeddingDim)

	logPoint := &qdrant.PointStruct{
		Id:      qdrant.NewID(retrievalLogID()),
		Vectors: qdrant.NewVectors(dummyVector...),
		Payload: map[string]*qdrant.Value{
			"entry_type": qdrant.NewValueString("retrieval_log"),
			"memory_id":  qdrant.NewValueString(memoryID),
			"query":      qdrant.NewValueString(query),
			"score":      NewValueFloat(float64(score)),
			"timestamp":  qdrant.NewValueString(now),
		},
	}

	_, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collectionName,
		Points:         []*qdrant.PointStruct{logPoint},
	})
	if err != nil {
		return fmt.Errorf("failed to create retrieval log: %w", err)
	}

	return nil
}

// TrackMemoryRetrievalBatch records multiple memory retrieval events in a single transaction.
func (s *QdrantStorage) TrackMemoryRetrievalBatch(ctx context.Context, retrievals []MemoryRetrieval, query string) error {
	// For simplicity, implement as loop for now.
	// Qdrant doesn't support a single atomic "update multiple points with different values" easily
	// without multiple requests or a complex batch operation.
	// Given this is a background tracking op, doing it sequentially is acceptable.

	// A better optimization would be to batch the Log Insertions, but the Stats Updates still need read-modify-write per point.

	for _, r := range retrievals {
		if err := s.TrackMemoryRetrieval(ctx, r.MemoryID, query, r.Score); err != nil {
			return err
		}
	}
	return nil
}

// GetMemoryStats returns stats for a specific memory entry.
func (s *QdrantStorage) GetMemoryStats(ctx context.Context, memoryID string) (*RetrievalStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	points, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collectionName,
		Ids:            []*qdrant.PointId{qdrant.NewID(memoryPointID(memoryID))},
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	if len(points) == 0 {
		return nil, ErrMemoryNotFound
	}

	return pointToStats(points[0]), nil
}

// GetAllMemoryStats returns stats for all tracked memories.
func (s *QdrantStorage) GetAllMemoryStats(ctx context.Context) ([]RetrievalStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	// Filter for entry_type="memory"
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			qdrant.NewMatch("entry_type", "memory"),
		},
	}

	// Scroll through all memories
	var allStats []RetrievalStats
	var nextOffset *qdrant.PointId

	for {
		scrollResult, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collectionName,
			Filter:         filter,
			WithPayload:    qdrant.NewWithPayload(true),
			Limit:          qdrant.PtrOf(uint32(100)), // Batch size
			Offset:         nextOffset,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to scroll memories: %w", err)
		}

		if len(scrollResult) == 0 {
			break
		}

		for _, point := range scrollResult {
			allStats = append(allStats, *pointToStats(point))
		}

		// Check if we reached the end
		if len(scrollResult) < 100 {
			break
		}
		// Update offset for next page
		lastPoint := scrollResult[len(scrollResult)-1]
		nextOffset = lastPoint.Id
	}

	return allStats, nil
}

// pointToStats converts a Qdrant point to RetrievalStats
func pointToStats(point *qdrant.RetrievedPoint) *RetrievalStats {
	payload := point.Payload
	stats := &RetrievalStats{}

	if v, ok := payload["memory_id"]; ok {
		stats.MemoryID = v.GetStringValue()
	}
	if v, ok := payload["question"]; ok {
		stats.Question = v.GetStringValue()
	}
	if v, ok := payload["created_at"]; ok {
		stats.CreatedAt = v.GetStringValue()
	}
	if v, ok := payload["status"]; ok {
		stats.Status = v.GetStringValue()
	}
	if v, ok := payload["retrieval_count"]; ok {
		stats.RetrievalCount = int(v.GetIntegerValue())
	}
	if v, ok := payload["last_retrieved"]; ok {
		stats.LastRetrieved = v.GetStringValue()
	}
	if v, ok := payload["avg_score"]; ok {
		stats.AvgScore = float32(v.GetDoubleValue())
	}
	if v, ok := payload["tags"]; ok {
		tagsStr := v.GetStringValue()
		if tagsStr != "" {
			stats.Tags = strings.Split(tagsStr, ",")
		}
	}

	return stats
}

// GetMemoryRetrievalHistory returns recent retrieval log entries for a memory.
func (s *QdrantStorage) GetMemoryRetrievalHistory(ctx context.Context, memoryID string, limit int) ([]RetrievalLogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	// Filter for entry_type="retrieval_log" AND memory_id=memoryID
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			qdrant.NewMatch("entry_type", "retrieval_log"),
			qdrant.NewMatch("memory_id", memoryID),
		},
	}

	// Scroll matching logs
	// Note: Qdrant doesn't guarantee sort order without explicit sort params (which require indexed payload)
	// For now, we fetch up to a reasonable limit and sort in memory.
	// Since `limit` is usually small (e.g., 50), fetching a few hundred should be enough to get recent ones if we assume insertion order roughly correlates,
	// but strictly we should fetch all (or many) and sort.

	// Better approach: Use Scroll with limit. Since IDs are time-based (timestamp+uuid) or random, we can't rely on ID sort.
	// We'll fetch up to 200 logs and sort them.

	scrollLimit := uint32(200)
	if limit > 200 {
		scrollLimit = uint32(limit)
	}

	scrollResult, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collectionName,
		Filter:         filter,
		WithPayload:    qdrant.NewWithPayload(true),
		Limit:          qdrant.PtrOf(scrollLimit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch retrieval history: %w", err)
	}

	var logs []RetrievalLogEntry
	for _, point := range scrollResult {
		log := RetrievalLogEntry{}
		// ID is string in Qdrant, but struct wants int64 (from sqlite autoinc).
		// We can leave ID 0 or hash the string ID. The struct seems to assume SQLite ID.
		// Let's just set MemoryID and other fields.

		payload := point.Payload
		if v, ok := payload["memory_id"]; ok {
			log.MemoryID = v.GetStringValue()
		}
		if v, ok := payload["query"]; ok {
			log.Query = v.GetStringValue()
		}
		if v, ok := payload["score"]; ok {
			log.Score = float32(v.GetDoubleValue())
		}
		if v, ok := payload["timestamp"]; ok {
			log.Timestamp = v.GetStringValue()
		}

		logs = append(logs, log)
	}

	// Sort by timestamp descending
	// Simple bubble sort or similar since len is small
	for i := 0; i < len(logs); i++ {
		for j := i + 1; j < len(logs); j++ {
			if logs[j].Timestamp > logs[i].Timestamp {
				logs[i], logs[j] = logs[j], logs[i]
			}
		}
	}

	if len(logs) > limit {
		logs = logs[:limit]
	}

	return logs, nil
}

// PruneMemoryRetrievalLog removes retrieval log entries older than the specified duration.
func (s *QdrantStorage) PruneMemoryRetrievalLog(ctx context.Context, olderThanDays int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, ErrStorageClosed
	}

	cutoff := time.Now().AddDate(0, 0, -olderThanDays).Format(time.RFC3339)

	// Rewriting strategy for Prune:
	// 1. Scroll all retrieval_logs.
	// 2. Check timestamp in Go.
	// 3. Collect IDs to delete.
	// 4. DeletePoints.

	logFilter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			qdrant.NewMatch("entry_type", "retrieval_log"),
		},
	}

	var idsToDelete []*qdrant.PointId
	var nextOffset *qdrant.PointId

	for {
		scrollResult, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collectionName,
			Filter:         logFilter,
			WithPayload:    qdrant.NewWithPayloadInclude("timestamp"),
			Limit:          qdrant.PtrOf(uint32(200)),
			Offset:         nextOffset,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to scroll for pruning: %w", err)
		}

		if len(scrollResult) == 0 {
			break
		}

		for _, point := range scrollResult {
			if v, ok := point.Payload["timestamp"]; ok {
				ts := v.GetStringValue()
				if ts < cutoff {
					idsToDelete = append(idsToDelete, point.Id)
				}
			}
		}

		if len(scrollResult) < 200 {
			break
		}
		nextOffset = scrollResult[len(scrollResult)-1].Id
	}

	if len(idsToDelete) == 0 {
		return 0, nil
	}

	// Delete in batches of 100
	deletedCount := int64(0)
	batchSize := 100
	for i := 0; i < len(idsToDelete); i += batchSize {
		end := i + batchSize
		if end > len(idsToDelete) {
			end = len(idsToDelete)
		}

		batch := idsToDelete[i:end]
		_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
			CollectionName: s.collectionName,
			Points: &qdrant.PointsSelector{
				PointsSelectorOneOf: &qdrant.PointsSelector_Points{
					Points: &qdrant.PointsIdsList{
						Ids: batch,
					},
				},
			},
		})
		if err != nil {
			return deletedCount, fmt.Errorf("failed to delete batch: %w", err)
		}
		deletedCount += int64(len(batch))
	}

	return deletedCount, nil
}

// UpdateMemoryStatsStatus updates the status of a memory entry.
func (s *QdrantStorage) UpdateMemoryStatsStatus(ctx context.Context, memoryID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	// Check if exists
	points, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collectionName,
		Ids:            []*qdrant.PointId{qdrant.NewID(memoryPointID(memoryID))},
	})
	if err != nil {
		return fmt.Errorf("failed to check memory: %w", err)
	}
	if len(points) == 0 {
		return ErrMemoryNotFound
	}

	// Update payload
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collectionName,
		Payload: map[string]*qdrant.Value{
			"status": qdrant.NewValueString(status),
		},
		PointsSelector: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: []*qdrant.PointId{points[0].Id},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}
