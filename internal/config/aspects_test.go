package config

import "testing"

func TestResolveHACCreds(t *testing.T) {
	store := &CredentialsStore{Projects: map[string]ProjectCreds{
		"p": {Subscriptions: map[string]SubCreds{
			"t1": {
				DefaultHACLogin:    "admin",
				DefaultHACPassword: "def",
				Environments: map[string]EnvCreds{
					"p1": {HACLogin: "prodadmin", HACPassword: "prod"},
				},
			},
		}},
	}}
	cases := []struct {
		project, sub, env string
		login, password   string
	}{
		{"p", "t1", "d1", "admin", "def"},
		{"p", "t1", "p1", "prodadmin", "prod"},
		{"p", "t2", "d1", "", ""},
		{"x", "t1", "d1", "", ""},
	}
	for _, tc := range cases {
		l, p := ResolveHACCreds(store, tc.project, tc.sub, tc.env)
		if l != tc.login || p != tc.password {
			t.Errorf("ResolveHACCreds(%s,%s,%s) = %q/%q, want %q/%q",
				tc.project, tc.sub, tc.env, l, p, tc.login, tc.password)
		}
	}
}

func TestResolveAspects(t *testing.T) {
	env := Environment{Code: "d1", Aspects: map[string]Aspect{
		"hcs_platform_backoffice":           {Code: "hcs_platform_backoffice"},
		"hcs_platform_backgroundProcessing": {Code: "hcs_platform_backgroundProcessing"},
		"hcs_platform_api":                  {Code: "hcs_platform_api"},
	}}

	all, err := ResolveAspects(env, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Code != "hcs_platform_api" {
		t.Errorf("all aspects should be sorted, got %+v", all)
	}

	// alias "processing" covers two codes, only one is present
	got, err := ResolveAspects(env, []string{"processing", "backoffice", "hcs_platform_backoffice"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Code != "hcs_platform_backgroundProcessing" || got[1].Code != "hcs_platform_backoffice" {
		t.Errorf("unexpected aspects %+v", got)
	}

	if _, err := ResolveAspects(env, []string{"storefront"}, false); err == nil {
		t.Error("expected error for aspect missing in environment")
	}
	if _, err := ResolveAspects(Environment{Code: "e"}, nil, true); err == nil {
		t.Error("expected error for environment without aspects")
	}
}
