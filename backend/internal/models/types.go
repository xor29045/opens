package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// StringArray maps a PostgreSQL text[] holding simple tokens (ISO codes).
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	parts := make([]string, len(a))
	for i, s := range a {
		if strings.ContainsAny(s, `{},"\ `) {
			return nil, fmt.Errorf("unsupported array element %q", s)
		}
		parts[i] = s
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func (a *StringArray) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case nil:
		*a = StringArray{}
		return nil
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("cannot scan %T into StringArray", src)
	}
	s = strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")
	if s == "" {
		*a = StringArray{}
		return nil
	}
	*a = strings.Split(s, ",")
	return nil
}

// JSON maps a jsonb column as raw bytes.
type JSON []byte

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

func (j *JSON) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*j = JSON("{}")
	case string:
		*j = JSON(v)
	case []byte:
		*j = append(JSON(nil), v...)
	default:
		return fmt.Errorf("cannot scan %T into JSON", src)
	}
	return nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("{}"), nil
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(b []byte) error {
	*j = append(JSON(nil), b...)
	return nil
}
