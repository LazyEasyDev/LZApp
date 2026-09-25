package users

import (
	"encoding/json"
	"slices"
)

const (
	ACCESS_ADMIN   = "admin"
	ACCESS_VIEWALL = "viewall"
	ACCESS_AM      = "am"
)

var accessList = []string{ACCESS_ADMIN, ACCESS_VIEWALL, ACCESS_AM}

func GetAccessList() []string {
	return slices.Clone(accessList)
}

func GetAccessListJsonStr() string {
	encoded, _ := json.Marshal(accessList)
	return string(encoded)
}
