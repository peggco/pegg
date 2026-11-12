package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"testing"
)

func TestExtractRaw(t *testing.T) {
	data := []byte("binary")
	out, err := ExtractBinary("plugin", data, ExtractOptions{})
	if err != nil || !bytes.Equal(out, data) {
		t.Fatalf("raw: %q %v", out, err)
	}
}

func TestExtractTarGz(t *testing.T) {
	data := tarGz(t, map[string]struct {
		body []byte
		mode int64
	}{
		"dir/plugin": {body: []byte("hello"), mode: 0o755},
		"README":     {body: []byte("readme"), mode: 0o644},
	})
	out, err := ExtractBinary("plugin.tar.gz", data, ExtractOptions{InstallName: "plugin"})
	if err != nil || string(out) != "hello" {
		t.Fatalf("tar: %q %v", out, err)
	}
	out, err = ExtractBinary("plugin.tar.gz", data, ExtractOptions{BinaryPath: "README"})
	if err != nil || string(out) != "readme" {
		t.Fatalf("binary path: %q %v", out, err)
	}
}

func TestExtractZip(t *testing.T) {
	data := zipBytes(t, map[string]struct {
		body []byte
		mode os.FileMode
	}{
		"bin/plugin": {body: []byte("zip-binary"), mode: 0o755},
		"README.txt": {body: []byte("docs"), mode: 0o644},
	})
	out, err := ExtractBinary("plugin.zip", data, ExtractOptions{BinaryPath: "bin/plugin"})
	if err != nil || string(out) != "zip-binary" {
		t.Fatalf("zip: %q %v", out, err)
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	data := tarGz(t, map[string]struct {
		body []byte
		mode int64
	}{
		"../evil": {body: []byte("bad"), mode: 0o755},
	})
	if _, err := ExtractBinary("plugin.tar.gz", data, ExtractOptions{}); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestExtractSkipsSymlink(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "target", Mode: 0o777})
	_ = tw.Close()
	_ = gz.Close()
	if _, err := ExtractBinary("plugin.tar.gz", buf.Bytes(), ExtractOptions{}); err == nil {
		t.Fatal("expected no regular files error")
	}
}

func TestExtractAmbiguousArchive(t *testing.T) {
	data := tarGz(t, map[string]struct {
		body []byte
		mode int64
	}{
		"a": {body: []byte("a"), mode: 0o755},
		"b": {body: []byte("b"), mode: 0o755},
	})
	if _, err := ExtractBinary("plugin.tar.gz", data, ExtractOptions{}); err == nil {
		t.Fatal("expected ambiguity error")
	}
}

func tarGz(t *testing.T, files map[string]struct {
	body []byte
	mode int64
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, file := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: file.mode, Size: int64(len(file.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(file.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipBytes(t *testing.T, files map[string]struct {
	body []byte
	mode os.FileMode
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, file := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(file.mode)
		writer, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(file.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
