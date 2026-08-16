// Package post provides a SQL-backed implementation of the PostRepository.
package post

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type SQLPostRepository struct{ db *sql.DB }

func NewSQLPostRepository(db *sql.DB) *SQLPostRepository {
	return &SQLPostRepository{db: db}
}

type rowScanner interface{ Scan(...any) error }

const postColumns = `id::text,COALESCE(scene_id::text,''),COALESCE(event_id::text,''),author_did,text,COALESCE(attachments,'[]'::jsonb),COALESCE(labels,'{}'),COALESCE(record_did,''),COALESCE(record_rkey,''),created_at,updated_at,deleted_at`

func scanPost(row rowScanner) (*Post, error) {
	var p Post
	var sceneID, eventID, rdID, rrKey sql.NullString
	var deletedAt sql.NullTime
	var labels pq.StringArray
	var attachmentsBytes []byte
	err := row.Scan(&p.ID, &sceneID, &eventID, &p.AuthorDID, &p.Text, &attachmentsBytes, &labels, &rdID, &rrKey, &p.CreatedAt, &p.UpdatedAt, &deletedAt)
	if err != nil {
		return nil, err
	}
	p.SceneID = stringPointer(sceneID)
	p.EventID = stringPointer(eventID)
	p.RecordDID = stringPointer(rdID)
	p.RecordRKey = stringPointer(rrKey)
	p.Labels = []string(labels)
	if deletedAt.Valid {
		p.DeletedAt = &deletedAt.Time
	}
	if len(attachmentsBytes) > 0 && string(attachmentsBytes) != "[]" {
		json.Unmarshal(attachmentsBytes, &p.Attachments)
	}
	return &p, nil
}

func stringPointer(value sql.NullString) *string {
	if !value.Valid || value.String == "" {
		return nil
	}
	result := value.String
	return &result
}

func (r *SQLPostRepository) Upsert(post *Post) (*UpsertResult, error) {
	if post.RecordDID != nil && *post.RecordDID != "" && post.RecordRKey != nil && *post.RecordRKey != "" {
		existing, err := r.GetByRecordKey(*post.RecordDID, *post.RecordRKey)
		if err == nil {
			post.ID = existing.ID
			post.CreatedAt = existing.CreatedAt
			attachmentsJSON, _ := json.Marshal(post.Attachments)
			_, err = r.db.Exec(`UPDATE posts SET text=$2,attachments=$3::jsonb,labels=$4,scene_id=$5::uuid,event_id=$6::uuid,updated_at=NOW() WHERE id=$1::uuid`,
				post.ID, post.Text, attachmentsJSON, pq.Array(post.Labels), nullString(post.SceneID), nullString(post.EventID))
			if err != nil {
				return nil, fmt.Errorf("post upsert update: %w", err)
			}
			post.UpdatedAt = time.Now()
			return &UpsertResult{ID: post.ID}, nil
		}
		if !errors.Is(err, ErrPostNotFound) {
			return nil, err
		}
	}
	now := time.Now()
	if post.ID == "" {
		post.ID = uuid.New().String()
	}
	post.CreatedAt = now
	post.UpdatedAt = now
	attachmentsJSON, _ := json.Marshal(post.Attachments)
	_, err := r.db.Exec(`INSERT INTO posts(id,scene_id,event_id,author_did,text,attachments,labels,record_did,record_rkey,created_at,updated_at)
		VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6::jsonb,$7,$8,$9,$10,$11)`,
		post.ID, nullString(post.SceneID), nullString(post.EventID), post.AuthorDID, post.Text, attachmentsJSON, pq.Array(post.Labels),
		nullString(post.RecordDID), nullString(post.RecordRKey), post.CreatedAt, post.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("post upsert insert: %w", err)
	}
	return &UpsertResult{Inserted: true, ID: post.ID}, nil
}

func (r *SQLPostRepository) Create(post *Post) error {
	now := time.Now()
	post.ID = uuid.New().String()
	post.CreatedAt = now
	post.UpdatedAt = now
	attachmentsJSON, _ := json.Marshal(post.Attachments)
	_, err := r.db.Exec(`INSERT INTO posts(id,scene_id,event_id,author_did,text,attachments,labels,record_did,record_rkey,created_at,updated_at)
		VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6::jsonb,$7,$8,$9,$10,$11)`,
		post.ID, nullString(post.SceneID), nullString(post.EventID), post.AuthorDID, post.Text, attachmentsJSON, pq.Array(post.Labels),
		nullString(post.RecordDID), nullString(post.RecordRKey), post.CreatedAt, post.UpdatedAt)
	return err
}

func (r *SQLPostRepository) Update(post *Post) error {
	attachmentsJSON, _ := json.Marshal(post.Attachments)
	result, err := r.db.Exec(`UPDATE posts SET text=$2,attachments=$3::jsonb,labels=$4,updated_at=NOW() WHERE id=$1::uuid AND deleted_at IS NULL`,
		post.ID, post.Text, attachmentsJSON, pq.Array(post.Labels))
	if err != nil {
		return fmt.Errorf("update post: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var deleted bool
		r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM posts WHERE id=$1::uuid AND deleted_at IS NOT NULL)`, post.ID).Scan(&deleted)
		if deleted {
			return ErrPostDeleted
		}
		return ErrPostNotFound
	}
	return nil
}

func (r *SQLPostRepository) Delete(id string) error {
	result, err := r.db.Exec(`UPDATE posts SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1::uuid AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrPostNotFound
	}
	return nil
}

func (r *SQLPostRepository) GetByID(id string) (*Post, error) {
	p, err := scanPost(r.db.QueryRow(`SELECT `+postColumns+` FROM posts WHERE id=$1::uuid AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPostNotFound
	}
	return p, err
}

func (r *SQLPostRepository) GetByRecordKey(did, rkey string) (*Post, error) {
	p, err := scanPost(r.db.QueryRow(`SELECT `+postColumns+` FROM posts WHERE record_did=$1 AND record_rkey=$2`, did, rkey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPostNotFound
	}
	return p, err
}

func (r *SQLPostRepository) ListByScene(sceneID string, limit int, cursor *FeedCursor) ([]*Post, *FeedCursor, error) {
	return r.listFeed("scene_id", sceneID, limit, cursor)
}

func (r *SQLPostRepository) ListByEvent(eventID string, limit int, cursor *FeedCursor) ([]*Post, *FeedCursor, error) {
	return r.listFeed("event_id", eventID, limit, cursor)
}

func (r *SQLPostRepository) listFeed(col, id string, limit int, cursor *FeedCursor) ([]*Post, *FeedCursor, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{id}
	where := fmt.Sprintf(`%s=$1::uuid AND deleted_at IS NULL AND NOT ($2::text[] && COALESCE(labels,'{}'::text[]))`, col)
	args = append(args, pq.Array([]string{LabelHidden}))
	if cursor != nil {
		where += ` AND (created_at < $3 OR (created_at = $3 AND id::text < $4))`
		args = append(args, cursor.CreatedAt, cursor.ID)
	}
	query := fmt.Sprintf(`SELECT %s FROM posts WHERE %s ORDER BY created_at DESC,id::text DESC LIMIT %d`, postColumns, where, limit+1)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list feed: %w", err)
	}
	defer rows.Close()
	var posts []*Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, nil, err
		}
		posts = append(posts, p)
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	var nextCursor *FeedCursor
	if len(posts) > limit {
		last := posts[limit-1]
		nextCursor = &FeedCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		posts = posts[:limit]
	}
	return posts, nextCursor, nil
}

func (r *SQLPostRepository) SearchPosts(query string, sceneID *string, limit int, cursor string, trustScores map[string]float64) ([]*Post, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	normalized := "%" + strings.TrimSpace(query) + "%"
	if strings.TrimSpace(query) == "" {
		return []*Post{}, "", nil
	}

	args := []any{normalized}
	where := `deleted_at IS NULL AND NOT ($2::text[] && COALESCE(labels,'{}'::text[])) AND text ILIKE $1`
	args = append(args, pq.Array([]string{LabelHidden, LabelSpam, LabelFlagged}))

	if sceneID != nil && *sceneID != "" {
		where += fmt.Sprintf(" AND scene_id=$%d::uuid", len(args)+1)
		args = append(args, *sceneID)
	}

	orderBy := `ORDER BY id::text ASC`
	decodedCursor, _ := DecodePostScoreCursor(cursor)
	if decodedCursor != nil {
		where += fmt.Sprintf(` AND id::text > $%d`, len(args)+1)
		args = append(args, decodedCursor.ID)
	}

	querySQL := fmt.Sprintf(`SELECT %s FROM posts WHERE %s %s LIMIT %d`, postColumns, where, orderBy, limit)
	rows, err := r.db.Query(querySQL, args...)
	if err != nil {
		return nil, "", fmt.Errorf("search posts: %w", err)
	}
	defer rows.Close()
	var posts []*Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, "", err
		}
		posts = append(posts, p)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(posts) == limit {
		last := posts[len(posts)-1]
		nextCursor = EncodePostScoreCursor(0, last.ID)
	}
	return posts, nextCursor, nil
}

func nullString(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return *value
}

var _ PostRepository = (*SQLPostRepository)(nil)
