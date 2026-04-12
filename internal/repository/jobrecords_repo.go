package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/Yash840/runrq/internal/model"
)

type JobRecordsRepo struct {
	db *sql.DB
}

func (repo *JobRecordsRepo) Create(record model.JobRecord) error {
	query := "INSERT INTO job_records(id, status, result, error, created_at, completed_at) VALUES ($1, $2, $3, $4, $5, $6);"

	_, err := repo.db.Exec(query, record.ID, record.Status, record.Result, record.Error, record.CreatedAt, record.CompletedAt)
	if err != nil {
		return fmt.Errorf("error occured in creating record: %w", err)
	} 

	return nil
}

func (repo *JobRecordsRepo) GetAll() ([]model.JobRecord, error) {
	query := "SELECT id, status, result, error, created_at, completed_at FROM job_records;"

	rows, err := repo.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error occured in fetching records: %w", err)
	}

	defer rows.Close()

	var records []model.JobRecord

	for rows.Next() {
		var record model.JobRecord

		err := rows.Scan(&record.ID, &record.Status, &record.Result, &record.Error, &record.CreatedAt, &record.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("error occured in fetching records: %w", err)
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
    	return nil, fmt.Errorf("error iterating over records: %w", err)
	}

	return records, nil
}

func (repo *JobRecordsRepo) Get(id string) (*model.JobRecord, error) {
	query := "SELECT id, status, result, error, created_at, completed_at FROM job_records WHERE id=$1;"

	var record model.JobRecord
	
	err := repo.db.QueryRow(query, id).Scan(&record.ID, &record.Status, &record.Result, &record.Error, &record.CreatedAt, &record.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("error occured in fetching record: %w", err)
	}

	return &record, nil
}

func (repo *JobRecordsRepo) Update(id string, opts model.JobRecordUpdateOpts) error {
	sets := make([]string, 0, 4)
	args := make([]any, 0, 5)
	argPos := 1

	 if opts.Status != nil {
        sets = append(sets, fmt.Sprintf("status = $%d", argPos))
        args = append(args, *opts.Status)
        argPos++
    }
    if opts.Error != nil {
        sets = append(sets, fmt.Sprintf("error = $%d", argPos))
        args = append(args, *opts.Error)
        argPos++
    }
    if opts.Result != nil {
        sets = append(sets, fmt.Sprintf("result = $%d", argPos))
        args = append(args, opts.Result)
        argPos++
    }
    if opts.CompletedAt != nil {
        sets = append(sets, fmt.Sprintf("completed_at = $%d", argPos))
        args = append(args, *opts.CompletedAt)
        argPos++
    }

	if len(sets) == 0 {
		return fmt.Errorf("error occured in updating record: no values to update")
	}

	query := fmt.Sprintf(
		"UPDATE job_records SET %s WHERE id= $%d;",
		strings.Join(sets, ", "),
		argPos,
	)

	args = append(args, id)

	_, err := repo.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("error occurred in updating record: %w", err)
	}

	return nil
}

func (repo *JobRecordsRepo) Delete(id string) error {
	query := "DELETE FROM job_records WHERE id= $1;"

	_, err := repo.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error occurred in deleting record: %w", err)
	}

	return nil
}