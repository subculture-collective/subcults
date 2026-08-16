// Package api provides HTTP API utilities including standardized error handling.
package api

import (
	"net/http"

	"github.com/onnwee/subcults/internal/alliance"
	"github.com/onnwee/subcults/internal/atprotocol"
	"github.com/onnwee/subcults/internal/identity"
	"github.com/onnwee/subcults/internal/livekit"
	"github.com/onnwee/subcults/internal/membership"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/signal"
	"github.com/onnwee/subcults/internal/stream"
	"github.com/onnwee/subcults/internal/touring"
)

// init registers all domain sentinel errors with the centralized error mapper.
// Each domain package's sentinel errors map to their HTTP status, error code,
// and human-readable message. Handlers use MapDomainError / WriteAPIError to
// convert domain errors to HTTP responses without per-handler switch statements.
func init() {
	// scene domain
	RegisterDomainError(scene.ErrSceneNotFound, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
	RegisterDomainError(scene.ErrSceneDeleted, http.StatusGone, ErrCodeSceneDeleted, "Scene deleted")
	RegisterDomainError(scene.ErrDuplicateSceneName, http.StatusConflict, ErrCodeDuplicateSceneName, "Scene name already exists for this owner")
	RegisterDomainError(scene.ErrEventNotFound, http.StatusNotFound, ErrCodeNotFound, "Event not found")
	RegisterDomainError(scene.ErrVersionConflict, http.StatusConflict, ErrCodeConflict, "The record changed; refresh before saving again")
	RegisterDomainError(scene.ErrInvalidSceneName, http.StatusBadRequest, ErrCodeInvalidSceneName, "Invalid scene name")
	RegisterDomainError(scene.ErrInvalidDescription, http.StatusBadRequest, ErrCodeValidation, "Invalid description")
	RegisterDomainError(scene.ErrInvalidVisibility, http.StatusBadRequest, ErrCodeValidation, "Invalid visibility mode")
	RegisterDomainError(scene.ErrInvalidTimeRange, http.StatusBadRequest, ErrCodeInvalidTimeRange, "Invalid time range")
	RegisterDomainError(scene.ErrInvalidEventTitle, http.StatusBadRequest, ErrCodeValidation, "Invalid event title")
	RegisterDomainError(scene.ErrInvalidRSVPStatus, http.StatusBadRequest, ErrCodeValidation, "Invalid RSVP status")
	RegisterDomainError(scene.ErrEmptyCoarseGeohash, http.StatusBadRequest, ErrCodeValidation, "coarse_geohash is required")
	RegisterDomainError(scene.ErrEmptyOwnerDID, http.StatusBadRequest, ErrCodeValidation, "owner_did is required")
	RegisterDomainError(scene.ErrEmptySceneID, http.StatusBadRequest, ErrCodeValidation, "scene_id is required")
	RegisterDomainError(scene.ErrEventNotUpcoming, http.StatusBadRequest, ErrCodeValidation, "Cannot RSVP to past or ongoing events")

	// stream domain
	RegisterDomainError(stream.ErrStreamNotFound, http.StatusNotFound, ErrCodeNotFound, "Stream session not found")
	RegisterDomainError(stream.ErrParticipantNotFound, http.StatusNotFound, ErrCodeNotFound, "Participant not found")
	RegisterDomainError(stream.ErrQualityMetricsNotFound, http.StatusNotFound, ErrCodeNotFound, "Quality metrics not found")
	RegisterDomainError(stream.ErrAnalyticsNotFound, http.StatusNotFound, ErrCodeNotFound, "Analytics not found")

	// atprotocol domain
	RegisterDomainError(atprotocol.ErrPublicationScope, http.StatusPreconditionRequired, "atproto_scope_upgrade_required", "AT Protocol publishing permission is required")
	RegisterDomainError(atprotocol.ErrEntityForbidden, http.StatusForbidden, ErrCodeForbidden, "Entity is not owned by the authenticated creator")
	RegisterDomainError(atprotocol.ErrRecordConflict, http.StatusConflict, "atproto_record_conflict", "The PDS record changed; reload it before publishing")
	RegisterDomainError(atprotocol.ErrOAuthSessionNotFound, http.StatusPreconditionRequired, "atproto_link_required", "Link an AT Protocol account before publishing")
	RegisterDomainError(atprotocol.ErrOAuthRequestNotFound, http.StatusNotFound, ErrCodeNotFound, "Projection not found")
	RegisterDomainError(atprotocol.ErrProvisioningDisabled, http.StatusServiceUnavailable, "atproto_provisioning_disabled", "AT Protocol provisioning is temporarily disabled")
	RegisterDomainError(atprotocol.ErrProvisioningLimit, http.StatusTooManyRequests, "atproto_provisioning_limited", "Provisioning rate limit exceeded")
	RegisterDomainError(atprotocol.ErrProvisioningConflict, http.StatusConflict, "atproto_provisioning_conflict", "Provisioning request conflicts with existing state")
	RegisterDomainError(atprotocol.ErrEmailVerificationRequired, http.StatusForbidden, "verified_email_required", "Verified email is required")

	// signal domain
	RegisterDomainError(signal.ErrSignalNotFound, http.StatusNotFound, ErrCodeNotFound, "Signal not found")
	RegisterDomainError(signal.ErrRevisionNotFound, http.StatusNotFound, ErrCodeNotFound, "Signal revision not found")

	// touring domain
	RegisterDomainError(touring.ErrVersionConflict, http.StatusConflict, ErrCodeConflict, "The record changed; refresh before saving again")

	// identity domain
	RegisterDomainError(identity.ErrInvalidEmail, http.StatusBadRequest, "invalid_magic_link_request", "Invalid email address")
	RegisterDomainError(identity.ErrInvalidReturnPath, http.StatusBadRequest, "invalid_magic_link_request", "Invalid return path")

	// livekit domain
	RegisterDomainError(livekit.ErrRoomNotFound, http.StatusNotFound, ErrCodeNotFound, "LiveKit room not found")

	// membership domain
	RegisterDomainError(membership.ErrAlreadyActiveMember, http.StatusConflict, ErrCodeConflict, "User is already an active member")
	RegisterDomainError(membership.ErrPendingRequestExists, http.StatusConflict, ErrCodeConflict, "Pending membership request already exists")
	RegisterDomainError(membership.ErrNotPending, http.StatusConflict, ErrCodeConflict, "Only pending membership requests can be updated")

	// alliance domain
	RegisterDomainError(alliance.ErrInvalidWeight, http.StatusBadRequest, ErrCodeInvalidWeight, "weight must be between 0.0 and 1.0")
	RegisterDomainError(alliance.ErrSelfAlliance, http.StatusBadRequest, ErrCodeSelfAlliance, "Cannot create alliance with same scene")
	RegisterDomainError(alliance.ErrEmptySceneIDs, http.StatusBadRequest, ErrCodeValidation, "from_scene_id and to_scene_id are required")
	RegisterDomainError(alliance.ErrReasonEmpty, http.StatusBadRequest, ErrCodeValidation, "reason cannot be empty or whitespace only")
	RegisterDomainError(alliance.ErrReasonTooLong, http.StatusBadRequest, ErrCodeValidation, "reason must not exceed 256 characters")
}
