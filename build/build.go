//go:generate go run .

package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/CarsonSlovoka/print-pdf/app"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func ZipSource() error {
	zipName := fmt.Sprintf("../bin/%s_%s_%s_%s.zip", app.Name,
		runtime.GOOS, runtime.GOARCH, app.Version)
	f, err := os.Create(zipName)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()

		bytesZip, _ := os.ReadFile(zipName)
		hasher256 := sha256.New()
		hasher256.Write(bytesZip)
		hashStr := strings.ToUpper(hex.EncodeToString(hasher256.Sum(nil)))
		hashContent := fmt.Sprintf("hashes.sha256\n\n%s\n%s\n", filepath.Base(zipName), hashStr)
		err = os.WriteFile("../bin/hash.md", []byte(hashContent), 0666)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("zipSource done!")
	}()

	zipWriter := zip.NewWriter(f)
	defer func() {
		log.Println("closing zip archive...")
		_ = zipWriter.Close()
	}()

	var w io.Writer
	for _, d := range []struct {
		srcPath string
		outPath string
	}{
		{fmt.Sprintf("../bin/%s.exe", app.Name), app.Name + ".exe"},
	} {
		log.Printf("opening %q...\n", d.srcPath)
		f, err = os.Open(d.srcPath)
		if err != nil {
			return err
		}
		log.Printf("writing %q to archive...\n", d.outPath)
		w, err = zipWriter.Create(d.outPath)
		if err != nil {
			return err
		}
		if _, err = io.Copy(w, f); err != nil {
			return err
		}
		_ = f.Close()
	}
	return nil
}

func Cmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println(strings.Join(cmd.Args, " "))
	return cmd
}

func main() {
	cmd := Cmd("go",
		"build",
		"-ldflags", "-s -w",
		"-tags", "tmpl",
		"-o", "./bin/"+app.Name+".exe",
		"-pkgdir", ".",
	)
	cmd.Dir = ".."
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	if err := ZipSource(); err != nil {
		log.Fatal(err)
	}
}
