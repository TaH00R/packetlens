package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type EpisodeQuery struct {
	Filter
	Limit  int
	Before string
}
type AnomalyQuery = EpisodeQuery

func validEventID(id string) bool {
	parts := strings.Split(id, "-")
	if len(parts) != 2 || !validID(parts[0]) {
		return false
	}
	n, err := strconv.Atoi(parts[1])
	return err == nil && n >= 0 && n < 1_000_000
}
func episodeCursor(at time.Time, id string) string {
	b, _ := json.Marshal(cursor{Timestamp: formatTime(at), ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}
func readEpisodeCursor(value string) (cursor, error) {
	var c cursor
	if len(value) > 256 {
		return c, fmt.Errorf("%w: invalid episode cursor", ErrInvalidQuery)
	}
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(b, &c) != nil || !validEventID(c.ID) {
		return c, fmt.Errorf("%w: invalid episode cursor", ErrInvalidQuery)
	}
	t, err := time.Parse(timestampLayout, c.Timestamp)
	if err != nil || t.Year() < 1 || formatTime(t) != c.Timestamp {
		return c, fmt.Errorf("%w: invalid episode cursor", ErrInvalidQuery)
	}
	return c, nil
}
func (q EpisodeQuery) validate() (EpisodeQuery, error) {
	if err := q.Filter.Validate(); err != nil {
		return q, err
	}
	if q.Limit == 0 {
		q.Limit = 50
	}
	if q.Limit < 1 || q.Limit > 200 {
		return q, fmt.Errorf("%w: episode limit must be 1..200", ErrInvalidQuery)
	}
	if q.Before != "" {
		if _, err := readEpisodeCursor(q.Before); err != nil {
			return q, err
		}
	}
	return q, nil
}

func episodeConditions(f Filter, overlap bool) (string, []any) {
	where := "1=1"
	args := []any{}
	if f.From != nil {
		if overlap {
			where += " AND (recovery_cycle IS NULL OR recovery_cycle >= ?)"
		} else {
			where += " AND cycle_timestamp >= ?"
		}
		args = append(args, formatTime(*f.From))
	}
	if f.To != nil {
		where += " AND cycle_timestamp <= ?"
		args = append(args, formatTime(*f.To))
	}
	if f.Type != "" {
		where += " AND type=?"
		args = append(args, f.Type)
	}
	if f.Target != "" {
		where += " AND target=?"
		args = append(args, f.Target)
	}
	return where, args
}
