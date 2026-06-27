package user

// UserCreatedPayload is the JSON payload carried in the Envelope for a
// UserCreated event.
type UserCreatedPayload struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
