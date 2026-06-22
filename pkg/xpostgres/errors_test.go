package xpostgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"vtv.vn/backend/pkg/xhttp"
)

func TestMapError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int // 0 = passthrough (not an AppError)
	}{
		{"nil passes through", nil, 0},
		{"non-pg error passes through", errors.New("boom"), 0},
		{"unique violation → 409", &pgconn.PgError{Code: "23505", ConstraintName: "products_code_key"}, 409},
		{"fk violation → 422", &pgconn.PgError{Code: "23503"}, 422},
		{"not null → 400", &pgconn.PgError{Code: "23502", ColumnName: "name"}, 400},
		{"check → 400", &pgconn.PgError{Code: "23514"}, 400},
		{"string too long → 400", &pgconn.PgError{Code: "22001", ColumnName: "external_id"}, 400},
		{"numeric out of range → 400", &pgconn.PgError{Code: "22003", ColumnName: "default_unit_cost"}, 400},
		{"wrapped pg error is unwrapped", fmt.Errorf("variant X: %w", &pgconn.PgError{Code: "23505"}), 409},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapError(tt.err)
			if tt.wantStatus == 0 {
				if ae, ok := xhttp.AsAppError(got); ok {
					t.Fatalf("expected passthrough, got AppError status %d", ae.Status)
				}
				return
			}
			ae, ok := xhttp.AsAppError(got)
			if !ok {
				t.Fatalf("expected AppError, got %v", got)
			}
			if ae.Status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", ae.Status, tt.wantStatus)
			}
			if ae.Message == "" {
				t.Fatal("expected non-empty message")
			}
		})
	}
}
