package cmd

import (
	"bytes"
	"testing"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
)

func TestValueToString(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{nil, "NULL"},
		{float64(42), "42"},
		{float64(8796093055021), "8796093055021"},
		{1.5, "1.5"},
		{"abc", "abc"},
		{true, "true"},
	}
	for _, tc := range cases {
		if got := valueToString(tc.in); got != tc.want {
			t.Errorf("valueToString(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrintQueryCSV(t *testing.T) {
	res := &hac.QueryResult{
		Headers: []string{"code", "name"},
		Results: [][]interface{}{{"p1", "a;b"}, {float64(2), nil}},
	}
	var buf bytes.Buffer
	if err := printQueryCSV(&buf, res); err != nil {
		t.Fatal(err)
	}
	want := "code;name\np1;\"a;b\"\n2;NULL\n"
	if buf.String() != want {
		t.Errorf("printQueryCSV output = %q, want %q", buf.String(), want)
	}
}

func TestPrintQueryTable(t *testing.T) {
	res := &hac.QueryResult{
		Headers: []string{"code", "pk"},
		Results: [][]interface{}{{"p1", float64(1)}},
	}
	var buf bytes.Buffer
	if err := printQueryTable(&buf, res); err != nil {
		t.Fatal(err)
	}
	want := "code  pk\np1    1\n"
	if buf.String() != want {
		t.Errorf("printQueryTable output = %q, want %q", buf.String(), want)
	}
}
