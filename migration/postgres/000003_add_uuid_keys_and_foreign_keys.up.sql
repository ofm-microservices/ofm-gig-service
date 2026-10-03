ALTER TABLE gig_media
    ALTER COLUMN media_id TYPE UUID USING media_id::uuid,
    ALTER COLUMN gig_id TYPE UUID USING gig_id::uuid;

ALTER TABLE gig_questions
    ALTER COLUMN question_id TYPE UUID USING question_id::uuid,
    ALTER COLUMN gig_id TYPE UUID USING gig_id::uuid;

ALTER TABLE gig_packages
    ALTER COLUMN package_id TYPE UUID USING package_id::uuid,
    ALTER COLUMN gig_id TYPE UUID USING gig_id::uuid;

ALTER TABLE gigs
    ALTER COLUMN gig_id TYPE UUID USING gig_id::uuid,
    ALTER COLUMN freelancer_id TYPE UUID USING freelancer_id::uuid;

DELETE FROM gig_packages p
WHERE NOT EXISTS (
    SELECT 1
    FROM gigs g
    WHERE g.gig_id = p.gig_id
);

DELETE FROM gig_questions q
WHERE NOT EXISTS (
    SELECT 1
    FROM gigs g
    WHERE g.gig_id = q.gig_id
);

DELETE FROM gig_media m
WHERE NOT EXISTS (
    SELECT 1
    FROM gigs g
    WHERE g.gig_id = m.gig_id
);

ALTER TABLE gig_packages
    ADD CONSTRAINT fk_gig_packages_gig_id
        FOREIGN KEY (gig_id) REFERENCES gigs(gig_id) ON DELETE CASCADE;

ALTER TABLE gig_questions
    ADD CONSTRAINT fk_gig_questions_gig_id
        FOREIGN KEY (gig_id) REFERENCES gigs(gig_id) ON DELETE CASCADE;

ALTER TABLE gig_media
    ADD CONSTRAINT fk_gig_media_gig_id
        FOREIGN KEY (gig_id) REFERENCES gigs(gig_id) ON DELETE CASCADE;
