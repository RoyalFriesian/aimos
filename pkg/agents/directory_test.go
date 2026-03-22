package agents

import "testing"

func TestMemoryDirectoryFindByCapabilities(t *testing.T) {
	directory := NewMemoryDirectory()
	for _, profile := range []Profile{
		{ID: "sub-ceo-platform", Role: "sub_ceo", Status: "active", Capabilities: []string{"handoff:sub_ceo", "mission:domain", "authority:domain", "topic:platform"}},
		{ID: "sub-ceo-networking", Role: "sub_ceo", Status: "active", Capabilities: []string{"handoff:sub_ceo", "mission:domain", "authority:domain", "topic:networking", "topic:connectivity"}},
		{ID: "execution-owner-networking", Role: "execution_owner", Status: "active", Capabilities: []string{"handoff:execution_owner", "mission:domain", "authority:execution", "topic:networking"}},
	} {
		if err := directory.Register(profile); err != nil {
			t.Fatalf("Register(%s) returned error: %v", profile.ID, err)
		}
	}

	matches, err := directory.FindByCapabilities("sub_ceo", []string{"handoff:sub_ceo", "mission:domain", "authority:domain", "topic:networking"}, 2)
	if err != nil {
		t.Fatalf("FindByCapabilities returned error: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Profile.ID != "sub-ceo-networking" {
		t.Fatalf("expected best match sub-ceo-networking, got %q", matches[0].Profile.ID)
	}
	if matches[0].Score <= matches[1].Score {
		t.Fatalf("expected best match to outrank fallback, got %#v", matches)
	}
	if len(matches[0].MissingCapabilities) != 0 {
		t.Fatalf("expected full match, got missing=%#v", matches[0].MissingCapabilities)
	}
}

func TestMemoryDirectoryClaimByCapabilitiesMarksProfileBusy(t *testing.T) {
	directory := NewMemoryDirectory()
	if err := directory.Register(Profile{ID: "sub-ceo-networking", Role: "sub_ceo", Status: "active", Capabilities: []string{"handoff:sub_ceo", "mission:domain", "authority:domain", "topic:networking"}}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	match, err := directory.ClaimByCapabilities("sub_ceo", []string{"handoff:sub_ceo", "mission:domain", "authority:domain", "topic:networking"})
	if err != nil {
		t.Fatalf("ClaimByCapabilities returned error: %v", err)
	}
	if match.Profile.Status != "busy" {
		t.Fatalf("expected claimed profile to be busy, got %q", match.Profile.Status)
	}
	profile, err := directory.GetProfile("sub-ceo-networking")
	if err != nil {
		t.Fatalf("GetProfile returned error: %v", err)
	}
	if profile.Status != "busy" {
		t.Fatalf("expected stored profile to be busy after claim, got %q", profile.Status)
	}

	if err := directory.UpdateStatus("sub-ceo-networking", "active"); err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	profile, err = directory.GetProfile("sub-ceo-networking")
	if err != nil {
		t.Fatalf("GetProfile returned error after reset: %v", err)
	}
	if profile.Status != "active" {
		t.Fatalf("expected reset profile status active, got %q", profile.Status)
	}
}
