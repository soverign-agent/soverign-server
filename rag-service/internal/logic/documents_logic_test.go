package logic

import "testing"

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{filename: "resume.pdf", want: "pdf"},
		{filename: "policy.MD", want: "md"},
		{filename: "notes", want: "txt"},
		{filename: "archive.final.docx", want: "docx"},
	}

	for _, tt := range tests {
		if got := detectFileType(tt.filename); got != tt.want {
			t.Fatalf("detectFileType(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}
