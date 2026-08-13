package schema

import "testing"

func TestManagerTableNames(t *testing.T) {
	if got := (User{}).TableName(); got != "manager_users" {
		t.Fatalf("User table = %q", got)
	}
	if got := (HymatrixPod{}).TableName(); got != "manager_hymatrix_pods" {
		t.Fatalf("HymatrixPod table = %q", got)
	}
	if got := (AccessKey{}).TableName(); got != "manager_access_keys" {
		t.Fatalf("AccessKey table = %q", got)
	}
}
