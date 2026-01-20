package post

import (
	"testing"
	"time"
)

// TestListByScene_BasicFunctionality tests basic scene feed retrieval.
func TestListByScene_BasicFunctionality(t *testing.T) {
	repo := NewInMemoryPostRepository()
	sceneID := "scene123"

	// Create 5 posts for this scene
	now := time.Now()
	for i := 0; i < 5; i++ {
		post := &Post{
			SceneID:   &sceneID,
			AuthorDID: "did:example:user1",
			Text:      "Post " + string(rune('A'+i)),
			CreatedAt: now.Add(-time.Duration(i) * time.Hour), // Each post 1 hour older
		}
		if err := repo.Create(post); err != nil {
			t.Fatalf("failed to create post: %v", err)
		}
	}

	// Retrieve all posts
	posts, nextCursor, err := repo.ListByScene(sceneID, 10, nil)
	if err != nil {
		t.Fatalf("ListByScene failed: %v", err)
	}

	if len(posts) != 5 {
		t.Errorf("expected 5 posts, got %d", len(posts))
	}

	if nextCursor != nil {
		t.Error("expected nil cursor (no more posts)")
	}

	// Verify ordering: newest first
	for i := 1; i < len(posts); i++ {
		if posts[i-1].CreatedAt.Before(posts[i].CreatedAt) {
			t.Errorf("posts not ordered correctly: post %d is older than post %d", i-1, i)
		}
	}
}

// TestListByScene_Pagination tests cursor-based pagination.
func TestListByScene_Pagination(t *testing.T) {
	repo := NewInMemoryPostRepository()
	sceneID := "scene123"

	// Create 25 posts
	now := time.Now()
	for i := 0; i < 25; i++ {
		post := &Post{
			SceneID:   &sceneID,
			AuthorDID: "did:example:user1",
			Text:      "Post " + string(rune('A'+i)),
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
		if err := repo.Create(post); err != nil {
			t.Fatalf("failed to create post: %v", err)
		}
	}

	// Get first page
	page1, cursor1, err := repo.ListByScene(sceneID, 10, nil)
	if err != nil {
		t.Fatalf("ListByScene page 1 failed: %v", err)
	}

	if len(page1) != 10 {
		t.Errorf("expected 10 posts on page 1, got %d", len(page1))
	}

	if cursor1 == nil {
		t.Fatal("expected cursor to be set")
	}

	// Get second page
	page2, cursor2, err := repo.ListByScene(sceneID, 10, cursor1)
	if err != nil {
		t.Fatalf("ListByScene page 2 failed: %v", err)
	}

	if len(page2) != 10 {
		t.Errorf("expected 10 posts on page 2, got %d", len(page2))
	}

	if cursor2 == nil {
		t.Fatal("expected cursor to be set for page 2")
	}

	// Get third page
	page3, cursor3, err := repo.ListByScene(sceneID, 10, cursor2)
	if err != nil {
		t.Fatalf("ListByScene page 3 failed: %v", err)
	}

	if len(page3) != 5 {
		t.Errorf("expected 5 posts on page 3, got %d", len(page3))
	}

	if cursor3 != nil {
		t.Error("expected nil cursor on last page")
	}

	// Verify no duplicates across pages
	seen := make(map[string]bool)
	for _, p := range append(append(page1, page2...), page3...) {
		if seen[p.ID] {
			t.Errorf("duplicate post ID found: %s", p.ID)
		}
		seen[p.ID] = true
	}
}

// TestListByScene_HiddenExcluded tests that hidden posts are excluded.
func TestListByScene_HiddenExcluded(t *testing.T) {
	repo := NewInMemoryPostRepository()
	sceneID := "scene123"

	// Create 3 normal posts
	now := time.Now()
	for i := 0; i < 3; i++ {
		post := &Post{
			SceneID:   &sceneID,
			AuthorDID: "did:example:user1",
			Text:      "Normal post",
			Labels:    []string{},
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
		if err := repo.Create(post); err != nil {
			t.Fatalf("failed to create post: %v", err)
		}
	}

	// Create 2 hidden posts
	for i := 0; i < 2; i++ {
		post := &Post{
			SceneID:   &sceneID,
			AuthorDID: "did:example:user1",
			Text:      "Hidden post",
			Labels:    []string{LabelHidden},
			CreatedAt: now.Add(time.Duration(i+1) * time.Hour), // Newer than normal posts
		}
		if err := repo.Create(post); err != nil {
			t.Fatalf("failed to create hidden post: %v", err)
		}
	}

	// Retrieve posts
	posts, _, err := repo.ListByScene(sceneID, 10, nil)
	if err != nil {
		t.Fatalf("ListByScene failed: %v", err)
	}

	// Should only return 3 normal posts
	if len(posts) != 3 {
		t.Errorf("expected 3 posts (hidden excluded), got %d", len(posts))
	}

	// Verify none have hidden label
	for _, p := range posts {
		if p.HasLabel(LabelHidden) {
			t.Errorf("found hidden post in results: %s", p.ID)
		}
	}
}

// TestListByScene_DeletedExcluded tests that soft-deleted posts are excluded.
func TestListByScene_DeletedExcluded(t *testing.T) {
	repo := NewInMemoryPostRepository()
	sceneID := "scene123"

	// Create 5 posts
	var postIDs []string
	now := time.Now()
	for i := 0; i < 5; i++ {
		post := &Post{
			SceneID:   &sceneID,
			AuthorDID: "did:example:user1",
			Text:      "Post " + string(rune('A'+i)),
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
		if err := repo.Create(post); err != nil {
			t.Fatalf("failed to create post: %v", err)
		}
		postIDs = append(postIDs, post.ID)
	}

	// Delete 2 posts
	if err := repo.Delete(postIDs[1]); err != nil {
		t.Fatalf("failed to delete post: %v", err)
	}
	if err := repo.Delete(postIDs[3]); err != nil {
		t.Fatalf("failed to delete post: %v", err)
	}

	// Retrieve posts
	posts, _, err := repo.ListByScene(sceneID, 10, nil)
	if err != nil {
		t.Fatalf("ListByScene failed: %v", err)
	}

	// Should only return 3 posts
	if len(posts) != 3 {
		t.Errorf("expected 3 posts (deleted excluded), got %d", len(posts))
	}

	// Verify deleted posts are not in results
	for _, p := range posts {
		if p.ID == postIDs[1] || p.ID == postIDs[3] {
			t.Errorf("found deleted post in results: %s", p.ID)
		}
	}
}

// TestListByEvent_BasicFunctionality tests basic event feed retrieval.
func TestListByEvent_BasicFunctionality(t *testing.T) {
	repo := NewInMemoryPostRepository()
	eventID := "event123"

	// Create 5 posts for this event
	now := time.Now()
	for i := 0; i < 5; i++ {
		post := &Post{
			SceneID:   strPtr("scene123"),
			EventID:   &eventID,
			AuthorDID: "did:example:user1",
			Text:      "Post " + string(rune('A'+i)),
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
		if err := repo.Create(post); err != nil {
			t.Fatalf("failed to create post: %v", err)
		}
	}

	// Retrieve all posts
	posts, nextCursor, err := repo.ListByEvent(eventID, 10, nil)
	if err != nil {
		t.Fatalf("ListByEvent failed: %v", err)
	}

	if len(posts) != 5 {
		t.Errorf("expected 5 posts, got %d", len(posts))
	}

	if nextCursor != nil {
		t.Error("expected nil cursor (no more posts)")
	}

	// Verify all posts belong to this event
	for _, p := range posts {
		if p.EventID == nil || *p.EventID != eventID {
			t.Errorf("post %s does not belong to event %s", p.ID, eventID)
		}
	}
}

// TestListByScene_EmptyResult tests behavior when no posts exist.
func TestListByScene_EmptyResult(t *testing.T) {
	repo := NewInMemoryPostRepository()
	sceneID := "scene123"

	posts, cursor, err := repo.ListByScene(sceneID, 10, nil)
	if err != nil {
		t.Fatalf("ListByScene failed: %v", err)
	}

	if len(posts) != 0 {
		t.Errorf("expected 0 posts, got %d", len(posts))
	}

	if cursor != nil {
		t.Error("expected nil cursor for empty result")
	}
}

// TestListByScene_TieBreaking tests ID-based tie-breaking for posts with same timestamp.
func TestListByScene_TieBreaking(t *testing.T) {
	repo := NewInMemoryPostRepository()
	sceneID := "scene123"

	// Create posts with identical timestamps but different IDs
	now := time.Now()
	var postIDs []string
	for i := 0; i < 3; i++ {
		post := &Post{
			SceneID:   &sceneID,
			AuthorDID: "did:example:user1",
			Text:      "Post " + string(rune('A'+i)),
		}
		if err := repo.Create(post); err != nil {
			t.Fatalf("failed to create post: %v", err)
		}
		// Manually override the created timestamp to be the same for all posts
		// This simulates posts created at exactly the same time
		retrievedPost, err := repo.GetByID(post.ID)
		if err != nil {
			t.Fatalf("failed to retrieve post: %v", err)
		}
		// Access the internal map to set timestamp (in production DB, this would be set by DB)
		repo.mu.Lock()
		repo.posts[retrievedPost.ID].CreatedAt = now
		repo.mu.Unlock()
		
		postIDs = append(postIDs, post.ID)
	}

	// Retrieve posts
	posts, _, err := repo.ListByScene(sceneID, 10, nil)
	if err != nil {
		t.Fatalf("ListByScene failed: %v", err)
	}

	if len(posts) != 3 {
		t.Fatalf("expected 3 posts, got %d", len(posts))
	}

	// Verify all have same timestamp
	for i := 1; i < len(posts); i++ {
		if !posts[i].CreatedAt.Equal(posts[0].CreatedAt) {
			t.Errorf("posts do not have same timestamp: %v vs %v", posts[i].CreatedAt, posts[0].CreatedAt)
		}
	}

	// Verify IDs are in ascending order (lexicographic)
	for i := 1; i < len(posts); i++ {
		if posts[i-1].ID >= posts[i].ID {
			t.Errorf("IDs not in ascending order: %s >= %s", posts[i-1].ID, posts[i].ID)
		}
	}
}

// TestListByScene_OtherSceneExcluded tests that posts from other scenes are excluded.
func TestListByScene_OtherSceneExcluded(t *testing.T) {
	repo := NewInMemoryPostRepository()
	targetScene := "scene123"
	otherScene := "scene456"

	// Create 3 posts for target scene
	now := time.Now()
	for i := 0; i < 3; i++ {
		post := &Post{
			SceneID:   &targetScene,
			AuthorDID: "did:example:user1",
			Text:      "Target scene post",
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
		if err := repo.Create(post); err != nil {
			t.Fatalf("failed to create post: %v", err)
		}
	}

	// Create 2 posts for other scene
	for i := 0; i < 2; i++ {
		post := &Post{
			SceneID:   &otherScene,
			AuthorDID: "did:example:user1",
			Text:      "Other scene post",
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
		if err := repo.Create(post); err != nil {
			t.Fatalf("failed to create post: %v", err)
		}
	}

	// Retrieve posts for target scene
	posts, _, err := repo.ListByScene(targetScene, 10, nil)
	if err != nil {
		t.Fatalf("ListByScene failed: %v", err)
	}

	// Should only return 3 posts from target scene
	if len(posts) != 3 {
		t.Errorf("expected 3 posts, got %d", len(posts))
	}

	// Verify all posts belong to target scene
	for _, p := range posts {
		if p.SceneID == nil || *p.SceneID != targetScene {
			t.Errorf("post %s does not belong to target scene", p.ID)
		}
	}
}
