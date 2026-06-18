package buildagent

import (
	"reflect"
	"testing"

	"github.com/zcube/go-gitversion/internal/output"
)

func sample() *output.VersionVariables {
	return &output.VersionVariables{FullSemVer: "1.0.1-1"}
}

func TestAppVeyorHTTPBody(t *testing.T) {
	vars := &output.VersionVariables{FullSemVer: "1.2.3-beta.1"}
	if got := appVeyorBuildNumberBody(vars, "42"); got != `{"version":"1.2.3-beta.1.build.42"}` {
		t.Fatalf("build number body = %q", got)
	}
	if got := appVeyorOutputVariableBody("Major", "1"); got != `{"name":"GitVersion_Major","value":"1"}` {
		t.Fatalf("output variable body = %q", got)
	}
}

func TestTeamCityFormat(t *testing.T) {
	a := teamCity()
	if got := a.SetBuildNumber(sample()); got != "##teamcity[buildNumber '1.0.1-1']" {
		t.Fatalf("build number = %q", got)
	}
	want := []string{
		"##teamcity[setParameter name='GitVersion.FullSemVer' value='1.0.1-1']",
		"##teamcity[setParameter name='system.GitVersion.FullSemVer' value='1.0.1-1']",
	}
	if got := a.SetOutputVariable("FullSemVer", "1.0.1-1"); !reflect.DeepEqual(got, want) {
		t.Fatalf("output = %v", got)
	}
}

func TestTeamCityEscape(t *testing.T) {
	a := teamCity()
	want := []string{
		"##teamcity[setParameter name='GitVersion.X' value='a|'b|[c|]']",
		"##teamcity[setParameter name='system.GitVersion.X' value='a|'b|[c|]']",
	}
	if got := a.SetOutputVariable("X", "a'b[c]"); !reflect.DeepEqual(got, want) {
		t.Fatalf("escape = %v", got)
	}
}

func TestBuildNumberSkippedWhenDisabled(t *testing.T) {
	out := teamCity().WriteIntegration(sample(), false)
	for _, l := range out {
		if l == "##teamcity[buildNumber '1.0.1-1']" {
			t.Fatal("build number should be skipped when disabled")
		}
	}
}
