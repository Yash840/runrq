package domain

type SubmitJobReq struct {
	Type 	 string   `json:"type"`
	Payload  string   `json:"payload"`
}