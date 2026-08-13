package evidence

import "testing"

func TestArgumentsMasksSensitiveValues(t *testing.T) {
	got := Arguments([]string{"--stacktrace", "-PsigningPassword=hunter2", "--token", "abc", "-Dapi.key=xyz", "--Password=hidden"})
	want := []string{"--stacktrace", "-PsigningPassword=<redacted>", "--token", "<redacted>", "-Dapi.key=<redacted>", "--Password=<redacted>"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("argument %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestTextMasksValuesAndLongestPathsFirst(t *testing.T) {
	got := Text("token=abc /home/user/project/file /home/user/other", map[string]string{
		"/home/user":         "$HOME",
		"/home/user/project": "$PROJECT",
	})
	if got != "token=<redacted> $PROJECT/file $HOME/other" {
		t.Fatalf("Text() = %q", got)
	}
}
