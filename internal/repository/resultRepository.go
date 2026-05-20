package repository

import (
	"database/sql"
	"time"

	"github.com/Yash840/runrq/internal/shared"
)

type ResultRepository struct {
	db *sql.DB
}

func NewResultRepository(db *sql.DB) *ResultRepository {
	return &ResultRepository{db: db}
}

func (r ResultRepository) Create(jobID string, body any) error {
	query := `
		INSERT INTO results(job_id, body) 
		VALUES ($1, $2)                                                           	
	`

	return r.db.QueryRow(
		query,
		jobID,
		body,
	).Err()
}

func (r ResultRepository) Remove(resID string) error {
	query := `DELETE FROM results WHERE result_id=$1`

	_, err := r.db.Exec(query, resID)

	return err
}

func (r ResultRepository) GetForJobID(jobID string) ([]shared.Result, error) {
	query := `
		SELECT result_id, job_id, finish_time, body FROM results 
		WHERE job_id=$1
		ORDER BY finish_time DESC
	`

	rows, err := r.db.Query(query, jobID)
	if err != nil {
		return nil, err
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	var results []shared.Result

	for rows.Next() {
		var res shared.Result

		err := rows.Scan(
			&res.ResultID,
			&res.JobID,
			&res.FinishTime,
			&res.Body,
		)

		if err != nil {
			return nil, err
		}

		results = append(results, res)
	}

	return results, nil
}

func (r ResultRepository) Sweep(durationHrs int) error {
	query := `
		DELETE FROM results WHERE EXTRACT(EPOCH FROM ($1 - finish_time)) / 3600 > $2
	`

	now := time.Now()

	_, err := r.db.Exec(query, now, durationHrs)
	if err != nil {
		return err
	}

	return nil
}
