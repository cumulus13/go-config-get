// File: cast.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-04-27
// Description: Package cast provides type coercion utilities for config values.
// License: MIT

// Package cast provides type coercion utilities for config values.
package cast

import (
	"fmt"
	"strconv"
	"strings"
)

// Type represents a supported cast target.
type Type string

const (
	TypeString Type = "string"
	TypeInt    Type = "int"
	TypeFloat  Type = "float"
	TypeBool   Type = "bool"
)

// ToString coerces v to a string.
func ToString(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	case bool:
		return strconv.FormatBool(val), nil
	case int:
		return strconv.Itoa(val), nil
	case int64:
		return strconv.FormatInt(val, 10), nil
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	case nil:
		return "", nil
	default:
		return fmt.Sprintf("%v", val), nil
	}
}

// ToInt coerces v to an int64.
func ToInt(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int64:
		return val, nil
	case float64:
		return int64(val), nil
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			// try float then truncate
			f, ferr := strconv.ParseFloat(strings.TrimSpace(val), 64)
			if ferr != nil {
				return 0, fmt.Errorf("cannot cast %q to int: %w", val, err)
			}
			return int64(f), nil
		}
		return i, nil
	default:
		s, _ := ToString(v)
		return ToInt(s)
	}
}

// ToFloat coerces v to a float64.
func ToFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case bool:
		if val {
			return 1.0, nil
		}
		return 0.0, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return 0, fmt.Errorf("cannot cast %q to float: %w", val, err)
		}
		return f, nil
	default:
		s, _ := ToString(v)
		return ToFloat(s)
	}
}

// ToBool coerces v to a bool.
// Truthy strings: "1", "true", "yes", "on" (case-insensitive).
// Falsy strings : "0", "false", "no",  "off" and everything else.
func ToBool(v interface{}) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case int:
		return val != 0, nil
	case int64:
		return val != 0, nil
	case float64:
		return val != 0, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "1", "true", "yes", "on":
			return true, nil
		case "0", "false", "no", "off", "":
			return false, nil
		default:
			return false, fmt.Errorf("cannot cast %q to bool", val)
		}
	default:
		s, _ := ToString(v)
		return ToBool(s)
	}
}
