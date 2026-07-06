package baseline

import (
	"path/filepath"
	"testing"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

func results(codes ...string) []hatchet.Result {
	r := hatchet.Result{Target: "https://example.com"}
	for _, c := range codes {
		r.Issues = append(r.Issues, hatchet.Issue{
			Code: c, Type: hatchet.TypeError, TypeCode: 1,
			Selector: "#" + c, Context: "<div id=\"" + c + "\">x</div>",
		})
	}
	return []hatchet.Result{r}
}

func TestFingerprintStability(t *testing.T) {
	is := hatchet.Issue{Code: "image-alt", Selector: "img", Context: "<img   src=\"x.png\">"}
	a := Fingerprint("https://example.com", is)

	// Whitespace-only context changes must not alter the fingerprint.
	is.Context = "<img src=\"x.png\">"
	if b := Fingerprint("https://example.com", is); a != b {
		t.Error("fingerprint changed on whitespace-only context change")
	}
	// Different target must.
	if b := Fingerprint("https://other.example.com", is); a == b {
		t.Error("fingerprint identical across targets")
	}
	// Different selector must.
	is.Selector = "img:nth-child(2)"
	if b := Fingerprint("https://example.com", is); a == b {
		t.Error("fingerprint identical across selectors")
	}
}

func TestRoundTripAndApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	bl := New(results("image-alt", "label"), "4.10.3")
	if len(bl.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(bl.Entries))
	}
	if err := bl.Write(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AxeVersion != "4.10.3" || len(loaded.Entries) != 2 {
		t.Fatalf("round trip lost data: %+v", loaded)
	}

	// Same issues plus one new one: only the new one survives Apply.
	rs := results("image-alt", "label", "color-contrast")
	stats := loaded.Apply(rs)
	if stats.New != 1 || stats.Baselined != 2 || stats.Fixed != 0 {
		t.Errorf("stats = %+v", stats)
	}
	if len(rs[0].Issues) != 1 || rs[0].Issues[0].Code != "color-contrast" {
		t.Errorf("kept issues = %+v", rs[0].Issues)
	}
}

func TestApplyCountsFixed(t *testing.T) {
	bl := New(results("image-alt", "label"), "4.10.3")

	// label is fixed; image-alt remains.
	rs := results("image-alt")
	stats := bl.Apply(rs)
	if stats.Fixed != 1 || stats.Baselined != 1 || stats.New != 0 {
		t.Errorf("stats = %+v", stats)
	}
}

func TestPrune(t *testing.T) {
	bl := New(results("image-alt", "label"), "4.10.3")

	bl.Prune(results("image-alt")) // label fixed → pruned
	if len(bl.Entries) != 1 || bl.Entries[0].Code != "image-alt" {
		t.Errorf("entries after prune = %+v", bl.Entries)
	}
}

func TestNewDeterministic(t *testing.T) {
	a := New(results("b-code", "a-code"), "4.10.3")
	b := New(results("a-code", "b-code"), "4.10.3")
	if len(a.Entries) != len(b.Entries) {
		t.Fatal("entry counts differ")
	}
	for i := range a.Entries {
		if a.Entries[i].Fingerprint != b.Entries[i].Fingerprint {
			t.Fatal("entry order depends on input order; must be sorted")
		}
	}
}
