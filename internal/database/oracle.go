package database

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"github.com/neckchi/schedulehub/internal/schema"
	go_ora "github.com/sijms/go-ora/v2"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type OracleRepository interface {
	QueryContext(ctx context.Context, queryParams schema.QueryParamsForVesselVoyage) ([]schema.ScheduleRow, error)
	Close() error
}

type OracleSettings struct {
	DBUser      *string
	DBPassword  *string
	Host        *string
	Port        *int
	ServiceName *string
}

type OracleDBConnectionPool struct {
	db          *sql.DB
	stmt        *sql.Stmt // Prepared statement
	concurrency int
	maxRetries  int
}

func openConnectionWithRetry(connStr string, maxRetries int) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for retry := 0; retry <= maxRetries; retry++ {
		db, err = sql.Open("oracle", connStr)
		if err == nil {
			return db, nil
		}
		log.Errorf("attempt %d: error creating database connection pool: %v", retry+1, err)
		time.Sleep(time.Second * time.Duration(retry+1))
	}
	return nil, fmt.Errorf("failed to open database connection after %d retries", maxRetries)
}

func getSQLquery() ([]byte, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sqlFilePath := filepath.Join(currentDir+"/internal/handlers", "master_voyage.sql")
	return os.ReadFile(sqlFilePath)
}

func NewOracleDBConnectionPool(settings OracleSettings, concurrency, maxRetries int) (*OracleDBConnectionPool, error) {
	if settings.DBUser == nil || settings.DBPassword == nil || settings.Host == nil ||
		settings.Port == nil || settings.ServiceName == nil {
		return nil, fmt.Errorf("all OracleSettings fields must be specified")
	}

	urlOptions := map[string]string{"PREFETCH_ROWS": "500"}
	connStr := go_ora.BuildUrl(*settings.Host, *settings.Port, *settings.ServiceName, *settings.DBUser, *settings.DBPassword, urlOptions)

	db, err := openConnectionWithRetry(connStr, maxRetries)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(concurrency)
	db.SetMaxIdleConns(100)
	db.SetConnMaxIdleTime(60 * time.Second)
	db.SetConnMaxLifetime(60 * time.Second)

	pool := &OracleDBConnectionPool{
		db:          db,
		concurrency: concurrency,
		maxRetries:  maxRetries,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping Oracle DB: %w", err)
	}

	queryString, err := getSQLquery()
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to get SQL script: %w", err)
	}

	stmt, err := db.PrepareContext(context.Background(), string(queryString))
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	pool.stmt = stmt

	return pool, nil
}

func (p *OracleDBConnectionPool) Close() error {
	var closeErr error
	if p.stmt != nil {
		closeErr = p.stmt.Close()
	}
	dbErr := p.db.Close()
	if closeErr != nil {
		return closeErr
	}
	return dbErr
}

func (p *OracleDBConnectionPool) QueryContext(ctx context.Context, queryParams schema.QueryParamsForVesselVoyage) ([]schema.ScheduleRow, error) {
	startTime := time.Now()
	log.Info("Started requesting vessel voyages from database")

	rows, err := p.stmt.QueryContext(ctx,
		sql.Named("scac", string(queryParams.SCAC)),
		sql.Named("imo", queryParams.VesselIMO),
		sql.Named("voyage", queryParams.Voyage),
		sql.Named("startDate", queryParams.StartDate),
		sql.Named("dateRange", queryParams.DateRange),
	)
	if err != nil {
		if ctx.Err() != nil {
			log.Errorf("QueryContext: query canceled due to context: %v", ctx.Err())
			return nil, fmt.Errorf("query canceled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Errorf("error closing rows: %v", closeErr)
		}
	}()

	var scheduleRows []schema.ScheduleRow
	for rows.Next() {
		var sr schema.ScheduleRow
		err := rows.Scan(
			&sr.DataSource,
			&sr.SCAC,
			&sr.ProvideVoyageID,
			&sr.VesselName,
			&sr.VesselIMO,
			&sr.VoyageNum,
			&sr.VoyageDirection,
			&sr.ServiceCode,
			&sr.PortCode,
			&sr.PortName,
			&sr.PortEvent,
			&sr.EventTime,
			//&sr.Rank,
		)
		if err != nil {
			log.Errorf("row scan error: %v", err)
		}
		scheduleRows = append(scheduleRows, sr)
	}

	slices.SortFunc(scheduleRows, func(a, b schema.ScheduleRow) int {
		return cmp.Or(
			cmp.Compare(a.VoyageNum, b.VoyageNum),
			cmp.Compare(a.EventTime, b.EventTime),
			cmp.Compare(a.PortCode, b.PortCode),
		)
	})
	log.Infof("Fetched vessel voyages from database %.3fs", time.Since(startTime).Seconds())
	return scheduleRows, nil
}
