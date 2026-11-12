package install

import "testing"

func TestParseChecksums(t *testing.T) {
	data := []byte("abc  plugin.tar.gz\n" +
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef *plugin.zip\n")
	hash, ok := ParseChecksums(data, "plugin.zip")
	if !ok || hash != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("checksum: %q %v", hash, ok)
	}
	if _, ok := ParseChecksums(data, "missing"); ok {
		t.Fatal("unexpected match")
	}
}

func TestVerifySHA256(t *testing.T) {
	data := []byte("hello")
	hash := SHA256Hex(data)
	if err := VerifySHA256(data, hash); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(data, "00"); err == nil {
		t.Fatal("expected mismatch")
	}
}
