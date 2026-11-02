// Package fakeauthentik answers the few Authentik API calls multipass makes,
// from a fixed set of test members, for running multipass locally and in
// tests without a real Authentik. It checks no token and holds no real data.
package fakeauthentik

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type user struct {
	PK         int            `json:"pk"`
	Username   string         `json:"username"`
	Name       string         `json:"name"`
	Email      string         `json:"email"`
	IsActive   bool           `json:"is_active"`
	Groups     []string       `json:"-"`
	Attributes map[string]any `json:"attributes"`
}

// Users are the test members. test.user@example.com is who development
// mode signs everyone in as; staff@example.com is in the staff group.
var Users = []user{
	{PK: 42, Username: "test.user", Name: "Test User", Email: "test.user@example.com", IsActive: true,
		Groups:     []string{"Members"},
		Attributes: map[string]any{"member_since": "2024-03-01", "membership_type": "Full", "membership_status": "active"}},
	{PK: 43, Username: "ada", Name: "Zoë Ada O'Brien-Lovelace", Email: "ada@example.com", IsActive: true,
		Groups:     []string{"Members"},
		Attributes: map[string]any{"member_since": "2021-11-15", "membership_type": "Full", "membership_status": "active"}},
	{PK: 7, Username: "staff", Name: "Sam Staff", Email: "staff@example.com", IsActive: true,
		Groups:     []string{"Members", "staff"},
		Attributes: map[string]any{"member_since": "2019-06-01", "membership_status": "active"}},
}

// Handler serves the users and groups API from the test members.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/core/users/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v3/core/users/"), "/")
		if rest == "" { // ?email=
			var found []user
			for _, u := range Users {
				if email := r.URL.Query().Get("email"); email == "" || strings.EqualFold(u.Email, email) {
					found = append(found, u)
				}
			}
			page(w, found)
			return
		}
		pk, _ := strconv.Atoi(rest)
		for _, u := range Users {
			if u.PK == pk {
				writeJSON(w, u)
				return
			}
		}
		http.Error(w, `{"detail":"Not found."}`, http.StatusNotFound)
	})

	mux.HandleFunc("/api/v3/core/groups/", func(w http.ResponseWriter, r *http.Request) {
		pk, _ := strconv.Atoi(r.URL.Query().Get("user"))
		var groups []map[string]string
		for _, u := range Users {
			if u.PK == pk {
				for _, g := range u.Groups {
					groups = append(groups, map[string]string{"name": g})
				}
			}
		}
		page(w, groups)
	})

	return mux
}

func page[T any](w http.ResponseWriter, results []T) {
	if results == nil {
		results = []T{}
	}
	writeJSON(w, map[string]any{"count": len(results), "next": nil, "previous": nil, "results": results})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
