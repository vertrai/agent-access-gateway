package google

import "testing"

func TestNewWorkspaceAdminUserUsesAgentOrgUnit(t *testing.T) {
	t.Parallel()

	user := newWorkspaceAdminUser(NewGoogleUser{
		Email:      "user123456@vertr.ai",
		Password:   "secret",
		GivenName:  "Agent",
		FamilyName: "user123456",
	})

	if user.OrgUnitPath != "/vertr-ai-agent" {
		t.Fatalf("OrgUnitPath = %q, want %q", user.OrgUnitPath, "/vertr-ai-agent")
	}
	if user.PrimaryEmail != "user123456@vertr.ai" {
		t.Fatalf("PrimaryEmail = %q", user.PrimaryEmail)
	}
	if user.Name == nil || user.Name.GivenName != "Agent" || user.Name.FamilyName != "user123456" {
		t.Fatalf("Name = %#v", user.Name)
	}
	if user.ChangePasswordAtNextLogin {
		t.Fatal("ChangePasswordAtNextLogin should remain disabled")
	}
}
