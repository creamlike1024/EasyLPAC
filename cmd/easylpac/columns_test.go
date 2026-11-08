package main

import "testing"

// The notifications table names each eSIM in a column of its own, beside
// its ICCID; the About header links the repository.
func TestNotificationColumnsAndAboutHeader(t *testing.T) {
	card := newFakeCard()
	card.add("delete", "8981100000000000041") // an eSIM no longer on the card
	f := newFixture(t, card)
	f.chooseReader(t, "ACS ACR38U-CCID")
	f.g.page = pageNotifications
	f.tt.Frame()
	f.shot(t, "notif-cols")
	for _, s := range []string{"ICCID", "Work line", "8944100098765432109", "Unnamed eSIM", "8981100000000000041"} {
		if !f.tt.HasText(s) {
			t.Fatalf("missing %q: %q", s, f.tt.Texts())
		}
	}
	f.g.page = pageAbout
	f.tt.Frame()
	f.shot(t, "about3")
	if !f.tt.HasText("Version: development") || !f.tt.HasText("GitHub Repo") {
		t.Fatalf("about %q", f.tt.Texts())
	}
}
