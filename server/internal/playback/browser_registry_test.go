package playback

import "testing"

func TestBrowserDeviceRegistry_RegisterListUnregister(t *testing.T) {
	r := NewBrowserDeviceRegistry()

	r.Register("c1", 42, "Chrome on macOS")
	r.Register("c2", 42, "Safari on iPhone")
	r.Register("c3", 7, "Firefox on Linux")

	devs := r.ListByUser(42)
	if len(devs) != 2 {
		t.Fatalf("expected 2 devices for user 42, got %d", len(devs))
	}

	r.Unregister("c1")
	devs = r.ListByUser(42)
	if len(devs) != 1 {
		t.Fatalf("expected 1 device for user 42 after unregister, got %d", len(devs))
	}
	if devs[0].ClientID != "c2" {
		t.Errorf("expected remaining device c2, got %q", devs[0].ClientID)
	}

	if d, ok := r.Get("c3"); !ok || d.UserID != 7 {
		t.Errorf("Get(c3): got %v, %v", d, ok)
	}
}

func TestBrowserDeviceRegistry_AnonymousExcludedFromList(t *testing.T) {
	r := NewBrowserDeviceRegistry()
	r.Register("c1", 0, "Browser") // anonymous
	r.Register("c2", 1, "Chrome")

	if got := r.ListByUser(0); len(got) != 0 {
		t.Errorf("ListByUser(0) should return no anonymous devices; got %d", len(got))
	}
	if got := r.ListByUser(1); len(got) != 1 {
		t.Errorf("ListByUser(1) expected 1 device; got %d", len(got))
	}
}

func TestBrowserDeviceRegistry_RegisterReplaces(t *testing.T) {
	r := NewBrowserDeviceRegistry()
	r.Register("c1", 42, "OldName")
	r.Register("c1", 42, "NewName")
	d, _ := r.Get("c1")
	if d.Name != "NewName" {
		t.Errorf("expected re-register to overwrite name; got %q", d.Name)
	}
}

func TestBrowserDeviceRegistry_EmptyClientIDIgnored(t *testing.T) {
	r := NewBrowserDeviceRegistry()
	r.Register("", 42, "Bogus")
	if got := r.ListByUser(42); len(got) != 0 {
		t.Errorf("empty clientID must not register; got %d", len(got))
	}
	r.Unregister("") // must not panic
}
