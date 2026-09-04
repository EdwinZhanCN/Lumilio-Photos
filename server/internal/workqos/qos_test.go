package workqos

import "testing"

func TestClassesRoundTripThroughDeliveryPriority(t *testing.T) {
	for _, class := range []Class{Interactive, Background, Maintenance} {
		priority, err := class.Priority()
		if err != nil {
			t.Fatal(err)
		}
		got, err := FromPriority(priority)
		if err != nil {
			t.Fatal(err)
		}
		if got != class {
			t.Fatalf("priority %d restored %s, want %s", priority, got, class)
		}
	}
	if _, err := FromPriority(4); err == nil {
		t.Fatal("unsupported delivery priority was accepted")
	}
}
