package config

import "testing"

func TestBranchPrefixRoundtrip(t *testing.T) {
	repo := t.TempDir()

	// Default: prefix == type, no override stored.
	if got := GetBranchPrefix(repo, "feature"); got != "feature" {
		t.Fatalf("default prefix = %q, want %q", got, "feature")
	}

	// Set a custom prefix (materializes full policy into branchTypes).
	if err := SetBranchTypePrefix(repo, "feature", "feat"); err != nil {
		t.Fatalf("SetBranchTypePrefix: %v", err)
	}
	if got := GetBranchPrefix(repo, "feature"); got != "feat" {
		t.Fatalf("prefix after set = %q, want %q", got, "feat")
	}
	if got := FormatBranchNameRepo(repo, "feature", "PROJ-123"); got != "feat/PROJ-123" {
		t.Fatalf("FormatBranchNameRepo = %q, want %q", got, "feat/PROJ-123")
	}

	// Reverse mapping: custom prefix maps back to the type name.
	if got := BranchTypeFromName(repo, "feat/PROJ-123"); got != "feature" {
		t.Fatalf("BranchTypeFromName(feat/...) = %q, want %q", got, "feature")
	}
	// Unconfigured type keeps identity mapping.
	if got := BranchTypeFromName(repo, "bugfix/PROJ-9"); got != "bugfix" {
		t.Fatalf("BranchTypeFromName(bugfix/...) = %q, want %q", got, "bugfix")
	}
	// Unknown prefix falls through to the raw prefix.
	if got := BranchTypeFromName(repo, "release/PROJ-1"); got != "release" {
		t.Fatalf("BranchTypeFromName(release/...) = %q, want %q", got, "release")
	}

	// Reset prefix to default.
	if err := SetBranchTypePrefix(repo, "feature", ""); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if got := GetBranchPrefix(repo, "feature"); got != "feature" {
		t.Fatalf("prefix after reset = %q, want %q", got, "feature")
	}
}

func TestResolveBranchTypesDefaultsAndLegacyOverlay(t *testing.T) {
	repo := t.TempDir()

	// No config: built-in defaults.
	defs := ResolveBranchTypes(repo)
	if len(defs) != len(DefaultBranchTypeDefs) {
		t.Fatalf("default defs len = %d, want %d", len(defs), len(DefaultBranchTypeDefs))
	}
	if defs[0].Name != "feature" || defs[0].Strategy != StrategyFromDefault {
		t.Fatalf("feature default = %+v", defs[0])
	}

	// Legacy overlay: baseBranches pins a base and flips from-default → from-branch.
	if err := SaveRepoConfig(repo, &RepoConfig{
		BaseBranches:   map[string]string{"feature": "develop"},
		BranchPrefixes: map[string]string{"feature": "feat"},
	}); err != nil {
		t.Fatalf("SaveRepoConfig: %v", err)
	}
	d, ok := GetBranchTypeDef(repo, "feature")
	if !ok {
		t.Fatal("feature def missing")
	}
	if d.Base != "develop" {
		t.Fatalf("overlay base = %q, want develop", d.Base)
	}
	if d.Strategy != StrategyFromBranch {
		t.Fatalf("overlay strategy = %q, want from-branch", d.Strategy)
	}
	if d.EffectivePrefix() != "feat" {
		t.Fatalf("overlay prefix = %q, want feat", d.EffectivePrefix())
	}
}

func TestScaffoldBranchTypes(t *testing.T) {
	repo := t.TempDir()
	defs, err := ScaffoldBranchTypes(repo)
	if err != nil {
		t.Fatalf("ScaffoldBranchTypes: %v", err)
	}
	if len(defs) != len(DefaultBranchTypeDefs) {
		t.Fatalf("scaffold len = %d", len(defs))
	}
	if !HasBranchTypesConfig(repo) {
		t.Fatal("branchTypes should be persisted after scaffold")
	}
}
