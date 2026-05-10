package db

import (
	"errors"
	"time"

	"gig-service/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fakeMigrator struct{ upErr error }

func (f fakeMigrator) Up() error { return f.upErr }

var _ = Describe("yugabyte bootstrap", func() {
	It("opens the db and applies settings", func() {
		raw, mock, err := sqlmock.New()
		Expect(err).NotTo(HaveOccurred())
		defer raw.Close()

		original := connectDB
		connectDB = func(driverName, dsn string) (*sqlx.DB, error) {
			Expect(driverName).To(Equal("pgx"))
			Expect(dsn).To(ContainSubstring("postgres://user:pass@db:5433/gig_service"))
			return sqlx.NewDb(raw, "sqlmock"), nil
		}
		DeferCleanup(func() { connectDB = original })

		db, err := Open(config.DBConfig{
			Host:            "db",
			Port:            5433,
			User:            "user",
			Password:        "pass",
			Name:            "gig_service",
			SSLMode:         "disable",
			MaxOpenConns:    9,
			MaxIdleConns:    7,
			ConnMaxLifetime: 3 * time.Minute,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(db).NotTo(BeNil())
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("wraps open db failures", func() {
		original := connectDB
		connectDB = func(string, string) (*sqlx.DB, error) { return nil, errors.New("dial failed") }
		DeferCleanup(func() { connectDB = original })

		db, err := Open(config.DBConfig{})
		Expect(db).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("open db")))
	})

	It("runs migrations and normalizes relative paths", func() {
		original := newMigrator
		newMigrator = func(sourceURL, databaseURL string) (migrator, error) {
			Expect(sourceURL).To(HavePrefix("file://"))
			Expect(databaseURL).To(ContainSubstring("yugabytedb://user:pass@db:5433/gig_service"))
			return fakeMigrator{}, nil
		}
		DeferCleanup(func() { newMigrator = original })

		Expect(RunMigrations(config.DBConfig{
			Host:            "db",
			Port:            5433,
			User:            "user",
			Password:        "pass",
			Name:            "gig_service",
			SSLMode:         "disable",
			MigrationsPath:   "file://migration/yugabyte",
			MigrationsTable:  "schema_migrations_gig_service",
		})).To(Succeed())
	})

	It("handles migration edge cases", func() {
		original := newMigrator
		newMigrator = func(string, string) (migrator, error) { return nil, errors.New("create failed") }
		DeferCleanup(func() { newMigrator = original })
		Expect(RunMigrations(config.DBConfig{})).To(MatchError(ContainSubstring("create migrator")))

		newMigrator = func(string, string) (migrator, error) { return fakeMigrator{upErr: migrate.ErrNoChange}, nil }
		Expect(RunMigrations(config.DBConfig{})).To(Succeed())

		newMigrator = func(string, string) (migrator, error) { return fakeMigrator{upErr: errors.New("boom")}, nil }
		Expect(RunMigrations(config.DBConfig{})).To(MatchError(ContainSubstring("run migrations")))
	})

	It("executes work inside a sql transaction", func() {
		raw, mock, err := sqlmock.New()
		Expect(err).NotTo(HaveOccurred())
		defer raw.Close()
		db := sqlx.NewDb(raw, "sqlmock")

		mock.ExpectBegin()
		mock.ExpectCommit()

		Expect(WithTx(func(tx *sqlx.Tx) error {
			Expect(tx).NotTo(BeNil())
			return nil
		}, db)).To(Succeed())

		mock.ExpectBegin()
		mock.ExpectRollback()
		Expect(WithTx(func(tx *sqlx.Tx) error {
			Expect(tx).NotTo(BeNil())
			return errors.New("boom")
		}, db)).To(MatchError("boom"))
	})

	It("wraps bootstrap errors", func() {
		Expect(WrapResolveMigrationsPathError(errors.New("boom"))).To(MatchError(ContainSubstring("resolve migrations path")))
		Expect(WrapCreateMigratorError(errors.New("boom"))).To(MatchError(ContainSubstring("create migrator")))
		Expect(WrapRunMigrationsError(errors.New("boom"))).To(MatchError(ContainSubstring("run migrations")))
		Expect(WrapOpenDBError(errors.New("boom"))).To(MatchError(ContainSubstring("open db")))
		Expect(WrapPingDBError(errors.New("boom"))).To(MatchError(ContainSubstring("ping db")))
	})
})
