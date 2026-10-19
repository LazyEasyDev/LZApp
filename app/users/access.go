package users

import (
	"encoding/json"
	"fmt"
	"slices"
)

const (
	AccessUser    = "user"
	AccessAdmin   = "admin"
	AccessViewAll = "viewall"
	AccessAM      = "am"
)

var accessList = []string{AccessUser, AccessAdmin, AccessViewAll, AccessAM}

func GetAccessList() []string {
	return slices.Clone(accessList)
}

func GetAccessListJSON() string {
	encoded, _ := json.Marshal(accessList)
	return string(encoded)
}

func EncodeAccess(values []string) (string, error) {
	normalized := make([]string, 0, len(values))
	for _, permission := range values {
		if !slices.Contains(accessList, permission) {
			return "", fmt.Errorf("unknown access permission %q", permission)
		}
		if !slices.Contains(normalized, permission) {
			normalized = append(normalized, permission)
		}
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("encode access: %w", err)
	}
	return string(encoded), nil
}

func normalizeAccess(value string) (string, error) {
	if value == "" {
		return "[]", nil
	}
	var access []string
	if err := json.Unmarshal([]byte(value), &access); err != nil {
		return "", fmt.Errorf("access must be a JSON array of strings: %w", err)
	}
	if access == nil {
		return "", fmt.Errorf("access must be a JSON array of strings")
	}
	return EncodeAccess(access)
}

func (user *User) HaveAllAccess(accessList []string) bool {
	if user == nil {
		return false
	}
	var granted []string
	if err := json.Unmarshal([]byte(user.Access), &granted); err != nil || granted == nil {
		return false
	}
	for _, access := range accessList {
		if !slices.Contains(granted, access) {
			return false
		}
	}
	return true
}

func (user *User) HaveAnyAccess(accessList []string) bool {
	if user == nil {
		return false
	}
	var granted []string
	if err := json.Unmarshal([]byte(user.Access), &granted); err != nil || granted == nil {
		return false
	}
	for _, access := range accessList {
		if slices.Contains(granted, access) {
			return true
		}
	}
	return false
}
