// Package exchanges reads core's tribrach.exchanges.v1 and
// tribrach.verdicts.v1 files, for tests.
package exchanges

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
)

// File is a tribrach.exchanges.v1 file.
type File struct {
	Format   string `json:"format"`
	Contract string `json:"contract"`
	Version  int    `json:"version"`
	Cases    []Case `json:"cases"`
}

// Case is one recorded exchange.
type Case struct {
	ID       string          `json:"id"`
	Given    Given           `json:"given"`
	Request  json.RawMessage `json:"request"`
	Status   int             `json:"status"`
	Calls    []Call          `json:"calls"`
	Response json.RawMessage `json:"response"`
}

// Given is what the model was told besides the request.
type Given struct {
	AuthID       string `json:"authId"`
	Card         string `json:"card"` // approve or decline
	Now          string `json:"now"`
	SessionID    string `json:"sessionId"`
	Token        string `json:"token"`
	ChargersFree *int   `json:"chargersFree"`
}

// Call is a call the service made to another one, with its answer.
type Call struct {
	Contract string          `json:"contract"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
}

// Verdicts is a tribrach.verdicts.v1 file.
type Verdicts struct {
	Format string `json:"format"`
	Schema string `json:"schema"`
	Cases  []struct {
		Doc    json.RawMessage `json:"doc"`
		Accept bool            `json:"accept"`
	} `json:"cases"`
}

// Load reads a JSON file and checks its format.
func Load(path, format string, into any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var head struct{ Format string }
	json.Unmarshal(data, &head)
	if head.Format != format {
		return fmt.Errorf("%s: format %q, want %q", path, head.Format, format)
	}
	return nil
}

// SameJSON reports whether two documents are equal as JSON values (key
// order and spacing aside; numbers compared exactly as written values).
func SameJSON(a, b []byte) bool {
	var x, y any
	da := json.NewDecoder(bytes.NewReader(a))
	da.UseNumber()
	db := json.NewDecoder(bytes.NewReader(b))
	db.UseNumber()
	if da.Decode(&x) != nil || db.Decode(&y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}
