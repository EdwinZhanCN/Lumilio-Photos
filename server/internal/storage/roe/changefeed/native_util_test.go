package changefeed

import "testing"

func TestValidUserRelativePathRejectsLumilioOwnedRootFiles(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		".lumilio",
		".lumilio/assets/thumbnail.webp",
		".lumiliorepo",
		".lumiliorepo.lock",
		".lumilioroot",
		".lumilioroot.lock",
		".lumilio_permission_test-1234",
		".lumilio_case_probe-a1234",
		".lumilio-write-test-1234",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			if validUserRelativePath(value) {
				t.Fatalf("validUserRelativePath(%q) = true, want false for application-owned path", value)
			}
		})
	}

	for _, value := range []string{
		"photo.jpg",
		"album/.hidden.jpg",
		".lumilio-photo.jpg",
	} {
		if !validUserRelativePath(value) {
			t.Fatalf("validUserRelativePath(%q) = false, want true for user media", value)
		}
	}
}
