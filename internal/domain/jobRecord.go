package domain

import "time"

type JobRecord struct {
	ID 		    string		// same as Job ID
	Status 	    string 	//["Pending", "Processing", "Completed", "Failed"]
	Result 	    any
	Error       string
	CreatedAt   time.Time
	CompletedAt time.Time
}