package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gig-service/config"
	app "gig-service/internal/application"
	"gig-service/internal/domain"
	eventbroker "gig-service/internal/presentation/event_broker"
	"github.com/jmoiron/sqlx"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/migration/events"
	requestmetadata "github.com/ofm-microservices/ofm-common/pkg/observability/metadata"
	commonrealtime "github.com/ofm-microservices/ofm-common/pkg/realtime"
	"github.com/ofm-microservices/ofm-common/pkg/resilience"
)

// GigRecoverySubscriber consumes durable monolith fallback commands owned by
// gig-service and dispatches them directly to the application boundary.
type GigRecoverySubscriber interface {
	Subscribe(context.Context) error
}

type gigRecoverySubscriber struct {
	broker eventBroker
	svc    app.GigService
	cfg    config.KafkaConfig
	log    logging.Logger
	db     *sqlx.DB
}

type eventBroker interface {
	RunPullConsumer(context.Context, config.PullConsumerConfig, eventbroker.MessageHandler) error
	Publish(context.Context, string, []byte) error
}

// NewGigRecoverySubscriber constructs the gig-owned recovery adapter.
func NewGigRecoverySubscriber(broker eventBroker, svc app.GigService, cfg config.KafkaConfig, log logging.Logger, db *sqlx.DB) (GigRecoverySubscriber, error) {
	if broker == nil || svc == nil || log == nil || db == nil {
		return nil, ErrInvalidSubscriberDependency
	}
	return &gigRecoverySubscriber{broker: broker, svc: svc, cfg: cfg, log: log.With(logging.String("module", "kafka-gig-recovery-subscriber")), db: db}, nil
}

func (s *gigRecoverySubscriber) Subscribe(ctx context.Context) error {
	return s.broker.RunPullConsumer(ctx, config.PullConsumerConfig{
		Subject: s.cfg.RecoveryTopic,
		GroupID: s.cfg.RecoveryGroup,
	}, s.handle)
}

func (s *gigRecoverySubscriber) handle(ctx context.Context, _ string, payload []byte) error {
	ctx = app.WithRecoveryContext(ctx)
	var command events.Envelope
	if err := json.Unmarshal(payload, &command); err != nil {
		return fmt.Errorf("decode gig recovery command: %w", err)
	}
	// Log every decoded command before validation or dispatch. This is
	// intentionally outside the verbose-only block: a command that is routed
	// incorrectly must remain observable instead of disappearing silently.
	s.log.Info("recovery command read",
		logging.Operation("gig.recovery.read"),
		logging.String("command_id", command.CommandID),
		logging.String("event_id", command.EventID),
		logging.String("event_type", command.EventType),
		logging.String("aggregate_type", command.AggregateType),
		logging.String("aggregate_id", command.AggregateID),
		logging.String("operation", command.Operation),
		logging.String("command_path", command.CommandPath),
		logging.String("test_run_id", command.TestRunID),
		logging.String("correlation_id", command.CorrelationID),
		logging.Int("payload_bytes", len(command.Payload)),
		logging.Any("command", command),
	)
	// Recovery invokes the normal application use cases. Restore the command's
	// correlation metadata before entering them so their realtime and
	// projection events keep the original operation_id instead of generating a
	// fresh random ID that the waiting client cannot correlate.
	ctx = requestmetadata.WithValues(ctx, requestmetadata.Values{
		CorrelationID:  command.CorrelationID,
		IdempotencyKey: command.IdempotencyKey,
		TestRunID:      command.TestRunID,
	})
	ctx = app.WithRecoveryContext(ctx)
	if command.AggregateType != "gig" && command.AggregateType != "gigs" {
		err := fmt.Errorf("unsupported gig recovery aggregate type: %q", command.AggregateType)
		s.log.Error("recovery command rejected", logging.Operation("gig.recovery.rejected"), logging.String("command_id", command.CommandID), logging.String("aggregate_type", command.AggregateType), logging.Err(err))
		return err
	}
	if strings.TrimSpace(command.AggregateID) == "" {
		return resilience.Permanent(fmt.Errorf("gig recovery command %s is missing aggregate_id", command.CommandID))
	}
	s.log.Info("recovery command received", logging.Operation("gig.recovery.received"), logging.String("command_id", command.CommandID), logging.String("test_run_id", command.TestRunID), logging.String("correlation_id", command.CorrelationID), logging.String("operation", command.Operation))
	if logging.IsVerbose(s.log) {
		s.log.Info("recovery command payload decoded", logging.Operation("gig.recovery.payload"), logging.String("command_id", command.CommandID), logging.String("command_path", command.CommandPath), logging.String("aggregate_id", command.AggregateID), logging.String("aggregate_type", command.AggregateType), logging.Int("payload_bytes", len(command.Payload)), logging.Any("payload", json.RawMessage(command.Payload)))
	}

	var result any
	aggregateID := command.AggregateID
	var publishedGig *domain.Gig
	var err error
	switch {
	case command.Operation == strings.ToLower(http.MethodPost) && strings.HasSuffix(command.CommandPath, "/gigs/drafts"):
		var request struct {
			FreelancerID      string `json:"freelancer_id"`
			FreelancerIDCamel string `json:"freelancerId"`
		}
		if err = json.Unmarshal(command.Payload, &request); err != nil {
			return fmt.Errorf("decode create gig recovery payload: %w", err)
		}
		freelancerID := strings.TrimSpace(request.FreelancerID)
		if freelancerID == "" {
			freelancerID = strings.TrimSpace(request.FreelancerIDCamel)
		}
		// The public create-draft HTTP contract derives the owner from JWT and
		// therefore legitimately captures an empty body in the monolith outbox.
		// Recovery must preserve that identity in the envelope, not reject the
		// command merely because the transport payload was {}.
		if freelancerID == "" {
			freelancerID = strings.TrimSpace(command.RecoveryPrincipalID)
		}
		if freelancerID == "" {
			return resilience.Permanent(fmt.Errorf("gig recovery command %s is missing freelancer_id", command.CommandID))
		}
		var gig *domain.Gig
		var callErr error
		var requestedID string
		var draftPayload struct {
			GigID string `json:"gig_id"`
		}
		_ = json.Unmarshal(command.Payload, &draftPayload)
		requestedID = strings.TrimSpace(draftPayload.GigID)
		if creator, ok := s.svc.(interface {
			CreateDraftWithID(context.Context, string, string) (*domain.Gig, error)
		}); ok && requestedID != "" {
			gig, callErr = creator.CreateDraftWithID(ctx, requestedID, freelancerID)
		} else {
			gig, callErr = s.svc.CreateDraft(ctx, freelancerID)
		}
		err = callErr
		if gig != nil {
			aggregateID = gig.ID
			result = map[string]any{"gig_id": gig.ID, "freelancer_id": gig.FreelancerID, "status": gig.Status}
			if _, ledgerErr := s.db.ExecContext(ctx, `UPDATE processed_events SET aggregate_id=$1, aggregate_type='gig' WHERE event_id=$2`, gig.ID, command.CommandID); ledgerErr != nil {
				return fmt.Errorf("persist gig recovery result: %w", ledgerErr)
			}
		}
	case command.Operation == strings.ToLower(http.MethodPatch) && strings.HasSuffix(command.CommandPath, "/basic-info"):
		var request struct {
			GigID        string `json:"gig_id"`
			FreelancerID string `json:"freelancer_id"`
			Title        string `json:"title"`
			ShortInfo    string `json:"short_info"`
			Description  string `json:"description"`
			CategoryID   int64  `json:"category_id"`
			Currency     string `json:"currency"`
		}
		if err = json.Unmarshal(command.Payload, &request); err != nil {
			return fmt.Errorf("decode basic info recovery payload: %w", err)
		}
		if request.GigID == "" {
			request.GigID = recoveryGigID(command, aggregateID)
		}
		request.FreelancerID = recoveryFreelancerID(request.FreelancerID, command)
		gig, callErr := s.svc.UpdateBasicInfo(ctx, request.GigID, request.FreelancerID, domain.UpdateBasicInfoParams{Title: request.Title, ShortInfo: request.ShortInfo, Description: request.Description, CategoryID: request.CategoryID, Currency: request.Currency})
		err = callErr
		if gig != nil {
			aggregateID = gig.ID
			result = map[string]any{"gig_id": gig.ID, "status": gig.Status}
		}
	case command.Operation == strings.ToLower(http.MethodPut) && strings.HasSuffix(command.CommandPath, "/packages"):
		var request struct {
			GigID        string `json:"gig_id"`
			FreelancerID string `json:"freelancer_id"`
			Packages     []struct {
				ID           string `json:"id"`
				Tier         string `json:"tier"`
				Description  string `json:"description"`
				DeliveryDays int32  `json:"delivery_days"`
				PriceCents   int64  `json:"price_cents"`
				SortOrder    int32  `json:"sort_order"`
			} `json:"packages"`
		}
		if err = json.Unmarshal(command.Payload, &request); err != nil {
			return fmt.Errorf("decode packages recovery payload: %w", err)
		}
		if request.GigID == "" {
			request.GigID = recoveryGigID(command, aggregateID)
		}
		request.FreelancerID = recoveryFreelancerID(request.FreelancerID, command)
		s.log.Info("recovery packages request decoded", logging.Operation("gig.recovery.packages.decode"), logging.String("command_id", command.CommandID), logging.String("gig_id", request.GigID), logging.String("freelancer_id", request.FreelancerID), logging.Int("package_count", len(request.Packages)), logging.Any("packages", request.Packages))
		packages := make([]domain.GigPackage, 0, len(request.Packages))
		for _, pkg := range request.Packages {
			if strings.TrimSpace(pkg.ID) == "" {
				return resilience.Permanent(fmt.Errorf("gig recovery command %s contains package without id", command.CommandID))
			}
			packages = append(packages, domain.GigPackage{ID: pkg.ID, Tier: pkg.Tier, Description: pkg.Description, DeliveryDays: pkg.DeliveryDays, PriceCents: pkg.PriceCents, SortOrder: pkg.SortOrder})
		}
		gig, callErr := s.svc.ReplacePackages(ctx, request.GigID, request.FreelancerID, domain.ReplacePackagesParams{Packages: packages})
		err = callErr
		fields := []logging.Field{logging.Operation("gig.recovery.packages.result"), logging.String("command_id", command.CommandID), logging.String("gig_id", request.GigID), logging.String("freelancer_id", request.FreelancerID), logging.Int("package_count", len(packages))}
		if gig != nil {
			fields = append(fields, logging.String("result_gig_id", gig.ID), logging.Any("packages_completed", gig.PackagesCompleted))
		}
		if err != nil {
			s.log.Error("recovery packages application failed", append(fields, logging.Err(err))...)
		} else {
			s.log.Info("recovery packages application result", fields...)
		}
		if gig != nil {
			aggregateID = gig.ID
			result = map[string]any{"gig_id": gig.ID, "status": gig.Status}
		}
	case command.Operation == strings.ToLower(http.MethodPut) && strings.HasSuffix(command.CommandPath, "/requirements"):
		var request struct {
			GigID        string               `json:"gig_id"`
			FreelancerID string               `json:"freelancer_id"`
			Questions    []domain.GigQuestion `json:"questions"`
		}
		if err = json.Unmarshal(command.Payload, &request); err != nil {
			return fmt.Errorf("decode questions recovery payload: %w", err)
		}
		if request.GigID == "" {
			request.GigID = recoveryGigID(command, aggregateID)
		}
		request.FreelancerID = recoveryFreelancerID(request.FreelancerID, command)
		gig, callErr := s.svc.ReplaceQuestions(ctx, request.GigID, request.FreelancerID, domain.ReplaceQuestionsParams{Questions: request.Questions})
		err = callErr
		if gig != nil {
			aggregateID = gig.ID
			result = map[string]any{"gig_id": gig.ID, "status": gig.Status}
		}
	case command.Operation == strings.ToLower(http.MethodPut) && strings.HasSuffix(command.CommandPath, "/media"):
		var request struct {
			GigID        string `json:"gig_id"`
			FreelancerID string `json:"freelancer_id"`
			Files        []struct {
				Filename    string `json:"filename"`
				ContentType string `json:"content_type"`
				Data        string `json:"data"`
			} `json:"files"`
		}
		if err = json.Unmarshal(command.Payload, &request); err != nil {
			return fmt.Errorf("decode media recovery payload: %w", err)
		}
		if request.GigID == "" {
			request.GigID = recoveryGigID(command, aggregateID)
		}
		request.FreelancerID = recoveryFreelancerID(request.FreelancerID, command)
		files := make([]domain.MediaUpload, 0, len(request.Files))
		for _, file := range request.Files {
			data, decodeErr := base64.StdEncoding.DecodeString(file.Data)
			if decodeErr != nil {
				return fmt.Errorf("decode media file: %w", decodeErr)
			}
			files = append(files, domain.MediaUpload{Filename: file.Filename, ContentType: file.ContentType, Data: data})
		}
		gig, callErr := s.svc.ReplaceMedia(ctx, request.GigID, request.FreelancerID, domain.ReplaceMediaUploadParams{Files: files})
		err = callErr
		if gig != nil {
			aggregateID = gig.ID
			result = map[string]any{"gig_id": gig.ID, "status": gig.Status}
		}
	case command.Operation == strings.ToLower(http.MethodPost) && strings.HasSuffix(command.CommandPath, "/publish"):
		var request struct {
			GigID        string `json:"gig_id"`
			FreelancerID string `json:"freelancer_id"`
			Username     string `json:"username"`
		}
		if len(command.Payload) > 0 {
			if err = json.Unmarshal(command.Payload, &request); err != nil {
				return fmt.Errorf("decode publish recovery payload: %w", err)
			}
		}
		if request.GigID == "" {
			request.GigID = recoveryGigID(command, aggregateID)
		}
		request.FreelancerID = recoveryFreelancerID(request.FreelancerID, command)
		if request.Username == "" {
			request.Username = command.RecoveryUsername
		}
		if request.Username == "" {
			if username, ok := recoveryTokenClaims(command.RecoveryAccessToken)["username"].(string); ok {
				request.Username = strings.TrimSpace(username)
			}
		}
		gig, callErr := s.svc.Publish(ctx, request.GigID, request.FreelancerID, request.Username)
		if errors.Is(callErr, domain.ErrGigAlreadyPublished) {
			// Recovery is idempotent: a command replayed after the original
			// publish must still emit completion instead of becoming a failure.
			gig, callErr = s.svc.GetByID(ctx, request.GigID, request.FreelancerID)
		}
		err = callErr
		if gig != nil {
			aggregateID = gig.ID
			publishedGig = gig
			result = map[string]any{"gig_id": gig.ID, "status": gig.Status}
		}
	default:
		return fmt.Errorf("unsupported gig recovery command operation=%s path=%s", command.Operation, command.CommandPath)
	}
	if err != nil {
		return err
	}
	completed := events.Envelope{
		EventID: command.EventID + ".completed", CommandID: command.CommandID,
		CorrelationID: command.CorrelationID, CausationID: command.EventID,
		IdempotencyKey: command.IdempotencyKey, TestRunID: command.TestRunID,
		EventType: "migration.recovery.completed", Operation: command.Operation,
		SchemaVersion: 1, AggregateType: "gig", SourceService: "gig-service-recovery",
		AggregateID: aggregateID, OccurredAt: time.Now().UTC(), Payload: mustJSON(result),
	}
	data, marshalErr := json.Marshal(completed)
	if marshalErr != nil {
		return fmt.Errorf("encode gig recovery completion: %w", marshalErr)
	}
	if err := s.broker.Publish(context.WithoutCancel(ctx), s.cfg.RecoveryCompletedTopic, data); err != nil {
		s.log.Error("recovery completion publish failed", logging.Operation("gig.recovery.completion_publish"), logging.String("command_id", command.CommandID), logging.Err(err))
		return err
	}
	// Recovery is an internal transport, but a recovered publish is still a
	// user-visible business outcome. Emit the same terminal notification as the
	// synchronous application path so clients and event-driven tests can safely
	// continue only after the service-owned state is restored.
	if publishedGig != nil {
		notificationOperationID := strings.TrimSpace(command.IdempotencyKey)
		if notificationOperationID == "" {
			notificationOperationID = command.CommandID
		}
		if err := app.PublishGigNotificationOutcome(
			context.WithoutCancel(ctx), s.broker, publishedGig, "gig.published",
			commonrealtime.StatusCompleted, "", notificationOperationID,
			command.CorrelationID, nil, nil,
		); err != nil {
			s.log.Warn("recovered gig publish notification failed",
				logging.String("gig_id", publishedGig.ID), logging.Err(err))
		}
	}
	s.log.Info("recovery command completed", logging.Operation("gig.recovery.completed"), logging.String("command_id", command.CommandID), logging.String("test_run_id", command.TestRunID), logging.String("correlation_id", command.CorrelationID), logging.String("aggregate_id", aggregateID))
	return nil
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}

// recoveryFreelancerID preserves the authenticated owner identity when the
// public command body intentionally contains only the gig identifier.
func recoveryFreelancerID(candidate string, command events.Envelope) string {
	if id := strings.TrimSpace(candidate); id != "" {
		return id
	}
	if id := strings.TrimSpace(command.RecoveryPrincipalID); id != "" {
		return id
	}
	claims := recoveryTokenClaims(command.RecoveryAccessToken)
	if id, ok := claims["sub"].(string); ok {
		return strings.TrimSpace(id)
	}
	return ""
}

// recoveryTokenClaims reads identity claims from a token already captured by
// the trusted monolith fallback envelope. The token is not accepted from a
// public transport here; it only preserves the authenticated principal for a
// durable internal recovery command created by the monolith.
func recoveryTokenClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}

// recoveryGigID preserves the UUID from a recovery URL when multipart or
// boundary-rewritten payloads cannot carry gig_id. The command path is part of
// the durable fallback envelope and is therefore deterministic and auditable.
func recoveryGigID(command events.Envelope, fallback string) string {
	path := strings.TrimSpace(command.CommandPath)
	marker := "/gigs/"
	if index := strings.Index(strings.ToLower(path), marker); index >= 0 {
		value := path[index+len(marker):]
		if slash := strings.IndexByte(value, '/'); slash >= 0 {
			value = value[:slash]
		}
		if value != "" && value != "drafts" {
			return value
		}
	}
	return strings.TrimSpace(fallback)
}
