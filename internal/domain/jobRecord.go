package domain

import "time"

type JobRecord struct {
	ID 		    string		`json:"id"`
	Status 	    string 		`json:"status"`//["Pending", "Processing", "Completed", "Failed"]
	Result 	    any			`json:"result"`
	Error       string 		`json:"error"`
	CreatedAt   time.Time 	`json:"created-at"`
	CompletedAt time.Time 	`json:"completed-at"`
}