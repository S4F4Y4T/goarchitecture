package response

import (
	"encoding/json"
	"log"
	"net/http"

	"microservice/pkg/appError"
)

type ErrorBody struct {
	Code    appError.Code         `json:"code"`
	Message string                `json:"message"`
	Fields  []appError.FieldError `json:"fields,omitempty"`
}

type ApiResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message,omitempty"`
	Data    any        `json:"data,omitempty"`
	Error   *ErrorBody `json:"error,omitempty"`
}

func JSONResponse(w http.ResponseWriter, statusCode int, payload ApiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("response: encode failed: %v", err)
	}
}

func Success(w http.ResponseWriter, status int, message string, data any) {
	JSONResponse(w, status, ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Error normalizes any error into an *appError.AppError and writes a
// consistent error response. Internal errors are logged server-side; the
// client only sees a generic message.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	appErr := appError.From(err)

	if appErr.Code == appError.CodeInternal && appErr.Err != nil {
		log.Printf("internal error %s %s: %v", r.Method, r.URL.Path, appErr.Err)
	}

	JSONResponse(w, appErr.HTTPStatus(), ApiResponse{
		Success: false,
		Error: &ErrorBody{
			Code:    appErr.Code,
			Message: appErr.Message,
			Fields:  appErr.Fields,
		},
	})
}
