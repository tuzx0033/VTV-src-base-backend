package model

import (
	"testing"
	"time"
)

func TestRoleValid(t *testing.T) {
	t.Parallel()
	cases := map[Role]bool{
		RoleAdmin:   true,
		RoleManager: true,
		RoleStaff:   true,
		Role(""):    false,
		Role("god"): false,
	}
	for r, want := range cases {
		if got := r.Valid(); got != want {
			t.Errorf("Role(%q).Valid() = %v, want %v", r, got, want)
		}
	}
}

func TestUserLockUnlock(t *testing.T) {
	t.Parallel()
	u := &User{Status: UserStatusActive}

	if !u.IsActive() {
		t.Fatal("new active user should be active")
	}
	if u.IsLocked() {
		t.Fatal("new active user should not be locked")
	}

	if err := u.Lock(); err != nil {
		t.Fatalf("Lock() error: %v", err)
	}
	if !u.IsLocked() || u.IsActive() {
		t.Fatalf("after Lock: status=%q", u.Status)
	}

	if err := u.Unlock(); err != nil {
		t.Fatalf("Unlock() error: %v", err)
	}
	if u.IsLocked() || !u.IsActive() {
		t.Fatalf("after Unlock: status=%q", u.Status)
	}
}

func TestUserLockRejectsDeleted(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	u := &User{Status: UserStatusActive, AuditFields: AuditFields{DeletedAt: &now}}

	if u.IsActive() {
		t.Fatal("deleted user must not be active")
	}
	if err := u.Lock(); err != ErrUserDeleted {
		t.Fatalf("Lock() on deleted = %v, want ErrUserDeleted", err)
	}
	if err := u.Unlock(); err != ErrUserDeleted {
		t.Fatalf("Unlock() on deleted = %v, want ErrUserDeleted", err)
	}
}

func TestNewPageNormalizes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		page, limit  int
		wantN, wantS int
		wantOffset   int
	}{
		{0, 0, 1, 20, 0},
		{1, 50, 1, 50, 0},
		{3, 20, 3, 20, 40},
		{2, 5000, 2, 1000, 1000},
		{-5, -5, 1, 20, 0},
	}
	for _, c := range cases {
		p := NewPage(c.page, c.limit)
		if p.Number != c.wantN || p.Size != c.wantS || p.Offset() != c.wantOffset {
			t.Errorf("NewPage(%d,%d) = {%d,%d off=%d}, want {%d,%d off=%d}",
				c.page, c.limit, p.Number, p.Size, p.Offset(), c.wantN, c.wantS, c.wantOffset)
		}
	}
}
