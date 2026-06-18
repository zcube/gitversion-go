package output

import (
	"strings"
	"testing"
)

func filesVars() *VersionVariables {
	return &VersionVariables{
		AssemblySemVer:       "1.0.1.0",
		AssemblySemFileVer:   "1.0.1.0",
		InformationalVersion: "1.0.1-1+Branch.main",
		SemVer:               "1.0.1-1",
	}
}

func TestAssemblyAttributeReplacement(t *testing.T) {
	src := "[assembly: AssemblyVersion(\"0.0.0.0\")]\n" +
		"[assembly: AssemblyFileVersion(\"0.0.0.0\")]\n" +
		"[assembly: AssemblyInformationalVersion(\"0.0.0.0\")]\n"
	out := replaceAssemblyAttributes(src, filesVars())
	for _, want := range []string{
		`AssemblyVersion("1.0.1.0")`,
		`AssemblyFileVersion("1.0.1.0")`,
		`AssemblyInformationalVersion("1.0.1-1+Branch.main")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestCreateCsAssemblyInfo(t *testing.T) {
	out := createAssemblyInfo("AssemblyInfo.cs", filesVars())
	if !strings.Contains(out, "using System.Reflection;") ||
		!strings.Contains(out, `[assembly: AssemblyFileVersion("1.0.1.0")]`) ||
		!strings.HasPrefix(out, "//---") {
		t.Fatalf("unexpected cs assembly info:\n%s", out)
	}
}

func TestProjectElementReplacementPreservesStructure(t *testing.T) {
	src := "<Project Sdk=\"Microsoft.NET.Sdk\">\n  <!-- 주석 유지 -->\n  <PropertyGroup>\n    <Version>0.0.0</Version>\n    <AssemblyVersion>0.0.0.0</AssemblyVersion>\n  </PropertyGroup>\n</Project>"
	out := replaceProjectElements(src, filesVars())
	for _, want := range []string{
		"<Version>1.0.1-1</Version>",
		"<AssemblyVersion>1.0.1.0</AssemblyVersion>",
		"<!-- 주석 유지 -->",
		`Sdk="Microsoft.NET.Sdk"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestProjectDoesNotTouchUnrelated(t *testing.T) {
	src := "<Project><PropertyGroup><Other>0.0.0</Other></PropertyGroup></Project>"
	out := replaceProjectElements(src, filesVars())
	if !strings.Contains(out, "<Other>0.0.0</Other>") {
		t.Fatalf("unrelated element changed:\n%s", out)
	}
}

func TestPackageJSONVersionUpdate(t *testing.T) {
	src := "{\n  \"name\": \"x\",\n  \"version\": \"0.0.0\",\n  \"private\": true\n}"
	out, ok := updatePackageJSON(src, "1.0.1-1")
	if !ok || !strings.Contains(out, "\"version\": \"1.0.1-1\"") {
		t.Fatalf("version not updated:\n%s", out)
	}
	if strings.Index(out, "\"name\"") > strings.Index(out, "\"version\"") {
		t.Fatal("key order not preserved")
	}
	if !strings.Contains(out, "\"private\"") {
		t.Fatal("private key lost")
	}
}

func TestCargoTomlPreservesComments(t *testing.T) {
	src := "# 주석\n[package]\nname = \"x\"  # inline\nversion = \"0.0.0\"\n"
	out, ok := updateTOMLSection(src, "1.0.1-1", "package")
	if !ok || !strings.Contains(out, "version = \"1.0.1-1\"") ||
		!strings.Contains(out, "# 주석") || !strings.Contains(out, "# inline") {
		t.Fatalf("cargo update wrong:\n%s", out)
	}
}

func TestPyprojectPep621AndPoetry(t *testing.T) {
	pep := "[project]\nname = \"x\"\nversion = \"0.0.0\"\n"
	if out, ok := updatePyproject(pep, "1.0.1-1"); !ok || !strings.Contains(out, "version = \"1.0.1-1\"") {
		t.Fatalf("pep621 wrong:\n%s", out)
	}
	poetry := "[tool.poetry]\nname = \"x\"\nversion = \"0.0.0\"\n"
	if out, ok := updatePyproject(poetry, "2.0.0"); !ok || !strings.Contains(out, "version = \"2.0.0\"") {
		t.Fatalf("poetry wrong:\n%s", out)
	}
}
