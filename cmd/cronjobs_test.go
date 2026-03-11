package cmd

import "testing"

func TestBuildCronJobsCondition(t *testing.T) {
	cases := []struct {
		field string
		value string
		like  bool
		want  string
	}{
		{
			field: "{cj.code}",
			value: "myBackofficeJob",
			like:  false,
			want:  "{cj.code} = 'myBackofficeJob'",
		},
		{
			field: "{j.code}",
			value: "myJob",
			like:  true,
			want:  "{j.code} LIKE '%myJob%'",
		},
		{
			field: "{sj.springId}",
			value: "com.example.bean",
			like:  false,
			want:  "{sj.springId} = 'com.example.bean'",
		},
		{
			field: "{sj.springId}",
			value: "myBean",
			like:  true,
			want:  "{sj.springId} LIKE '%myBean%'",
		},
	}
	for _, tc := range cases {
		got := buildCronJobsCondition(tc.field, tc.value, tc.like)
		if got != tc.want {
			t.Errorf("buildCronJobsCondition(%q, %q, %v) = %q, want %q",
				tc.field, tc.value, tc.like, got, tc.want)
		}
	}
}
