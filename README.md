# OFM Gig Service

## Purpose

The Gig Service owns freelancer gig data: drafts, packages, questions, media references, requirements, publication state, and gig-facing projections. Status: active.

## Boundaries and flow

The service receives gig and onboarding commands, validates ownership, writes PostgreSQL state and outbox records, and publishes changes for projections and dependent flows. Redis is a derived read model where configured. It does not own user profiles, auth credentials, orders, or file binaries.

Important operations include draft creation, basic-info updates, package and question replacement, media association, requirements, publication, and public gig reads. Kafka/CDC projection events are acknowledged only after the destination transaction succeeds.

## Configuration

.env.example groups are DB_*, MIGRATIONS_*, REDIS_*, NATS commands, Kafka/CDC settings, listener ports, and observability. Database values select gig storage; Redis values select the derived read model; broker values select command and event consumers.

## Local development

    cp .env.example .env
    just run
    go test ./...

## Build and operations

Dockerfile builds ofm/gig-service:<tag>. Helm and ofm-infra provide the database, Redis, brokers, and deployment. Use gig traces, outbox/CDC logs, Kafka lag, projection audit, and draft/publish checks for diagnosis.

