package repository

// DBErrorTranslator maps low-level Yugabyte errors into domain-aware
// repository errors.
type DBErrorTranslator interface {
	TranslateCreateGigError(err error) error
	TranslateFindGigError(err error) error
	TranslatePublishGigError(err error) error
}
