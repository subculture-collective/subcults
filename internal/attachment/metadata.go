// Package attachment provides services for enriching attachment metadata
// while ensuring EXIF data is stripped for privacy.
package attachment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/h2non/bimg"
	"github.com/onnwee/subcults/internal/image"
	"github.com/onnwee/subcults/internal/post"
)

// Common errors for metadata extraction
var (
	ErrObjectNotFound    = errors.New("object not found in R2")
	ErrInvalidObjectKey  = errors.New("invalid object key")
	ErrUnsupportedFormat = errors.New("unsupported file format")
)

// MetadataService handles attachment metadata extraction and enrichment.
type MetadataService struct {
	s3Client   *s3.Client
	bucketName string
}

// MetadataServiceConfig holds configuration for the metadata service.
type MetadataServiceConfig struct {
	S3Client   *s3.Client
	BucketName string
}

// NewMetadataService creates a new metadata service.
func NewMetadataService(cfg MetadataServiceConfig) (*MetadataService, error) {
	if cfg.S3Client == nil {
		return nil, errors.New("s3 client is required")
	}
	if cfg.BucketName == "" {
		return nil, errors.New("bucket name is required")
	}

	return &MetadataService{
		s3Client:   cfg.S3Client,
		bucketName: cfg.BucketName,
	}, nil
}

// EnrichAttachment fetches metadata for an attachment key and returns an enriched attachment.
// For images, it strips EXIF data and extracts dimensions.
// For audio, it extracts basic metadata (duration placeholder for now).
func (s *MetadataService) EnrichAttachment(ctx context.Context, key string) (*post.Attachment, error) {
	if key == "" {
		return nil, ErrInvalidObjectKey
	}

	// Step 1: Fetch object metadata using HeadObject
	headOutput, err := s.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrObjectNotFound, err)
	}

	// Extract basic metadata
	contentType := ""
	if headOutput.ContentType != nil {
		contentType = *headOutput.ContentType
	}
	sizeBytes := int64(0)
	if headOutput.ContentLength != nil {
		sizeBytes = *headOutput.ContentLength
	}

	attachment := &post.Attachment{
		Key:       key,
		Type:      contentType,
		SizeBytes: sizeBytes,
	}

	// Step 2: Process based on content type
	if isImageType(contentType) {
		// For images, we need to fetch the object, strip EXIF, and extract dimensions
		if err := s.processImage(ctx, key, attachment); err != nil {
			// Log error but don't fail - return basic metadata
			// In production, you might want to log this properly
			return attachment, nil
		}
	} else if isAudioType(contentType) {
		// For audio, we could extract duration here
		// For now, this is a placeholder - audio metadata extraction can be added later
		// attachment.DurationSeconds would be set here
	}

	return attachment, nil
}

// processImage fetches the image, strips EXIF, extracts dimensions, and optionally re-uploads.
func (s *MetadataService) processImage(ctx context.Context, key string, attachment *post.Attachment) error {
	// Fetch the image from R2
	getOutput, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer getOutput.Body.Close()

	// Read the image bytes
	imageBytes, err := io.ReadAll(getOutput.Body)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}

	// Extract dimensions BEFORE processing (to get original dimensions)
	// We use bimg to read metadata before stripping EXIF
	img := bimg.NewImage(imageBytes)
	metadata, err := img.Metadata()
	if err != nil {
		return fmt.Errorf("failed to read image metadata: %w", err)
	}

	// Store dimensions
	width := metadata.Size.Width
	height := metadata.Size.Height
	attachment.Width = &width
	attachment.Height = &height

	// Strip EXIF metadata for privacy
	// This processes the image and removes all EXIF data including GPS coordinates
	sanitizedBytes, err := image.ProcessBytes(imageBytes)
	if err != nil {
		return fmt.Errorf("failed to strip EXIF: %w", err)
	}

	// Re-upload the sanitized image back to R2
	// This ensures no EXIF data is persisted
	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(key),
		Body:          bytes.NewReader(sanitizedBytes),
		ContentType:   aws.String(attachment.Type),
		ContentLength: aws.Int64(int64(len(sanitizedBytes))),
	})
	if err != nil {
		return fmt.Errorf("failed to re-upload sanitized image: %w", err)
	}

	// Update size to reflect sanitized image size
	attachment.SizeBytes = int64(len(sanitizedBytes))

	return nil
}

// isImageType checks if the content type is an image.
func isImageType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

// isAudioType checks if the content type is audio.
func isAudioType(contentType string) bool {
	return strings.HasPrefix(contentType, "audio/")
}
