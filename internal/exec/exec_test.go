package exec

import "testing"

func TestRenderSubstitutesAndPreserves(t *testing.T) {
	m := map[string]string{"SemVer": "1.2.3"}
	if got := render("echo {SemVer}", m); got != "echo 1.2.3" {
		t.Fatalf("substitute = %q", got)
	}
	// 미지의 토큰은 보존.
	if got := render("echo {Unknown}", m); got != "echo {Unknown}" {
		t.Fatalf("preserve = %q", got)
	}
	// 쉘 변수($)는 영향 없음.
	if got := render("echo $HOME {SemVer}", m); got != "echo $HOME 1.2.3" {
		t.Fatalf("shell var = %q", got)
	}
}
