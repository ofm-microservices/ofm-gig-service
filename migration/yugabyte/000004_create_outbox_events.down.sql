DROP TRIGGER IF EXISTS gigs_outbox ON gigs;
DROP TRIGGER IF EXISTS gig_packages_outbox ON gig_packages;
DROP TRIGGER IF EXISTS gig_questions_outbox ON gig_questions;
DROP TRIGGER IF EXISTS gig_media_outbox ON gig_media;
DROP FUNCTION IF EXISTS capture_gig_outbox_event();
DROP TABLE IF EXISTS outbox_events;
