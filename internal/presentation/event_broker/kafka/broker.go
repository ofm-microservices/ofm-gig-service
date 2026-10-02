package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"gig-service/config"
	"gig-service/internal/domain"
	eventbroker "gig-service/internal/presentation/event_broker"
	"github.com/jmoiron/sqlx"
	commonevents "github.com/ofm-microservices/ofm-common/pkg/events"
	"github.com/ofm-microservices/ofm-common/pkg/idempotency"
	"github.com/ofm-microservices/ofm-common/pkg/migration/events"
	kafkaprop "github.com/ofm-microservices/ofm-common/pkg/observability/kafka"
	transportkafka "github.com/ofm-microservices/ofm-common/pkg/observability/kafka"
	requestmetadata "github.com/ofm-microservices/ofm-common/pkg/observability/metadata"
	sharedmetrics "github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	"github.com/ofm-microservices/ofm-common/pkg/resilience"
	"github.com/segmentio/kafka-go"
)

type broker struct {
	brokers           []string
	group, deadLetter string
	mu                sync.Mutex
	readers           []*kafka.Reader
	writers           map[string]*kafka.Writer
	db                *sqlx.DB
}

func decodeMigrationEnvelope(payload []byte) (events.Envelope, error) {
	var envelope events.Envelope
	err := json.Unmarshal(payload, &envelope)
	return envelope, err
}

// NewBroker constructs the Kafka broker used by gig-service event adapters.
func NewBroker(cfg config.KafkaConfig) (eventbroker.EventBroker, error) {
	return newBroker(cfg, nil)
}

// NewBrokerWithDB enables durable event claims for gig projections.
func NewBrokerWithDB(cfg config.KafkaConfig, db *sqlx.DB) (eventbroker.EventBroker, error) {
	return newBroker(cfg, db)
}

func newBroker(cfg config.KafkaConfig, db *sqlx.DB) (eventbroker.EventBroker, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers are empty")
	}
	return &broker{brokers: cfg.Brokers, group: cfg.GroupID, deadLetter: cfg.DeadLetterTopic, writers: make(map[string]*kafka.Writer), db: db}, nil
}

func (b *broker) Publish(ctx context.Context, subject string, payload []byte) error {
	enveloped, _, err := commonevents.Wrap(subject, payload)
	if err != nil {
		return err
	}
	b.mu.Lock()
	w := b.writers[subject]
	if w == nil {
		w = &kafka.Writer{
			Addr:         kafka.TCP(b.brokers...),
			Topic:        subject,
			RequiredAcks: kafka.RequireOne,
			BatchSize:    32,
			BatchTimeout: 10 * time.Millisecond,
		}
		b.writers[subject] = w
	}
	b.mu.Unlock()
	transportkafka.Published(subject, enveloped)
	return w.WriteMessages(ctx, kafka.Message{Value: enveloped, Headers: kafkaHeaders(ctx)})
}

func kafkaHeaders(ctx context.Context) []kafka.Header {
	headers := make([]kafka.Header, 0, 5)
	for key, value := range requestmetadata.OutgoingHeaders(ctx) {
		headers = append(headers, kafka.Header{Key: key, Value: []byte(value)})
	}
	return headers
}

func (b *broker) Subscribe(ctx context.Context, subject string, handler eventbroker.MessageHandler) error {
	return b.RunPullConsumer(ctx, config.PullConsumerConfig{Subject: subject}, handler)
}

func (b *broker) RunPullConsumer(ctx context.Context, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler) error {
	if strings.Contains(cfg.Subject, "projection.requested") {
		return b.runProjectionBatchConsumer(ctx, cfg, handler)
	}
	groupID := strings.TrimSpace(cfg.GroupID)
	if groupID == "" {
		groupID = strings.TrimSpace(b.group) + "-" + strings.TrimSpace(cfg.Subject)
	}
	if strings.TrimSpace(cfg.GroupID) != "" {
		return b.runExplicitConsumerGroup(ctx, cfg, handler, groupID)
	}
	go func() {
		_ = (resilience.KafkaRetryQueueConfig{Brokers: b.brokers, Group: groupID, MaxAttempts: resilience.DefaultRetryPolicy.MaxAttempts}).Run(ctx)
	}()
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: b.brokers, Topic: cfg.Subject, GroupID: groupID,
		MinBytes: 1, MaxBytes: 16 << 20, MaxWait: 50 * time.Millisecond, StartOffset: kafka.FirstOffset,
	})
	b.mu.Lock()
	b.readers = append(b.readers, r)
	b.mu.Unlock()
	defer r.Close()
	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			return fmt.Errorf("fetch Kafka message topic=%s group=%s: %w", cfg.Subject, groupID, err)
		}
		attempts := retryAttempt(msg.Headers)
		var event idempotency.Event
		if b.db != nil {
			event = idempotency.DecodeOrFingerprint(cfg.Subject, msg.Value)
			if envelope, decodeErr := decodeMigrationEnvelope(msg.Value); decodeErr == nil && envelope.CommandID != "" {
				event.EventID = envelope.CommandID
			}
			event.EventID = idempotency.ScopedEventID(cfg.Subject, event.EventID)
			claimed, claimErr := idempotency.ClaimDB(ctx, b.db, event)
			if claimErr != nil {
				return claimErr
			}
			if !claimed {
				continue
			}
		}
		payload, _, unwrapErr := commonevents.Unwrap(msg.Value)
		if unwrapErr != nil {
			return b.deadLetterMessage(ctx, cfg.Subject, msg, attempts, unwrapErr)
		}
		transportkafka.Consumed(cfg.Subject, msg.Partition, msg.Offset, attempts, payload)
		err = handler(kafkaprop.Context(ctx, msg.Headers), cfg.Subject, payload)
		if err != nil {
			if b.db != nil {
				_ = idempotency.Release(ctx, b.db, event.EventID)
			}
			var permanent resilience.PermanentError
			if !errors.As(err, &permanent) && attempts < resilience.DefaultRetryPolicy.MaxAttempts {
				writer := &kafka.Writer{Addr: kafka.TCP(b.brokers...), Topic: resilience.RetryTopic(groupID), WriteTimeout: 5 * time.Second}
				queueErr := (resilience.KafkaRetryQueue{Writer: writer}).Enqueue(ctx, msg, cfg.Subject, attempts+1, err)
				_ = writer.Close()
				if queueErr != nil {
					return queueErr
				}
				continue
			}
			return b.deadLetterMessage(ctx, cfg.Subject, msg, attempts, err)
		}
		if err := r.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func retryAttempt(headers []kafka.Header) int {
	for _, h := range headers {
		if h.Key == "x-ofm-retry-attempt" {
			if n, err := strconv.Atoi(string(h.Value)); err == nil && n > 0 {
				return n
			}
		}
	}
	return 1
}

func (b *broker) deadLetterMessage(ctx context.Context, subject string, msg kafka.Message, attempts int, cause error) error {
	payload, err := resilience.MarshalDLQ(resilience.DLQRecord{OriginalKey: msg.Key, OriginalValue: msg.Value, OriginalTopic: subject, OriginalPartition: msg.Partition, OriginalOffset: msg.Offset, Attempts: attempts, ErrorClass: fmt.Sprintf("%T", cause), Error: cause.Error(), FailedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	writer := &kafka.Writer{Addr: kafka.TCP(b.brokers...), Topic: b.deadLetter, Balancer: &kafka.Hash{}, WriteTimeout: 5 * time.Second}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = writer.WriteMessages(writeCtx, kafka.Message{Key: msg.Key, Value: payload, Headers: msg.Headers})
	cancel()
	_ = writer.Close()
	if err != nil {
		return err
	}
	sharedmetrics.IncKafkaDLQ(b.deadLetter)
	return nil
}

// runProjectionBatchConsumer drains projection requests in bounded batches.
// Each message still gets its own idempotency claim and handler result; the
// batch only changes fetch/commit overhead and permits independent aggregates
// to be processed concurrently.
func (b *broker) runProjectionBatchConsumer(ctx context.Context, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler) error {
	groupID := strings.TrimSpace(cfg.GroupID)
	if groupID == "" {
		groupID = strings.TrimSpace(b.group) + "-" + strings.TrimSpace(cfg.Subject)
	}
	r := kafka.NewReader(kafka.ReaderConfig{Brokers: b.brokers, Topic: cfg.Subject, GroupID: groupID, MinBytes: 1, MaxBytes: 16 << 20, MaxWait: 50 * time.Millisecond, StartOffset: kafka.FirstOffset})
	defer r.Close()
	dlqWriter := &kafka.Writer{Addr: kafka.TCP(b.brokers...), Topic: b.deadLetter, Balancer: &kafka.Hash{}, WriteTimeout: 5 * time.Second}
	defer dlqWriter.Close()
	for {
		messages := make([]kafka.Message, 0, 32)
		first, err := r.FetchMessage(ctx)
		if err != nil {
			return fmt.Errorf("fetch projection batch topic=%s: %w", cfg.Subject, err)
		}
		messages = append(messages, first)
		for len(messages) < 32 {
			fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Millisecond)
			message, fetchErr := r.FetchMessage(fetchCtx)
			cancel()
			if fetchErr != nil {
				break
			}
			messages = append(messages, message)
		}
		work := make(chan kafka.Message)
		errs := make(chan error, len(messages))
		workers := 8
		if len(messages) < workers {
			workers = len(messages)
		}
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for message := range work {
					attempts := 0
					err := resilience.Retry(ctx, resilience.RetryPolicy{MaxAttempts: 1}, func(attemptCtx context.Context, _ int) error {
						attempts++
						var event idempotency.Event
						if b.db != nil {
							event = idempotency.DecodeOrFingerprint(cfg.Subject, message.Value)
							if envelope, decodeErr := decodeMigrationEnvelope(message.Value); decodeErr == nil && envelope.CommandID != "" {
								event.EventID = envelope.CommandID
							}
							event.EventID = idempotency.ScopedEventID(cfg.Subject, event.EventID)
							claimed, claimErr := idempotency.ClaimDB(attemptCtx, b.db, event)
							if claimErr != nil {
								return claimErr
							}
							if !claimed {
								return nil
							}
						}
						payload, _, unwrapErr := commonevents.Unwrap(message.Value)
						if unwrapErr != nil {
							return unwrapErr
						}
						handlerErr := handler(kafkaprop.Context(attemptCtx, message.Headers), cfg.Subject, payload)
						if handlerErr != nil && b.db != nil {
							_ = idempotency.Release(attemptCtx, b.db, event.EventID)
						}
						return handlerErr
					})
					if err != nil {
						payload, marshalErr := resilience.MarshalDLQ(resilience.DLQRecord{OriginalKey: message.Key, OriginalValue: message.Value, OriginalTopic: message.Topic, OriginalPartition: message.Partition, OriginalOffset: message.Offset, Attempts: attempts, ErrorClass: fmt.Sprintf("%T", err), Error: err.Error(), FailedAt: time.Now().UTC()})
						if marshalErr != nil {
							errs <- marshalErr
							continue
						}
						if dlqErr := dlqWriter.WriteMessages(ctx, kafka.Message{Key: message.Key, Value: payload, Headers: message.Headers}); dlqErr != nil {
							errs <- dlqErr
						} else {
							sharedmetrics.IncKafkaDLQ(b.deadLetter)
						}
					}
				}
			}()
		}
		for _, message := range messages {
			work <- message
		}
		close(work)
		wg.Wait()
		close(errs)
		for err := range errs {
			return fmt.Errorf("projection batch failed topic=%s: %w", cfg.Subject, err)
		}
		if err := r.CommitMessages(ctx, messages...); err != nil {
			return fmt.Errorf("commit projection batch topic=%s: %w", cfg.Subject, err)
		}
	}
}

// runExplicitConsumerGroup keeps Kafka group coordination separate from the
// partition readers. This avoids the kafka-go group Reader fetch path that is
// incompatible with the local broker's advertised listener setup.
func (b *broker) runExplicitConsumerGroup(ctx context.Context, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler, groupID string) error {
	cg, err := kafka.NewConsumerGroup(kafka.ConsumerGroupConfig{Brokers: b.brokers, Topics: []string{cfg.Subject}, ID: groupID, StartOffset: kafka.FirstOffset})
	if err != nil {
		return fmt.Errorf("create Kafka consumer group topic=%s group=%s: %w", cfg.Subject, groupID, err)
	}
	defer cg.Close()
	for {
		generation, nextErr := cg.Next(ctx)
		if nextErr != nil {
			return nextErr
		}
		generation.Start(func(genCtx context.Context) {
			log.Printf("gig recovery generation started topic=%s group=%s assignments=%v", cfg.Subject, groupID, generation.Assignments)
			// More replicas than partitions is valid. Keep idle members in the
			// generation instead of immediately rejoining in a CPU-heavy loop.
			if len(generation.Assignments[cfg.Subject]) == 0 {
				<-genCtx.Done()
				return
			}
			var workers sync.WaitGroup
			for _, assignment := range generation.Assignments[cfg.Subject] {
				log.Printf("gig recovery partition assigned topic=%s partition=%d offset=%d", cfg.Subject, assignment.ID, assignment.Offset)
				workers.Add(1)
				go func(assignment kafka.PartitionAssignment) {
					defer workers.Done()
					reader := kafka.NewReader(kafka.ReaderConfig{Brokers: b.brokers, Topic: cfg.Subject, Partition: assignment.ID, MinBytes: 1, MaxBytes: 16 << 20, MaxWait: 50 * time.Millisecond})
					if assignment.Offset >= 0 {
						_ = reader.SetOffset(assignment.Offset)
					}
					for genCtx.Err() == nil {
						if err := b.runRecoveryBatch(genCtx, reader, cfg, handler, func(offset int64) error {
							return generation.CommitOffsets(map[string]map[int]int64{cfg.Subject: {assignment.ID: offset}})
						}); err != nil {
							if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
								log.Printf("gig recovery batch failed topic=%s partition=%d: %v", cfg.Subject, assignment.ID, err)
							}
							break
						}
					}
					/*
						The legacy single-message loop below is retained temporarily as
						an unreachable reference while the batch path is rolled out.
					*/
					for false && genCtx.Err() == nil {
						message, fetchErr := reader.FetchMessage(genCtx)
						if fetchErr != nil {
							log.Printf("gig recovery partition fetch failed topic=%s partition=%d: %v", cfg.Subject, assignment.ID, fetchErr)
							_ = reader.Close()
							return
						}
						log.Printf("gig recovery message fetched topic=%s partition=%d offset=%d", cfg.Subject, message.Partition, message.Offset)
						attempts := 0
						handleErr := resilience.Retry(genCtx, resilience.RetryPolicy{MaxAttempts: 1}, func(attemptCtx context.Context, attempt int) error {
							attempts = attempt
							messageCtx, cancelMessage := context.WithTimeout(attemptCtx, 30*time.Second)
							defer cancelMessage()
							var event idempotency.Event
							if b.db != nil {
								event = idempotency.DecodeOrFingerprint(cfg.Subject, message.Value)
								if envelope, decodeErr := decodeMigrationEnvelope(message.Value); decodeErr == nil && envelope.CommandID != "" {
									event.EventID = envelope.CommandID
								}
								event.EventID = idempotency.ScopedEventID(cfg.Subject, event.EventID)
								claimed, claimErr := idempotency.ClaimDB(messageCtx, b.db, event)
								if claimErr != nil {
									return claimErr
								}
								if !claimed {
									return nil
								}
							}
							// A group rebalance cancels genCtx after the message has been
							// fetched. Recovery handlers must still be able to publish the
							// completion event after their DB transaction, otherwise the
							// command is applied but its durable acknowledgement is lost.
							handlerCtx, cancelHandler := context.WithTimeout(context.WithoutCancel(messageCtx), 30*time.Second)
							payload, _, unwrapErr := commonevents.Unwrap(message.Value)
							if unwrapErr != nil {
								return unwrapErr
							}
							err := handler(kafkaprop.Context(handlerCtx, message.Headers), cfg.Subject, payload)
							// Validation failures cannot be repaired by retrying the same
							// recovery command. Mark them permanent so they go to DLQ and
							// the partition advances to newer, valid commands.
							if isPermanentRecoveryError(err) {
								err = resilience.Permanent(err)
							}
							cancelHandler()
							if err != nil && b.db != nil {
								_ = idempotency.Release(messageCtx, b.db, event.EventID)
							}
							return err
						})
						if handleErr != nil {
							log.Printf("gig recovery handler failed topic=%s partition=%d offset=%d attempts=%d: %v", cfg.Subject, message.Partition, message.Offset, attempts, handleErr)
							payload, marshalErr := resilience.MarshalDLQ(resilience.DLQRecord{
								OriginalKey: message.Key, OriginalValue: message.Value, OriginalTopic: message.Topic,
								OriginalPartition: message.Partition, OriginalOffset: message.Offset,
								Attempts: attempts, ErrorClass: fmt.Sprintf("%T", handleErr), Error: handleErr.Error(), FailedAt: time.Now().UTC(),
							})
							if marshalErr == nil {
								writer := &kafka.Writer{Addr: kafka.TCP(b.brokers...), Topic: b.deadLetter, Balancer: &kafka.Hash{}, WriteTimeout: 5 * time.Second}
								// A rebalance may cancel genCtx after the handler has
								// failed. DLQ publication is the durable failure record
								// and must survive that generation cancellation.
								writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
								dlqErr := writer.WriteMessages(writeCtx, kafka.Message{Key: message.Key, Value: payload, Headers: message.Headers})
								cancel()
								_ = writer.Close()
								if dlqErr != nil {
									log.Printf("gig recovery DLQ publish failed topic=%s command_offset=%d: %v", b.deadLetter, message.Offset, dlqErr)
								} else {
									sharedmetrics.IncKafkaDLQ(b.deadLetter)
								}
							}
						}
						if commitErr := generation.CommitOffsets(map[string]map[int]int64{cfg.Subject: {assignment.ID: message.Offset + 1}}); commitErr != nil {
							_ = reader.Close()
							return
						}
					}
					_ = reader.Close()
				}(assignment)
			}
			workers.Wait()
		})
	}
}

func (b *broker) runRecoveryBatch(ctx context.Context, reader *kafka.Reader, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler, commit func(int64) error) error {
	batch := make([]kafka.Message, 0, 200)
	message, err := reader.FetchMessage(ctx)
	if err != nil {
		return err
	}
	batch = append(batch, message)
	fetchCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	for len(batch) < cap(batch) {
		next, fetchErr := reader.FetchMessage(fetchCtx)
		if fetchErr != nil {
			break
		}
		batch = append(batch, next)
	}
	cancel()

	lanes := make([]chan kafka.Message, 16)
	results := make(chan error, len(batch))
	var workers sync.WaitGroup
	for i := range lanes {
		lanes[i] = make(chan kafka.Message, len(batch)/len(lanes)+1)
		workers.Add(1)
		go func(messages <-chan kafka.Message) {
			defer workers.Done()
			for item := range messages {
				if err := b.handleRecoveryMessage(ctx, cfg, handler, item); err != nil {
					results <- err
				}
			}
		}(lanes[i])
	}
	for _, item := range batch {
		h := fnv.New32a()
		_, _ = h.Write(recoveryLaneKey(item))
		lanes[int(h.Sum32()%uint32(len(lanes)))] <- item
	}
	for _, lane := range lanes {
		close(lane)
	}
	workers.Wait()
	close(results)
	for err := range results {
		return fmt.Errorf("recovery batch was not committed: %w", err)
	}
	return commit(batch[len(batch)-1].Offset + 1)
}

func recoveryLaneKey(message kafka.Message) []byte {
	envelope, err := decodeMigrationEnvelope(message.Value)
	if err != nil {
		return message.Key
	}
	path := strings.TrimSpace(envelope.CommandPath)
	if strings.HasSuffix(path, "/gigs/drafts") && strings.TrimSpace(envelope.CommandID) != "" {
		return []byte(envelope.CommandID)
	}
	if marker := strings.Index(path, "/gigs/"); marker >= 0 {
		value := path[marker+len("/gigs/"):]
		if slash := strings.IndexByte(value, '/'); slash >= 0 {
			value = value[:slash]
		}
		if value != "" && value != "drafts" {
			return []byte(value)
		}
	}
	if strings.TrimSpace(envelope.AggregateID) != "" {
		return []byte(envelope.AggregateID)
	}
	return message.Key
}

func (b *broker) handleRecoveryMessage(ctx context.Context, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler, message kafka.Message) error {
	envelope, envelopeErr := decodeMigrationEnvelope(message.Value)
	commandID := ""
	if envelopeErr == nil {
		commandID = envelope.CommandID
	}
	log.Printf("gig recovery message received topic=%s partition=%d offset=%d command_id=%s event_type=%s command_path=%s", cfg.Subject, message.Partition, message.Offset, commandID, envelope.EventType, envelope.CommandPath)

	attempts := 0
	handleErr := resilience.Retry(ctx, resilience.RetryPolicy{MaxAttempts: 1}, func(attemptCtx context.Context, attempt int) error {
		attempts = attempt
		messageCtx, cancelMessage := context.WithTimeout(attemptCtx, 30*time.Second)
		defer cancelMessage()
		var event idempotency.Event
		if b.db != nil {
			event = idempotency.DecodeOrFingerprint(cfg.Subject, message.Value)
			if envelope, decodeErr := decodeMigrationEnvelope(message.Value); decodeErr == nil && envelope.CommandID != "" {
				event.EventID = envelope.CommandID
			}
			event.EventID = idempotency.ScopedEventID(cfg.Subject, event.EventID)
			claimed, claimErr := idempotency.ClaimDB(messageCtx, b.db, event)
			if claimErr != nil {
				return claimErr
			}
			if !claimed {
				return nil
			}
		}
		handlerCtx, cancelHandler := context.WithTimeout(context.WithoutCancel(messageCtx), 30*time.Second)
		// Recovery subscribers need the complete migration envelope. Passing only
		// envelope.Payload drops command_id, aggregate_type, aggregate_id and the
		// command path before the recovery handler can validate or dispatch it.
		// Ordinary Kafka consumers below still receive the unwrapped payload; the
		// recovery contract is intentionally envelope-preserving.
		transportkafka.Consumed(cfg.Subject, message.Partition, message.Offset, attempt, message.Value)
		err := handler(kafkaprop.Context(handlerCtx, message.Headers), cfg.Subject, message.Value)
		if isPermanentRecoveryError(err) {
			err = resilience.Permanent(err)
		}
		cancelHandler()
		if err != nil && b.db != nil {
			_ = idempotency.Release(messageCtx, b.db, event.EventID)
		}
		return err
	})
	if handleErr == nil {
		log.Printf("gig recovery message processed topic=%s partition=%d offset=%d command_id=%s attempts=%d", cfg.Subject, message.Partition, message.Offset, commandID, attempts)
		return nil
	}
	log.Printf("gig recovery handler failed topic=%s partition=%d offset=%d attempts=%d: %v", cfg.Subject, message.Partition, message.Offset, attempts, handleErr)
	payload, marshalErr := resilience.MarshalDLQ(resilience.DLQRecord{OriginalKey: message.Key, OriginalValue: message.Value, OriginalTopic: message.Topic, OriginalPartition: message.Partition, OriginalOffset: message.Offset, Attempts: attempts, ErrorClass: fmt.Sprintf("%T", handleErr), Error: handleErr.Error(), FailedAt: time.Now().UTC()})
	if marshalErr != nil {
		return fmt.Errorf("marshal recovery DLQ record topic=%s partition=%d offset=%d command_id=%s: %w", cfg.Subject, message.Partition, message.Offset, commandID, marshalErr)
	}
	writer := &kafka.Writer{Addr: kafka.TCP(b.brokers...), Topic: b.deadLetter, Balancer: &kafka.Hash{}, WriteTimeout: 5 * time.Second}
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	dlqErr := writer.WriteMessages(writeCtx, kafka.Message{Key: message.Key, Value: payload, Headers: message.Headers})
	cancel()
	_ = writer.Close()
	if dlqErr != nil {
		log.Printf("gig recovery DLQ publish failed topic=%s command_offset=%d: %v", b.deadLetter, message.Offset, dlqErr)
		return fmt.Errorf("publish recovery DLQ topic=%s partition=%d offset=%d command_id=%s: %w", b.deadLetter, message.Partition, message.Offset, commandID, dlqErr)
	}
	sharedmetrics.IncKafkaDLQ(b.deadLetter)
	log.Printf("gig recovery message dead-lettered source_topic=%s partition=%d offset=%d command_id=%s dlq_topic=%s attempts=%d", cfg.Subject, message.Partition, message.Offset, commandID, b.deadLetter, attempts)
	return nil
}

func isPermanentRecoveryError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, domain.ErrInvalidGigID) ||
		errors.Is(err, domain.ErrInvalidFreelancerID) ||
		errors.Is(err, domain.ErrInvalidPackageID) ||
		errors.Is(err, domain.ErrInvalidQuestionID) ||
		errors.Is(err, domain.ErrInvalidUsername) ||
		errors.Is(err, domain.ErrInvalidTitle) ||
		errors.Is(err, domain.ErrInvalidShortInfo) ||
		errors.Is(err, domain.ErrInvalidDescription) ||
		errors.Is(err, domain.ErrInvalidCategoryID) ||
		errors.Is(err, domain.ErrInvalidCurrency) ||
		errors.Is(err, domain.ErrInvalidPackageCount) ||
		errors.Is(err, domain.ErrInvalidPackageTier) ||
		errors.Is(err, domain.ErrInvalidPackageDescription) ||
		errors.Is(err, domain.ErrInvalidPackageDeliveryDays) ||
		errors.Is(err, domain.ErrInvalidPackagePriceCents) ||
		errors.Is(err, domain.ErrInvalidQuestionContent) ||
		errors.Is(err, domain.ErrInvalidFileID)
}

func (b *broker) ensureTopic(ctx context.Context, topic string) error {
	if strings.TrimSpace(topic) == "" {
		return errors.New("kafka topic is empty")
	}
	conn, err := kafka.DialContext(ctx, "tcp", b.brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ReadPartitions(topic); err == nil {
		return nil
	}
	return conn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 8, ReplicationFactor: 1})
}

func (b *broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.readers {
		_ = r.Close()
	}
	for topic, w := range b.writers {
		if err := w.Close(); err != nil {
			log.Printf("close kafka writer topic=%s: %v", topic, err)
		}
	}
	b.writers = nil
}
