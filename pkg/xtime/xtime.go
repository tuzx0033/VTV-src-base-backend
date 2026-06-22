// Package xtime: store/compare UTC everywhere; render Vietnam time only at the edge.
package xtime

import "time"

// VNLoc is the Vietnam timezone (GMT+7, no DST).
var VNLoc = mustLoad("Asia/Ho_Chi_Minh")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("ICT", 7*3600)
	}
	return loc
}

// Now returns the current time in UTC. Always store this.
func Now() time.Time { return time.Now().UTC() }

// ToVN renders t in Vietnam local time (for display only).
func ToVN(t time.Time) time.Time { return t.In(VNLoc) }

// StartOfDayVN returns 00:00:00 of t's date in VN time, expressed in UTC.
func StartOfDayVN(t time.Time) time.Time {
	v := t.In(VNLoc)
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, VNLoc).UTC()
}

// EndOfDayVN returns 23:59:59.999999999 of t's date in VN time, expressed in UTC.
func EndOfDayVN(t time.Time) time.Time {
	v := t.In(VNLoc)
	return time.Date(v.Year(), v.Month(), v.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), VNLoc).UTC()
}

// StartOfMonthVN returns the first instant of t's month in VN time, in UTC.
func StartOfMonthVN(t time.Time) time.Time {
	v := t.In(VNLoc)
	return time.Date(v.Year(), v.Month(), 1, 0, 0, 0, 0, VNLoc).UTC()
}
