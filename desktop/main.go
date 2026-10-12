package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"
)

//go:embed assets
var assets embed.FS

const maxROMSize = 32 << 20

func readROM(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxROMSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxROMSize {
		return nil, errors.New("ROM file exceeds 32 MiB")
	}
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("open compressed ROM: %w", err)
		}
		defer reader.Close()
		data, err = io.ReadAll(io.LimitReader(reader, maxROMSize+1))
		if err != nil {
			return nil, fmt.Errorf("decompress ROM: %w", err)
		}
	}
	if len(data) > maxROMSize {
		return nil, errors.New("decompressed ROM exceeds 32 MiB")
	}
	magic := []byte("DAILYRED-CONF-V2\x00")
	offset := bytes.Index(data, magic)
	if offset < 0 || offset+len(magic)+9 > len(data) {
		return nil, errors.New("not a Daily Red ROM; build one with scripts/build-rom.sh (a retail FireRed ROM will not work)")
	}
	return data, nil
}

func findROM(dir string) (string, error) {
	for _, name := range []string{"dailyred.gba.gz", "dailyred.gba"} {
		path := filepath.Join(dir, name)
		_, err := os.Stat(path)
		if err == nil {
			return path, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return "", errors.New("put your locally built dailyred.gba.gz or dailyred.gba next to DailyRed.exe, or use -rom PATH; the executable does not include a ROM")
}

func gameHandler(files fs.FS, rom []byte, host string) http.Handler {
	static := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host {
			http.Error(w, "Invalid host", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		if r.URL.Path == "/game/dailyred.gba.gz" {
			w.Header().Set("Content-Type", "application/octet-stream")
			http.ServeContent(w, r, "dailyred.gba", time.Time{}, bytes.NewReader(rom))
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			index, err := fs.ReadFile(files, "index.html")
			if err != nil {
				log.Printf("Read embedded index: %v", err)
				http.Error(w, "Website assets unavailable", http.StatusInternalServerError)
				return
			}
			// The generated RED sprite is game content, not bundled software.
			index = bytes.ReplaceAll(index, []byte(`src="game/red.png"`), []byte(`src="favicon.svg"`))
			index = bytes.ReplaceAll(index, []byte(`alt="RED"`), []byte(`alt="Daily Red"`))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
			return
		}
		static.ServeHTTP(w, r)
	})
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		command = exec.Command("open", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	return command.Run()
}

func run() error {
	romPath := flag.String("rom", "", "path to a locally built Daily Red ROM (.gba or .gba.gz)")
	port := flag.Int("port", 17893, "local server port (keep it unchanged to preserve browser stats)")
	noBrowser := flag.Bool("no-browser", false, "do not automatically open the browser")
	flag.Parse()
	if *port < 0 || *port > 65535 {
		return errors.New("port must be between 0 and 65535")
	}
	if *romPath == "" {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		*romPath, err = findROM(filepath.Dir(executable))
		if err != nil {
			return err
		}
	}
	rom, err := readROM(*romPath)
	if err != nil {
		return fmt.Errorf("load ROM: %w", err)
	}
	files, err := fs.Sub(assets, "assets")
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		return fmt.Errorf("start local server (another Daily Red window may be open; use -port to change the port): %w", err)
	}
	defer listener.Close()
	url := "http://" + listener.Addr().String()
	server := &http.Server{
		Handler:           gameHandler(files, rom, listener.Addr().String()),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Shutdown: %v", err)
		}
	}()
	fmt.Printf("Daily Red is ready at %s\nKeep this window open while playing. Press Ctrl+C to quit.\n", url)
	if !*noBrowser {
		go func() {
			if err := openBrowser(url); err != nil {
				log.Printf("Could not open the browser: %v. Open %s manually.", err, url)
			}
		}()
	}
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Daily Red:", err)
		if runtime.GOOS == "windows" {
			fmt.Fprintln(os.Stderr, "\nPress Enter to close.")
			_, _ = fmt.Scanln()
		}
		os.Exit(1)
	}
}
