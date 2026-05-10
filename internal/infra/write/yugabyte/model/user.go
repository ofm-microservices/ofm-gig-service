package model

import "time"

// GigRow is the Yugabyte persistence model for the gig write model.
type GigRow struct {
	ID                    string     `db:"gig_id"`
	FreelancerID          string     `db:"freelancer_id"`
	Slug                  string     `db:"slug"`
	Title                 string     `db:"title"`
	Description           string     `db:"description"`
	CategoryID            int64      `db:"category_id"`
	Currency              string     `db:"currency"`
	Status                string     `db:"status"`
	BasicInfoCompleted    bool       `db:"basic_info_completed"`
	PackagesCompleted     bool       `db:"packages_completed"`
	RequirementsCompleted bool       `db:"requirements_completed"`
	MediaCompleted        bool       `db:"media_completed"`
	PictureFileID         string     `db:"picture_file_id"`
	PublishedAt           *time.Time `db:"published_at"`
	CreatedAt             time.Time  `db:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at"`
}

// GigPackageRow is the Yugabyte persistence model for gig packages.
type GigPackageRow struct {
	ID           string    `db:"package_id"`
	GigID        string    `db:"gig_id"`
	Tier         string    `db:"tier"`
	Description  string    `db:"description"`
	DeliveryDays int32     `db:"delivery_days"`
	PriceCents   int64     `db:"price_cents"`
	SortOrder    int32     `db:"sort_order"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// GigQuestionRow is the Yugabyte persistence model for gig questions.
type GigQuestionRow struct {
	ID        string    `db:"question_id"`
	GigID     string    `db:"gig_id"`
	Content   string    `db:"content"`
	SortOrder int32     `db:"sort_order"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// GigMediaRow is the Yugabyte persistence model for gig gallery file IDs.
type GigMediaRow struct {
	ID        string    `db:"media_id"`
	GigID     string    `db:"gig_id"`
	FileID    string    `db:"file_id"`
	SortOrder int32     `db:"sort_order"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
