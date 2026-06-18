package version

import "testing"

func TestParseBasic(t *testing.T) {
	v, ok := Parse("v1.2.3", "[vV]?")
	if !ok || v.Major != 1 || v.Minor != 2 || v.Patch != 3 {
		t.Fatalf("parse v1.2.3: %+v ok=%v", v, ok)
	}
	if v.PreReleaseTag.HasTag() {
		t.Fatal("v1.2.3 should not have pre-release tag")
	}
}

func TestParsePartialAndPrerelease(t *testing.T) {
	v, _ := Parse("1.2", "[vV]?")
	if v.Major != 1 || v.Minor != 2 || v.Patch != 0 {
		t.Fatalf("1.2 -> %+v", v)
	}
	v, _ = Parse("2.0.0-beta.4", "[vV]?")
	if v.PreReleaseTag.Name != "beta" || v.PreReleaseTag.Number == nil || *v.PreReleaseTag.Number != 4 {
		t.Fatalf("beta.4 -> %+v", v.PreReleaseTag)
	}
}

func TestStrictRejectsPartial(t *testing.T) {
	if _, ok := ParseWith("1.2", "[vV]?", true); ok {
		t.Fatal("strict should reject 1.2")
	}
	if _, ok := ParseWith("1.2.3.4", "[vV]?", true); ok {
		t.Fatal("strict should reject 4-part")
	}
	if _, ok := ParseWith("01.02.03", "[vV]?", true); ok {
		t.Fatal("strict should reject leading zero")
	}
	if _, ok := ParseWith("1.2.3", "[vV]?", true); !ok {
		t.Fatal("strict should accept 1.2.3")
	}
}

func TestLooseFourPart(t *testing.T) {
	v, ok := ParseWith("1.2.3.4", "[vV]?", false)
	if !ok || v.BuildMetaData.CommitsSinceTag == nil || *v.BuildMetaData.CommitsSinceTag != 4 {
		t.Fatalf("loose 1.2.3.4 -> %+v", v)
	}
}

func TestOrderingStableGtPrerelease(t *testing.T) {
	stable, _ := Parse("1.0.0", "")
	pre, _ := Parse("1.0.0-alpha.1", "")
	if stable.Compare(pre) <= 0 {
		t.Fatal("stable should be greater than pre-release")
	}
}

func TestIncrementEmptyLabelPromotes(t *testing.T) {
	base := NewSemanticVersion(0, 0, 0)
	empty := ""
	v := base.Increment(FieldPatch, &empty, false)
	if v.MajorMinorPatch() != "0.0.1" || v.String() != "0.0.1-1" {
		t.Fatalf("empty label promote -> %s", v.String())
	}
}

func TestIncrementNamedLabelResetsToOne(t *testing.T) {
	base := NewSemanticVersion(1, 0, 0)
	alpha := "alpha"
	v := base.Increment(FieldMinor, &alpha, false)
	if v.String() != "1.1.0-alpha.1" {
		t.Fatalf("-> %s", v.String())
	}
}

func TestIncrementSameLabelBumpsNumber(t *testing.T) {
	base := NewSemanticVersion(1, 1, 0)
	base.PreReleaseTag = NewPreReleaseTag("alpha", i64ptr(1), false)
	alpha := "alpha"
	v := base.Increment(FieldMinor, &alpha, false)
	if v.String() != "1.1.0-alpha.2" {
		t.Fatalf("-> %s", v.String())
	}
}

func TestBuildMetaSanitize(t *testing.T) {
	c := int64(3)
	m := BuildMetaData{CommitsSinceTag: &c, Branch: "feature/foo", Sha: "abc1234", OtherMetadata: "extra!info"}
	full := m.FormatFull()
	if want := "3.Branch.feature-foo.Sha.abc1234.extra-info"; full != want {
		t.Fatalf("FormatFull = %q want %q", full, want)
	}
}
