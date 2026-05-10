package repository

const (
	createGigDraftQuery = `
		INSERT INTO gigs (
			gig_id, freelancer_id, slug, status, picture_file_id,
			basic_info_completed, packages_completed, requirements_completed, media_completed,
			created_at, updated_at
		)
		VALUES ($1, $2, '', 'draft', '', FALSE, FALSE, FALSE, FALSE, NOW(), NOW())
		RETURNING gig_id, freelancer_id, slug, title, description, category_id, currency, status,
			basic_info_completed, packages_completed, requirements_completed, media_completed, picture_file_id,
			published_at, created_at, updated_at
	`

	updateGigBasicInfoQuery = `
		UPDATE gigs
		SET title = $2,
			slug = $3,
			description = $4,
			category_id = $5,
			currency = $6,
			basic_info_completed = TRUE,
			updated_at = NOW()
		WHERE gig_id = $1
		RETURNING gig_id, freelancer_id, slug, title, description, category_id, currency, status,
			basic_info_completed, packages_completed, requirements_completed, media_completed, picture_file_id,
			published_at, created_at, updated_at
	`

	publishGigQuery = `
		UPDATE gigs
		SET status = 'published',
			published_at = NOW(),
			updated_at = NOW()
		WHERE gig_id = $1
		RETURNING gig_id, freelancer_id, slug, title, description, category_id, currency, status,
			basic_info_completed, packages_completed, requirements_completed, media_completed, picture_file_id,
			published_at, created_at, updated_at
	`

	getGigQuery = `
		SELECT gig_id, freelancer_id, slug, title, description, category_id, currency, status,
			basic_info_completed, packages_completed, requirements_completed, media_completed, picture_file_id,
			published_at, created_at, updated_at
		FROM gigs
		WHERE gig_id = $1
	`

	deleteGigPackagesQuery = `DELETE FROM gig_packages WHERE gig_id = $1`
	insertGigPackageQuery  = `
		INSERT INTO gig_packages (
			package_id, gig_id, tier, description, delivery_days, price_cents, sort_order, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`

	deleteGigQuestionsQuery = `DELETE FROM gig_questions WHERE gig_id = $1`
	insertGigQuestionQuery  = `
		INSERT INTO gig_questions (
			question_id, gig_id, content, sort_order, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`

	deleteGigMediaQuery = `DELETE FROM gig_media WHERE gig_id = $1`
	insertGigMediaQuery = `
		INSERT INTO gig_media (
			media_id, gig_id, file_id, sort_order, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`

	getGigPackagesQuery = `
		SELECT package_id, gig_id, tier, description, delivery_days, price_cents, sort_order, created_at, updated_at
		FROM gig_packages
		WHERE gig_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`

	getGigQuestionsQuery = `
		SELECT question_id, gig_id, content, sort_order, created_at, updated_at
		FROM gig_questions
		WHERE gig_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`

	getGigMediaQuery = `
		SELECT gig_id, file_id, sort_order, created_at, updated_at
		FROM gig_media
		WHERE gig_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`
)
