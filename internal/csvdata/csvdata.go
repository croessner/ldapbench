package csvdata

// Package csvdata provides loading of benchmark users from a CSV file.
// Expected header: username,password

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// User represents one username/password credential pair.
type User struct {
	Username string
	Password string
	// ExpectedOK reflects optional CSV column `expected_ok`.
	// When the column exists, only rows with true are included by Load.
	ExpectedOK bool
}

// Users holds all parsed users.
type Users struct {
	All []User
}

// Load reads a CSV file and returns all users. Additional columns are ignored.
func Load(path string) (*Users, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	r := csv.NewReader(f)
	// Read header to find indices for username/password.
	h, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	idxU, idxP, idxOK := -1, -1, -1
	for i, name := range h {
		col := strings.TrimSpace(strings.ToLower(name))
		switch col {
		case "username":
			idxU = i
		case "password":
			idxP = i
		case "expected_ok":
			idxOK = i
		}
	}

	if idxU < 0 || idxP < 0 {
		return nil, fmt.Errorf("csv must have username,password headers")
	}

	var users []User
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if idxU >= len(rec) || idxP >= len(rec) {
			continue
		}

		// Trim username and strip trailing CR/LF from password to avoid CSV line-ending artifacts
		u := User{Username: strings.TrimSpace(rec[idxU]), Password: strings.TrimRight(rec[idxP], "\r\n")}

		// If expected_ok column exists, parse and filter accordingly.
		if idxOK >= 0 {
			val := ""
			if idxOK < len(rec) {
				val = rec[idxOK]
			}

			if strings.EqualFold(strings.TrimSpace(val), "true") {
				u.ExpectedOK = true
			} else {
				// Skip row when column exists and not explicitly true
				continue
			}
		}

		users = append(users, u)
	}

	return &Users{All: users}, nil
}
