package jira

import "github.com/felixgeelhaar/go-teamhealthcheck/sdk"

func init() {
	sdk.Register(&JiraPlugin{})
}
