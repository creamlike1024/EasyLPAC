package logmask

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

const (
	eid   = "89049032000001000000123456789068"
	iccid = "8985200012345678901"
	// iccidCard is iccid as the card stores it: F padded, nibbles swapped.
	iccidCard = "985802002143658709F1"
)

func TestSwapNibbles(t *testing.T) {
	if got := swapNibbles(iccid); got != iccidCard {
		t.Fatalf("swapNibbles = %s", got)
	}
}

func TestApplyEncodings(t *testing.T) {
	var s Set
	s.Add(Identifier, eid, iccid)
	eidBytes, _ := hex.DecodeString(eid)
	iccidBytes, _ := hex.DecodeString(iccidCard)
	for _, tc := range []struct {
		name, text, leak string
	}{
		{"EID digits", "BF3E125A10" + eid + "9000", eid},
		{"EID lower-case JSON", `{"eid":"` + eid + `"}`, eid},
		{"ICCID digits", "iccid " + iccid, iccid},
		{"ICCID on the card", "E3125A0A" + iccidCard + "4F10", iccidCard},
		{"ICCID on the card, lower case", "5a0a" + strings.ToLower(iccidCard), strings.ToLower(iccidCard)},
		{"EID in a certificate", "130" + "20" + strings.ToUpper(hex.EncodeToString([]byte(eid))), strings.ToUpper(hex.EncodeToString([]byte(eid)))},
	} {
		got := s.Apply(tc.text)
		if strings.Contains(got, tc.leak) {
			t.Errorf("%s: %q leaks in %q", tc.name, tc.leak, got)
		}
	}
	// Base64, at every alignment of the value in the encoded bytes.
	for _, b := range [][]byte{eidBytes, iccidBytes, []byte(eid)} {
		for lead := range 3 {
			msg := append(append([]byte("0123456789"[:4+lead]), b...), "trailer"...)
			enc := base64.StdEncoding.EncodeToString(msg)
			got := s.Apply(`{"x":"` + enc + `"}`)
			if got == `{"x":"`+enc+`"}` {
				t.Errorf("base64 at alignment %d not obscured: %s", lead, enc)
			}
			inner := base64Inner(b, (4+lead)%3)
			if strings.Contains(got, inner) {
				t.Errorf("base64 at alignment %d leaks %s in %s", lead, inner, got)
			}
		}
	}
	// Digits keep their start, as the UI shows identifiers.
	if got := s.Apply("eid " + eid); got != "eid "+Mask(eid) {
		t.Fatalf("digits = %q", got)
	}
	if got := s.Apply("A0A40400"); got != "A0A40400" {
		t.Fatalf("unrelated text changed: %q", got)
	}
}

func TestApplyCutValues(t *testing.T) {
	var s Set
	s.Add(Identifier, iccid)
	// An ICCID split across two GET RESPONSE blocks.
	first, second := "E3125A0A"+iccidCard[:9], iccidCard[9:]+"4F10A0000005591010"
	if got := s.Apply(first); strings.Contains(got, iccidCard[:9]) || !strings.HasPrefix(got, "E3125A0A") {
		t.Fatalf("first block = %q", got)
	}
	if got := s.Apply(second); strings.Contains(got, iccidCard[9:]) || !strings.HasSuffix(got, "4F10A0000005591010") {
		t.Fatalf("second block = %q", got)
	}
	// Fewer than edgeMin characters of it stay.
	if got := s.Apply("E3125A0A98582"); got != "E3125A0A98582" {
		t.Fatalf("short piece = %q", got)
	}
}

func TestSecrets(t *testing.T) {
	var s Set
	s.Add(Secret, "K2-1ABC23-XYZ9")
	ascii := hex.EncodeToString([]byte("K2-1ABC23-XYZ9"))
	for _, text := range []string{
		"8010" + strings.ToUpper(ascii) + "A1",
		"80" + ascii,
		base64.StdEncoding.EncodeToString([]byte("..ctx K2-1ABC23-XYZ9 ctx..")),
		"matchingId K2-1ABC23-XYZ9",
	} {
		got := s.Apply(text)
		if strings.Contains(got, "K2-1ABC23-XYZ9") || strings.Contains(strings.ToLower(got), ascii) || got == text {
			t.Errorf("secret not obscured in %q: %q", text, got)
		}
	}
}

func TestShortAndEmpty(t *testing.T) {
	var s Set
	s.Add(Identifier, "", "123", "not digits at all")
	s.Add(Secret, "ab")
	if !s.Empty() {
		t.Fatal("short or non-digit values were added")
	}
	var nilSet *Set
	if nilSet.Apply("x") != "x" || !nilSet.Empty() {
		t.Fatal("nil set")
	}
}
