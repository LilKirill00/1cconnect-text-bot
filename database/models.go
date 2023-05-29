package database

type (
	Chat struct {
		PreviousState string `json:"prev_state" binding:"required" example:"100"`
		CurrentState  string `json:"curr_state" binding:"required" example:"300"`
	}
)

const (
	GREETINGS = "greetings"
	START     = "start"
	FINAL     = "final_menu"
	FAIL_QNA  = "fail_qna_menu"
)
