package entity

import "encoding/json"

type Data struct {
	ID     int             `json:"id"`
	Path   string          `json:"path"`
	Source string          `json:"source"`
	Meta   json.RawMessage `json:"meta"`
}
