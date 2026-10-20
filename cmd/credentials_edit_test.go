package cmd

import (
	"testing"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/config"
)

func TestEffectiveHACCreds(t *testing.T) {
	sc := config.SubCreds{
		DefaultHACLogin: "admin", DefaultHACPassword: "def",
		Environments: map[string]config.EnvCreds{"p1": {HACLogin: "prod", HACPassword: "pp"}},
	}
	if l, p := effectiveHACCreds(sc, "d1"); l != "admin" || p != "def" {
		t.Errorf("d1 = %q/%q, want admin/def", l, p)
	}
	if l, p := effectiveHACCreds(sc, "p1"); l != "prod" || p != "pp" {
		t.Errorf("p1 = %q/%q, want prod/pp", l, p)
	}
}

func TestSetEnvHACCreds(t *testing.T) {
	newSC := func() config.SubCreds {
		return config.SubCreds{
			DefaultHACLogin: "admin", DefaultHACPassword: "def",
			Environments: map[string]config.EnvCreds{"p1": {HACLogin: "prod", HACPassword: "pp"}},
		}
	}

	t.Run("new override keeps default password when empty", func(t *testing.T) {
		sc := newSC()
		setEnvHACCreds(&sc, "d1", "dev", "")
		if got := sc.Environments["d1"]; got != (config.EnvCreds{HACLogin: "dev", HACPassword: "def"}) {
			t.Errorf("d1 = %+v", got)
		}
	})
	t.Run("update override login and password", func(t *testing.T) {
		sc := newSC()
		setEnvHACCreds(&sc, "p1", " prod2 ", "new")
		if got := sc.Environments["p1"]; got != (config.EnvCreds{HACLogin: "prod2", HACPassword: "new"}) {
			t.Errorf("p1 = %+v", got)
		}
	})
	t.Run("update override keeps its own password when empty", func(t *testing.T) {
		sc := newSC()
		setEnvHACCreds(&sc, "p1", "prod", "")
		if got := sc.Environments["p1"]; got.HACPassword != "pp" {
			t.Errorf("p1 password = %q, want pp", got.HACPassword)
		}
	})
	t.Run("empty login removes override", func(t *testing.T) {
		sc := newSC()
		setEnvHACCreds(&sc, "p1", "  ", "ignored")
		if _, ok := sc.Environments["p1"]; ok {
			t.Error("override for p1 should be removed")
		}
		if l, _ := effectiveHACCreds(sc, "p1"); l != "admin" {
			t.Errorf("p1 should fall back to default, got %q", l)
		}
	})
	t.Run("nil map", func(t *testing.T) {
		sc := config.SubCreds{DefaultHACLogin: "admin", DefaultHACPassword: "def"}
		setEnvHACCreds(&sc, "d1", "dev", "x")
		if sc.Environments["d1"].HACLogin != "dev" {
			t.Errorf("d1 override not created: %+v", sc.Environments)
		}
	})
}
