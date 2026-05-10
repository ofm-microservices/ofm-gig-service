package nats

import (
	"errors"

	"gig-service/internal/domain"
)

// FailureReasonResolver maps gig domain errors to stable external reason
// strings.
type FailureReasonResolver interface {
	CreateGigFailureReason(err error) string
	PublishGigFailureReason(err error) string
}

// DomainFailureReasonResolver is the default domain-error-to-string mapper for
// gig lifecycle operations.
type DomainFailureReasonResolver struct{}

// NewDomainFailureReasonResolver constructs the default failure reason
// resolver.
func NewDomainFailureReasonResolver() FailureReasonResolver {
	return &DomainFailureReasonResolver{}
}

func (r *DomainFailureReasonResolver) CreateGigFailureReason(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidGigID):
		return "invalid gig id"
	case errors.Is(err, domain.ErrInvalidFreelancerID):
		return "invalid freelancer id"
	case errors.Is(err, domain.ErrInvalidPackageCount):
		return "invalid package count"
	case errors.Is(err, domain.ErrInvalidPackageTier):
		return "invalid package tier"
	default:
		return "failed to create gig"
	}
}

func (r *DomainFailureReasonResolver) PublishGigFailureReason(err error) string {
	switch {
	case errors.Is(err, domain.ErrGigDraftIncomplete):
		return "gig draft is incomplete"
	case errors.Is(err, domain.ErrGigAlreadyPublished):
		return "gig already published"
	default:
		return "failed to publish gig"
	}
}
