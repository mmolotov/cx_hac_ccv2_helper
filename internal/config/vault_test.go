package config

import (
	"testing"
)

func TestVaultRoundtrip(t *testing.T) {
	t.Setenv("HAC_HELPER_CONFIG_DIR", t.TempDir())

	master := "super-secret-master-password"

	original := &CredentialsStore{
		Projects: map[string]ProjectCreds{
			"myproject": {
				Subscriptions: map[string]SubCreds{
					"t1": {
						SAPLogin:           "user@example.com",
						SAPPassword:        "hunter2",
						DefaultHACLogin:    "hacuser",
						DefaultHACPassword: "hacpass",
					},
				},
			},
		},
	}

	if err := SaveCredentials(original, master); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	loaded, err := LoadCredentials(master)
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}

	sub := loaded.Projects["myproject"].Subscriptions["t1"]
	if sub.SAPLogin != "user@example.com" {
		t.Errorf("SAPLogin: got %q, want %q", sub.SAPLogin, "user@example.com")
	}
	if sub.SAPPassword != "hunter2" {
		t.Errorf("SAPPassword mismatch")
	}
	if sub.DefaultHACLogin != "hacuser" {
		t.Errorf("DefaultHACLogin mismatch")
	}
}

func TestVaultWrongPassword(t *testing.T) {
	t.Setenv("HAC_HELPER_CONFIG_DIR", t.TempDir())

	store := &CredentialsStore{Projects: map[string]ProjectCreds{}}
	if err := SaveCredentials(store, "correct-password"); err != nil {
		t.Fatal(err)
	}

	_, err := LoadCredentials("wrong-password")
	if err == nil {
		t.Fatal("expected error with wrong password, got nil")
	}
}

func TestVaultMissingFile(t *testing.T) {
	t.Setenv("HAC_HELPER_CONFIG_DIR", t.TempDir())

	// File doesn't exist → should return empty store without error
	store, err := LoadCredentials("any-password")
	if err != nil {
		t.Fatalf("LoadCredentials on missing file: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil empty store")
	}
}
