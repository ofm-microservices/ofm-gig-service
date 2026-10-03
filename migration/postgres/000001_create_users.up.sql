CREATE TABLE IF NOT EXISTS gigs (
    gig_id TEXT PRIMARY KEY,
    freelancer_id TEXT NOT NULL,
    slug TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    category_id BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    basic_info_completed BOOLEAN NOT NULL DEFAULT FALSE,
    packages_completed BOOLEAN NOT NULL DEFAULT FALSE,
    requirements_completed BOOLEAN NOT NULL DEFAULT FALSE,
    media_completed BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gig_packages (
    package_id TEXT PRIMARY KEY,
    gig_id TEXT NOT NULL,
    tier TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    delivery_days INT NOT NULL DEFAULT 0,
    price_cents BIGINT NOT NULL DEFAULT 0,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gig_questions (
    question_id TEXT PRIMARY KEY,
    gig_id TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gig_media (
    media_id TEXT PRIMARY KEY,
    gig_id TEXT NOT NULL,
    file_id TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gigs_freelancer_id ON gigs(freelancer_id);
CREATE INDEX IF NOT EXISTS idx_gig_packages_gig_id ON gig_packages(gig_id);
CREATE INDEX IF NOT EXISTS idx_gig_questions_gig_id ON gig_questions(gig_id);
CREATE INDEX IF NOT EXISTS idx_gig_media_gig_id ON gig_media(gig_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_gig_packages_gig_id_tier ON gig_packages(gig_id, tier);
CREATE UNIQUE INDEX IF NOT EXISTS ux_gig_questions_gig_id_sort_order ON gig_questions(gig_id, sort_order);
CREATE UNIQUE INDEX IF NOT EXISTS ux_gig_media_gig_id_sort_order ON gig_media(gig_id, sort_order);
ALTER TABLE gig_packages DROP COLUMN IF EXISTS title;

ALTER TABLE gigs ADD COLUMN IF NOT EXISTS picture_file_id TEXT NOT NULL DEFAULT '';
