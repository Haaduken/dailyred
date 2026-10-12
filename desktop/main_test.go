package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func testROM() []byte {
	return append([]byte("DAILYRED-CONF-V2\x00"), make([]byte, 9)...)
}

func TestReadROM(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(testROM()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var oversized bytes.Buffer
	writer = gzip.NewWriter(&oversized)
	if _, err := writer.Write(make([]byte, maxROMSize+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
		ok   bool
	}{
		{"raw", testROM(), true},
		{"gzip", compressed.Bytes(), true},
		{"retail", []byte("not Daily Red"), false},
		{"truncated config", []byte("DAILYRED-CONF-V2\x00"), false},
		{"corrupt gzip", []byte{0x1f, 0x8b, 0}, false},
		{"oversized", make([]byte, maxROMSize+1), false},
		{"oversized gzip", oversized.Bytes(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rom")
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			data, err := readROM(path)
			if (err == nil) != tc.ok {
				t.Fatalf("readROM error = %v, want success %v", err, tc.ok)
			}
			if tc.ok && !bytes.Equal(data, testROM()) {
				t.Fatal("ROM bytes changed")
			}
		})
	}
}

func TestFindROM(t *testing.T) {
	dir := t.TempDir()
	if _, err := findROM(dir); err == nil {
		t.Fatal("missing ROM was accepted")
	}
	path := filepath.Join(dir, "dailyred.gba")
	if err := os.WriteFile(path, testROM(), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := findROM(dir)
	if err != nil || got != path {
		t.Fatalf("findROM = %q, %v", got, err)
	}
}

func TestBundledAssets(t *testing.T) {
	for _, name := range []string{
		"index.html", "app.js", "licenses.txt",
		"vendor/nostalgist.js", "vendor/mgba_libretro.js", "vendor/mgba_libretro.wasm",
		"vendor/retroarch.COPYING.txt", "vendor/mgba.LICENSE.txt",
	} {
		if _, err := fs.Stat(assets, "assets/"+name); err != nil {
			t.Fatalf("missing offline asset %s: %v", name, err)
		}
	}
	if err := fs.WalkDir(assets, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "assets/game" || strings.HasSuffix(path, ".gba") || strings.HasSuffix(path, ".gba.gz") {
			t.Errorf("game content must not be bundled: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGameHandler(t *testing.T) {
	files := fstest.MapFS{
		"index.html": {Data: []byte(`<img src="game/red.png" alt="RED">`)},
		"app.js":     {Data: []byte("export {};")},
	}
	host := "127.0.0.1:17893"
	handler := gameHandler(files, testROM(), host)
	for _, tc := range []struct {
		path, method, host string
		status             int
	}{
		{"/", "GET", host, 200},
		{"/app.js", "GET", host, 200},
		{"/game/dailyred.gba.gz", "GET", host, 200},
		{"/game/dailyred.gba.gz", "HEAD", host, 200},
		{"/missing", "GET", host, 404},
		{"/", "POST", host, 405},
		{"/", "GET", "untrusted.example", 403},
	} {
		t.Run(tc.method+" "+tc.path+" "+tc.host, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			body, err := io.ReadAll(response.Result().Body)
			if err != nil {
				t.Fatal(err)
			}
			if tc.status == 200 && tc.method == "GET" && tc.path == "/" && !strings.Contains(string(body), `src="favicon.svg"`) {
				t.Fatal("standalone title card depends on unbundled game sprite")
			}
			if tc.status == 200 && tc.method == "GET" && tc.path == "/game/dailyred.gba.gz" && !bytes.Equal(body, testROM()) {
				t.Fatal("served ROM differs from supplied ROM")
			}
		})
	}
}
