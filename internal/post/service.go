package post

import (
	"fmt"
	"strings"

	"github.com/onnwee/subcults/internal/validate"
)

// Post text validation constraints
const (
	MaxPostTextLength = 5000
	MaxAttachments    = 6
)

// Service contains business logic for post operations.
type Service struct {
	repo PostRepository
}

// NewService creates a new post Service.
func NewService(repo PostRepository) *Service {
	return &Service{repo: repo}
}

// CreatePostInput holds the parameters for creating a post.
type CreatePostInput struct {
	Text        string
	SceneID     *string
	EventID     *string
	Attachments []Attachment
	Labels      []string
	AuthorDID   string
}

// UpdatePostInput holds the parameters for updating a post.
type UpdatePostInput struct {
	Text        *string
	Attachments *[]Attachment
	Labels      *[]string
}

// CreatePost validates and creates a new post.
func (s *Service) CreatePost(input CreatePostInput) (*Post, error) {
	// Validate at least one of sceneId/eventId is provided
	if input.SceneID == nil && input.EventID == nil {
		return nil, fmt.Errorf("either scene_id or event_id must be provided")
	}

	// Validate and sanitize text
	validatedText, err := validate.PostContent(input.Text)
	if err != nil {
		return nil, fmt.Errorf("invalid post text: %w", err)
	}

	// Validate attachments count
	if len(input.Attachments) > MaxAttachments {
		return nil, fmt.Errorf("maximum %d attachments allowed", MaxAttachments)
	}

	// Validate attachment URLs for SSRF protection
	for i, att := range input.Attachments {
		if att.URL != "" {
			if _, err := validate.MediaURL(att.URL); err != nil {
				return nil, fmt.Errorf("attachment %d: invalid URL: %w", i, err)
			}
		}
	}

	// Sanitize and validate labels
	sanitizedLabels := make([]string, len(input.Labels))
	for i, label := range input.Labels {
		sanitizedLabels[i] = validate.SanitizeHTML(strings.TrimSpace(label))
	}
	if err := ValidateLabels(sanitizedLabels); err != nil {
		return nil, fmt.Errorf("invalid moderation label")
	}

	// Create post
	newPost := &Post{
		SceneID:     input.SceneID,
		EventID:     input.EventID,
		AuthorDID:   input.AuthorDID,
		Text:        validatedText,
		Attachments: input.Attachments,
		Labels:      sanitizedLabels,
	}

	if err := s.repo.Create(newPost); err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	return newPost, nil
}

// GetPost retrieves a post by ID.
func (s *Service) GetPost(id string) (*Post, error) {
	return s.repo.GetByID(id)
}

// ListPostsByScene retrieves posts for a scene with cursor-based pagination.
func (s *Service) ListPostsByScene(sceneID string, limit int, cursor *FeedCursor) ([]*Post, *FeedCursor, error) {
	return s.repo.ListByScene(sceneID, limit, cursor)
}

// ListPostsByEvent retrieves posts for an event with cursor-based pagination.
func (s *Service) ListPostsByEvent(eventID string, limit int, cursor *FeedCursor) ([]*Post, *FeedCursor, error) {
	return s.repo.ListByEvent(eventID, limit, cursor)
}

// UpdatePost validates and updates an existing post.
// Returns an error if the post is not found, is deleted, or the author doesn't match.
func (s *Service) UpdatePost(id, authorDID string, input UpdatePostInput) (*Post, error) {
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if existing.AuthorDID != authorDID {
		return nil, ErrPostNotFound
	}

	if input.Text != nil {
		validatedText, err := validate.PostContent(*input.Text)
		if err != nil {
			return nil, fmt.Errorf("invalid post text: %w", err)
		}
		existing.Text = validatedText
	}

	if input.Attachments != nil {
		if len(*input.Attachments) > MaxAttachments {
			return nil, fmt.Errorf("maximum %d attachments allowed", MaxAttachments)
		}
		for i, att := range *input.Attachments {
			if att.URL != "" {
				if _, err := validate.MediaURL(att.URL); err != nil {
					return nil, fmt.Errorf("attachment %d: invalid URL: %w", i, err)
				}
			}
		}
		existing.Attachments = *input.Attachments
	}

	if input.Labels != nil {
		sanitizedLabels := make([]string, len(*input.Labels))
		for i, label := range *input.Labels {
			sanitizedLabels[i] = validate.SanitizeHTML(strings.TrimSpace(label))
		}
		if err := ValidateLabels(sanitizedLabels); err != nil {
			return nil, fmt.Errorf("invalid moderation label")
		}
		existing.Labels = sanitizedLabels
	}

	if err := s.repo.Update(existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeletePost soft-deletes a post after verifying ownership.
func (s *Service) DeletePost(id, authorDID string) error {
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	if existing.AuthorDID != authorDID {
		return ErrPostNotFound
	}

	return s.repo.Delete(id)
}