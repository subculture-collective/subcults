//go:build integration

package post_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/post"
	"github.com/onnwee/subcults/internal/testutil"
)

func insertPostScene(t *testing.T, tdb *testutil.TestDB) string {
	t.Helper()
	sceneID := uuid.New().String()
	tdb.DB.Exec(`DELETE FROM posts`)
	_, err := tdb.DB.Exec(`INSERT INTO scenes(id,owner_did,name,description,coarse_geohash,allow_precise) VALUES($1,$2,$3,$4,$5,FALSE)`,
		sceneID, "did:plc:poster", "Test Post Scene", "Scene for post tests", "u4pruydqqvj")
	if err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	return sceneID
}

func TestSQLPost_UpsertInsert(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := post.NewSQLPostRepository(tdb.DB)
	sceneID := insertPostScene(t, tdb)

	p := &post.Post{
		SceneID:    &sceneID,
		AuthorDID:  "did:plc:author",
		Text:       "hello world",
		Labels:     []string{},
		RecordDID:  strPtr("did:plc:record-did"),
		RecordRKey: strPtr("test-post-key"),
	}
	result, err := repo.Upsert(p)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !result.Inserted {
		t.Error("expected Inserted=true")
	}
	if p.ID == "" {
		t.Error("expected ID to be set")
	}

	got, err := repo.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Text != "hello world" {
		t.Errorf("Text = %q", got.Text)
	}
}

func TestSQLPost_UpsertUpdate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := post.NewSQLPostRepository(tdb.DB)
	sceneID := insertPostScene(t, tdb)

	p1 := &post.Post{
		SceneID:    &sceneID,
		AuthorDID:  "did:plc:author",
		Text:       "original",
		RecordDID:  strPtr("did:plc:record-did-2"),
		RecordRKey: strPtr("test-post-key-2"),
	}
	repo.Upsert(p1)

	p2 := &post.Post{
		SceneID:    &sceneID,
		AuthorDID:  "did:plc:author",
		Text:       "updated",
		RecordDID:  strPtr("did:plc:record-did-2"),
		RecordRKey: strPtr("test-post-key-2"),
	}
	result, err := repo.Upsert(p2)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if result.Inserted {
		t.Error("expected Inserted=false on update")
	}

	got, _ := repo.GetByID(p2.ID)
	if got.Text != "updated" {
		t.Errorf("Text = %q, want 'updated'", got.Text)
	}
}

func TestSQLPost_CreateAndUpdate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := post.NewSQLPostRepository(tdb.DB)
	sceneID := insertPostScene(t, tdb)

	p := &post.Post{
		SceneID:   &sceneID,
		AuthorDID: "did:plc:author",
		Text:      "initial",
	}
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	p.Text = "modified"
	if err := repo.Update(p); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(p.ID)
	if got.Text != "modified" {
		t.Errorf("Text = %q", got.Text)
	}
}

func TestSQLPost_Delete(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := post.NewSQLPostRepository(tdb.DB)
	sceneID := insertPostScene(t, tdb)

	p := &post.Post{SceneID: &sceneID, AuthorDID: "did:plc:author", Text: "to-delete"}
	repo.Create(p)

	if err := repo.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(p.ID)
	if err != post.ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
	// Second delete should also return ErrPostNotFound
	if err := repo.Delete(p.ID); err != post.ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound on second delete, got %v", err)
	}
}

func TestSQLPost_GetByIDNotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := post.NewSQLPostRepository(tdb.DB)
	_, err := repo.GetByID("00000000-0000-0000-0000-000000000000")
	if err != post.ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestSQLPost_ListByScene(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := post.NewSQLPostRepository(tdb.DB)
	sceneID := insertPostScene(t, tdb)

	for i := 0; i < 3; i++ {
		p := &post.Post{SceneID: &sceneID, AuthorDID: "did:plc:author", Text: "post"}
		if err := repo.Create(p); err != nil {
			t.Fatalf("Create post %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	posts, next, err := repo.ListByScene(sceneID, 2, nil)
	if err != nil {
		t.Fatalf("ListByScene: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if next == nil {
		t.Error("expected next cursor")
	}

	posts2, next2, err := repo.ListByScene(sceneID, 2, next)
	if err != nil {
		t.Fatalf("ListByScene page 2: %v", err)
	}
	if len(posts2) != 1 {
		t.Errorf("expected 1 post on page 2, got %d", len(posts2))
	}
	if next2 != nil {
		t.Error("expected nil cursor on last page")
	}
}

func TestSQLPost_ListByEvent(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := post.NewSQLPostRepository(tdb.DB)
	sceneID := insertPostScene(t, tdb)

	eventID := uuid.New().String()
	_, err := tdb.DB.Exec(`INSERT INTO events(id,scene_id,title,starts_at) VALUES($1,$2,$3,NOW())`, eventID, sceneID, "Test Event")
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}

	repo.Create(&post.Post{EventID: &eventID, AuthorDID: "did:plc:author", Text: "event-post"})
	posts, _, err := repo.ListByEvent(eventID, 10, nil)
	if err != nil {
		t.Fatalf("ListByEvent: %v", err)
	}
	if len(posts) != 1 {
		t.Errorf("expected 1 post, got %d", len(posts))
	}
}

func TestSQLPost_HiddenPostsExcluded(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := post.NewSQLPostRepository(tdb.DB)
	sceneID := insertPostScene(t, tdb)

	repo.Create(&post.Post{SceneID: &sceneID, AuthorDID: "did:plc:author", Text: "visible", Labels: []string{}})
	repo.Create(&post.Post{SceneID: &sceneID, AuthorDID: "did:plc:author", Text: "hidden-post", Labels: []string{post.LabelHidden}})

	posts, _, _ := repo.ListByScene(sceneID, 10, nil)
	if len(posts) != 1 {
		t.Errorf("expected 1 visible post (hidden excluded), got %d", len(posts))
	}
}

func TestSQLPost_SearchPosts(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := post.NewSQLPostRepository(tdb.DB)
	sceneID := insertPostScene(t, tdb)

	repo.Create(&post.Post{SceneID: &sceneID, AuthorDID: "did:plc:author", Text: "searchable content here"})
	repo.Create(&post.Post{SceneID: &sceneID, AuthorDID: "did:plc:author", Text: "nothing matching"})

	results, next, err := repo.SearchPosts("searchable", nil, 10, "", nil)
	if err != nil {
		t.Fatalf("SearchPosts: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if next != "" {
		t.Error("expected empty cursor with 1 result")
	}
}

func strPtr(s string) *string { return &s }
