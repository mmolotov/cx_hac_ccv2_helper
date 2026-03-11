package cmd

import "testing"

func TestBuildCountQuery(t *testing.T) {
	cases := []struct {
		typeCode string
		where    string
		want     string
	}{
		{
			typeCode: "Product",
			want:     "SELECT count({PK}) FROM {Product}",
		},
		{
			typeCode: "Order",
			where:    "{status} = 'CREATED'",
			want:     "SELECT count({PK}) FROM {Order} WHERE {status} = 'CREATED'",
		},
	}
	for _, tc := range cases {
		got := buildCountQuery(tc.typeCode, tc.where)
		if got != tc.want {
			t.Errorf("buildCountQuery(%q, %q) = %q, want %q", tc.typeCode, tc.where, got, tc.want)
		}
	}
}
