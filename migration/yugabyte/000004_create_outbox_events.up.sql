CREATE TABLE IF NOT EXISTS outbox_events (
    event_id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    operation TEXT NOT NULL,
    schema_version INT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate
    ON outbox_events (aggregate_type, aggregate_id, created_at);

CREATE OR REPLACE FUNCTION capture_gig_outbox_event() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE row_data JSONB; operation_value TEXT := CASE TG_OP WHEN 'INSERT' THEN 'created' WHEN 'DELETE' THEN 'deactivated' ELSE 'updated' END; aggregate_type_value TEXT;
BEGIN
    row_data := CASE WHEN TG_OP = 'DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END;
    aggregate_type_value := CASE WHEN TG_TABLE_NAME = 'gig_packages' THEN 'gig_packages' WHEN TG_TABLE_NAME = 'gig_questions' THEN 'gig_questions' WHEN TG_TABLE_NAME = 'gig_media' THEN 'gig_media' ELSE 'gigs' END;
    INSERT INTO outbox_events(event_id, aggregate_type, aggregate_id, event_type, operation, payload)
    VALUES (md5(clock_timestamp()::TEXT || random()::TEXT)::UUID, aggregate_type_value, COALESCE(row_data->>'gig_id', row_data->>'package_id', row_data->>'question_id', row_data->>'media_id', row_data->>'id'), 'gig-service.' || aggregate_type_value || '.changed', operation_value, row_data);
    IF TG_OP = 'DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$$;

DROP TRIGGER IF EXISTS gigs_outbox ON gigs;
CREATE TRIGGER gigs_outbox AFTER INSERT OR UPDATE OR DELETE ON gigs FOR EACH ROW EXECUTE FUNCTION capture_gig_outbox_event();
DROP TRIGGER IF EXISTS gig_packages_outbox ON gig_packages;
CREATE TRIGGER gig_packages_outbox AFTER INSERT OR UPDATE OR DELETE ON gig_packages FOR EACH ROW EXECUTE FUNCTION capture_gig_outbox_event();
DROP TRIGGER IF EXISTS gig_questions_outbox ON gig_questions;
CREATE TRIGGER gig_questions_outbox AFTER INSERT OR UPDATE OR DELETE ON gig_questions FOR EACH ROW EXECUTE FUNCTION capture_gig_outbox_event();
DROP TRIGGER IF EXISTS gig_media_outbox ON gig_media;
CREATE TRIGGER gig_media_outbox AFTER INSERT OR UPDATE OR DELETE ON gig_media FOR EACH ROW EXECUTE FUNCTION capture_gig_outbox_event();
