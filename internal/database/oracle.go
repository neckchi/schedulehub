package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/neckchi/schedulehub/internal/schema"
	go_ora "github.com/sijms/go-ora/v2"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type OracleRepository interface {
	QueryContext(ctx context.Context, queryParams schema.QueryParamsForVesselVoyage) ([]schema.ScheduleRow, error)
}

// Settings represents application configuration
type OracleSettings struct {
	DBUser      *string
	DBPassword  *string
	Host        *string
	Port        *int
	ServiceName *string
}

// OracleDBConnectionPool implements the OracleRepository interface
type OracleDBConnectionPool struct {
	db          *sql.DB
	stmt        *sql.Stmt
	concurrency int
	maxRetries  int
}

// NewOracleDBConnectionPool creates a new instance of OracleDBConnectionPool
func NewOracleDBConnectionPool(settings OracleSettings, concurrency, maxRetries int) (*OracleDBConnectionPool, error) {
	//instead of fetching rows one by one, we fetch multiple rows in one network operation
	urlOptions := map[string]string{
		"PREFETCH_ROWS": "500",
	}
	connStr := go_ora.BuildUrl(*settings.Host, *settings.Port, *settings.ServiceName, *settings.DBUser, *settings.DBPassword, urlOptions)
	var db *sql.DB
	var err error

	// Retry mechanism for opening the database connection
	for retry := 0; retry <= maxRetries; retry++ {
		db, err = sql.Open("oracle", connStr)
		if err == nil {
			break
		}
		log.Errorf("attempt %d: error opening database connection: %v", retry+1, err)
		if retry < maxRetries {
			time.Sleep(time.Second * time.Duration(retry+1)) // Exponential backoff
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection after %d retries: %w", maxRetries, err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(concurrency)
	db.SetMaxIdleConns(100)
	db.SetConnMaxIdleTime(10 * time.Minute)
	db.SetConnMaxLifetime(20 * time.Minute)

	pool := &OracleDBConnectionPool{
		db:          db,
		concurrency: concurrency,
		maxRetries:  maxRetries,
	}
	// Read SQL query once and prepare it
	queryString, err := pool.getSQLquery()
	if err != nil {
		db.Close()
		return nil, err
	}
	//stmt will be bound to a single underlying connection forever. https://pkg.go.dev/database/sql#Stmt
	stmt, err := db.PrepareContext(context.Background(), string(queryString))
	if err != nil {
		db.Close()
		return nil, err
	}
	pool.stmt = stmt

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for retry := 0; retry <= maxRetries; retry++ {
		err = pool.db.PingContext(ctx)
		if err == nil {
			log.Info("Connected To Oracle DB connection pool")
			break
		}
		log.Errorf("attempt %d: failed to connect to Oracle DB: %v", retry+1, err)
		if retry < maxRetries {
			time.Sleep(time.Second * time.Duration(retry+2))
		}
	}
	if err != nil {
		pool.db.Close()
		return nil, fmt.Errorf("failed to connect to Oracle DB after %d retries: %w", maxRetries, err)
	}
	return pool, nil
}

func (p *OracleDBConnectionPool) getSQLquery() ([]byte, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sqlFilePath := filepath.Join(currentDir+"/internal/handlers", "master_voyage.sql")
	queryString, err := os.ReadFile(sqlFilePath)
	if err != nil {
		return nil, err
	}
	return queryString, nil
}

// QueryContext executes a query that returns rows
func (p *OracleDBConnectionPool) QueryContext(ctx context.Context, queryParams schema.QueryParamsForVesselVoyage) ([]schema.ScheduleRow, error) {
	log.Info("Started requesting vessel voyages from database")
	rows, err := p.stmt.QueryContext(ctx, sql.Named("scac", string(*queryParams.SCAC)),
		sql.Named("imo", *queryParams.VesselIMO), sql.Named("voyage", *queryParams.Voyage))
	if err != nil {
		return nil, err
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
			&sr.Rank,
		)
		if err != nil {
			log.Errorf("row scan error: %v", err)
		}
		scheduleRows = append(scheduleRows, sr)
	}
	//sort desc
	sort.Slice(scheduleRows, func(i, j int) bool {
		return scheduleRows[i].EventTime < scheduleRows[j].EventTime
	})
	return scheduleRows, nil
}
