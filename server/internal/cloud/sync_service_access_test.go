package cloud

import (
	"errors"
	"testing"

	"server/internal/db/repo"
)

func TestAuthorizeCredentialAllowsOwnerAndAdministrator(t *testing.T) {
	credential := repo.CloudCredential{OwnerID: 7}

	if err := authorizeCredential(credential, CredentialAccess{UserID: 7}); err != nil {
		t.Fatalf("owner access rejected: %v", err)
	}
	if err := authorizeCredential(credential, CredentialAccess{UserID: 99, IsAdmin: true}); err != nil {
		t.Fatalf("administrator access rejected: %v", err)
	}
}

func TestAuthorizeCredentialRejectsDifferentOwner(t *testing.T) {
	err := authorizeCredential(repo.CloudCredential{OwnerID: 7}, CredentialAccess{UserID: 8})
	if !errors.Is(err, ErrCredentialAccessDenied) {
		t.Fatalf("access error = %v, want ErrCredentialAccessDenied", err)
	}
}
