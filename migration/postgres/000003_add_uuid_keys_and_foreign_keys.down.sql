ALTER TABLE gig_packages
    DROP CONSTRAINT IF EXISTS fk_gig_packages_gig_id;

ALTER TABLE gig_questions
    DROP CONSTRAINT IF EXISTS fk_gig_questions_gig_id;

ALTER TABLE gig_media
    DROP CONSTRAINT IF EXISTS fk_gig_media_gig_id;

ALTER TABLE gigs
    ALTER COLUMN freelancer_id TYPE TEXT USING freelancer_id::text,
    ALTER COLUMN gig_id TYPE TEXT USING gig_id::text;

ALTER TABLE gig_packages
    ALTER COLUMN gig_id TYPE TEXT USING gig_id::text,
    ALTER COLUMN package_id TYPE TEXT USING package_id::text;

ALTER TABLE gig_questions
    ALTER COLUMN gig_id TYPE TEXT USING gig_id::text,
    ALTER COLUMN question_id TYPE TEXT USING question_id::text;

ALTER TABLE gig_media
    ALTER COLUMN gig_id TYPE TEXT USING gig_id::text,
    ALTER COLUMN media_id TYPE TEXT USING media_id::text;
