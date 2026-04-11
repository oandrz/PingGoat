package httputil

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func IntFromInt4(v pgtype.Int4) int {
	if !v.Valid {
		return 0
	}
	return int(v.Int32)
}

func FormatNullableTime(v pgtype.Timestamptz) *string {
	if !v.Valid {
		return nil
	}
	return new(v.Time.Format(time.RFC3339))
}

func FormatNullableString(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return new(v.String)
}
