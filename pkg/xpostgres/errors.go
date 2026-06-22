package xpostgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"vtv.vn/backend/pkg/xhttp"
)

// MapError translates Postgres driver errors into AppError when possible so
// repository callers don't leak raw driver errors as generic 500s. Unrecognised
// errors pass through unchanged.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		return err
	}
	switch pg.Code {
	case "23503": // foreign_key_violation
		return xhttp.UnprocessableErrorf("dữ liệu tham chiếu không hợp lệ").WithField(pg.ConstraintName).Wrap(err)
	case "23505": // unique_violation
		return xhttp.ConflictErrorf("bản ghi đã tồn tại").WithField(pg.ConstraintName).Wrap(err)
	case "23502": // not_null_violation
		return xhttp.BadRequestErrorf("trường bắt buộc bị thiếu").WithField(pg.ColumnName).Wrap(err)
	case "23514": // check_violation
		return xhttp.BadRequestErrorf("dữ liệu vi phạm ràng buộc").WithField(pg.ConstraintName).Wrap(err)
	case "22001": // string_data_right_truncation
		return xhttp.BadRequestErrorf("giá trị nhập vào quá dài").WithField(pg.ColumnName).Wrap(err)
	case "22003", "22P02": // numeric_value_out_of_range / invalid_text_representation
		return xhttp.BadRequestErrorf("giá trị số không hợp lệ").WithField(pg.ColumnName).Wrap(err)
	}
	return err
}
