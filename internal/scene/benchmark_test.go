// Package scene provides benchmarks for critical scene operations.
package scene

import (
	"testing"
	"time"
)

// BenchmarkSceneRepository_Insert measures the performance of scene creation
func BenchmarkSceneRepository_Insert(b *testing.B) {
	repo := NewInMemorySceneRepository()

	b.Run("single_scene_insertion", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			now := time.Now()
			scene := &Scene{
				ID:            generateTestSceneID(i),
				Name:          "Test Scene",
				Description:   "A test scene for benchmarking",
				OwnerDID:      "did:plc:test123",
				CreatedAt:     &now,
				UpdatedAt:     &now,
				AllowPrecise:  false,
				CoarseGeohash: "dr5r",
				PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
			}
			_ = repo.Insert(scene)
		}
	})

	b.Run("concurrent_scene_insertion", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			var counter int64
			for pb.Next() {
				now := time.Now()
				// Use unique ID based on timestamp and a sequential counter to avoid collisions
				uniqueID := generateTestSceneID(int(time.Now().UnixNano()%1000000) + int(counter))
				scene := &Scene{
					ID:            uniqueID,
					Name:          "Test Scene",
					Description:   "A test scene for benchmarking",
					OwnerDID:      "did:plc:test123",
					CreatedAt:     &now,
					UpdatedAt:     &now,
					AllowPrecise:  false,
					CoarseGeohash: "dr5r",
					PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
				}
				_ = repo.Insert(scene)
				counter++
			}
		})
	})
}

// BenchmarkSceneRepository_GetByID measures the performance of scene retrieval
func BenchmarkSceneRepository_GetByID(b *testing.B) {
	repo := NewInMemorySceneRepository()

	// Pre-populate with test data
	for i := 0; i < 1000; i++ {
		now := time.Now()
		scene := &Scene{
			ID:            generateTestSceneID(i),
			Name:          "Test Scene",
			Description:   "A test scene for benchmarking",
			OwnerDID:      "did:plc:test123",
			CreatedAt:     &now,
			UpdatedAt:     &now,
			AllowPrecise:  false,
			CoarseGeohash: "dr5r",
			PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
		}
		_ = repo.Insert(scene)
	}

	b.Run("single_lookup", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			id := generateTestSceneID(i % 1000)
			_, _ = repo.GetByID(id)
		}
	})

	b.Run("concurrent_lookup", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			var counter int64
			for pb.Next() {
				// Use modulo to cycle through existing test scenes
				id := generateTestSceneID(int(counter % 1000))
				_, _ = repo.GetByID(id)
				counter++
			}
		})
	})
}

// BenchmarkSceneRepository_ListByOwner measures the performance of owner-based queries
func BenchmarkSceneRepository_ListByOwner(b *testing.B) {
	repo := NewInMemorySceneRepository()

	// Pre-populate with test data for multiple owners
	for i := 0; i < 1000; i++ {
		now := time.Now()
		ownerDID := generateTestOwnerDID(i % 100) // 100 different owners, 10 scenes each
		scene := &Scene{
			ID:            generateTestSceneID(i),
			Name:          "Test Scene",
			Description:   "A test scene for benchmarking",
			OwnerDID:      ownerDID,
			CreatedAt:     &now,
			UpdatedAt:     &now,
			AllowPrecise:  false,
			CoarseGeohash: "dr5r",
			PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
		}
		_ = repo.Insert(scene)
	}

	b.Run("list_by_owner", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ownerDID := generateTestOwnerDID(i % 100)
			_, _ = repo.ListByOwner(ownerDID)
		}
	})
}

// BenchmarkScene_EnforceLocationConsent measures the performance of privacy enforcement
func BenchmarkScene_EnforceLocationConsent(b *testing.B) {
	b.Run("with_precise_location", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			scene := &Scene{
				AllowPrecise: true,
				PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060},
			}
			scene.EnforceLocationConsent()
		}
	})

	b.Run("without_precise_location", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			scene := &Scene{
				AllowPrecise: false,
				PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060},
			}
			scene.EnforceLocationConsent()
		}
	})
}

// Helper functions for generating test data

func generateTestSceneID(i int) string {
	return generateID("scene", i)
}

func generateTestOwnerDID(i int) string {
	return generateID("did:plc:owner", i)
}

func generateID(prefix string, i int) string {
	return prefix + string(rune(i%26+'a')) + string(rune((i/26)%26+'a')) + string(rune((i/676)%26+'a'))
}
