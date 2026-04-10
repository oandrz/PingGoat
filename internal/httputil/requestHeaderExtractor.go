package httputil

import (
	"errors"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

func GetUserID(r *http.Request, pgUUID *pgtype.UUID, key any) error {
	userID, ok := r.Context().Value(key).(string)
	if !ok {
		log.Printf("User not found in system")
		return errors.New("User not found in system")
	}

	err := pgUUID.Scan(userID)
	if err != nil {
		log.Printf("User ID Parsing Error")
		return errors.New("User ID Parsing Error")
	}

	return nil
}
