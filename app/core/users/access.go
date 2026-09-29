package users

import (
	"encoding/json"
	"slices"
)

const (
	ACCESS_USER    = "user"
	ACCESS_ADMIN   = "admin"
	ACCESS_VIEWALL = "viewall"
	ACCESS_AM      = "am"
)

var accessList = []string{ACCESS_USER, ACCESS_ADMIN, ACCESS_VIEWALL, ACCESS_AM}

func GetAccessList() []string {
	return slices.Clone(accessList)
}

func GetAccessListJsonStr() string {
	encoded, _ := json.Marshal(accessList)
	return string(encoded)
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
