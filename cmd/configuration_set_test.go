package cmd

import "testing"

func TestParseKVPairs(t *testing.T) {
	got, err := parseKVPairs([]string{"a=1", "b.c=x=y", "empty="})
	if err != nil {
		t.Fatal(err)
	}
	want := []configKV{{"a", "1"}, {"b.c", "x=y"}, {"empty", ""}}
	if len(got) != len(want) {
		t.Fatalf("got %d pairs, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pair %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	for _, bad := range []string{"noequals", "=value"} {
		if _, err := parseKVPairs([]string{bad}); err == nil {
			t.Errorf("parseKVPairs(%q) expected error", bad)
		}
	}
}
