package activation

import "testing"

func TestDecode(t *testing.T) {
	for _, code := range []string{"", "LPA:", "LPA:1", "LPA:2$example.com"} {
		if _, _, err := Decode(code); err == nil {
			t.Errorf("Decode(%q): expected error", code)
		}
	}

	info, needed, err := Decode("LPA:1$example.com")
	if err != nil || info.SMDP != "example.com" || needed {
		t.Fatalf("got %+v %v %v", info, needed, err)
	}

	info, needed, err = Decode(" LPA:1$example.com$matching-id ")
	if err != nil || info.SMDP != "example.com" || info.MatchID != "matching-id" || needed {
		t.Fatalf("got %+v %v %v", info, needed, err)
	}

	info, needed, err = Decode("LPA:1$example.com$matching-id$$1")
	if err != nil || info.SMDP != "example.com" || info.MatchID != "matching-id" || !needed {
		t.Fatalf("got %+v %v %v", info, needed, err)
	}

	// Extra fields beyond the defined ones must not panic.
	if _, _, err := Decode("LPA:1$example.com$id$oid$1$extra"); err != nil {
		t.Fatal(err)
	}
}

func TestComplete(t *testing.T) {
	const want = "LPA:1$example.com$matching-id"
	for _, in := range []string{want, "1$example.com$matching-id", "$example.com$matching-id"} {
		if got := Complete(in); got != want {
			t.Errorf("Complete(%q) = %q", in, got)
		}
	}
}
