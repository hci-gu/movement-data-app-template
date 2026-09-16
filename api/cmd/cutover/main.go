// cutover converts a stopped installation into a new directory, leaving its source untouched.
package main

import (
	"app/internal/cutover"
	"app/internal/study"
	"database/sql"
	"flag"
	"fmt"
	"github.com/pocketbase/pocketbase"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Cutover failed:", err)
		os.Exit(1)
	}
}
func run() error {
	source := flag.String("source", "", "Stopped source PocketBase data directory")
	output := flag.String("out", "", "New output directory (must not exist)")
	flag.Parse()
	if *source == "" || *output == "" {
		return fmt.Errorf("provide --source and --out; stop the server before copying")
	}
	src, err := filepath.Abs(*source)
	if err != nil {
		return err
	}
	dst, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	if dst == src || strings.HasPrefix(dst, src+string(os.PathSeparator)) {
		return fmt.Errorf("output must be outside the source directory")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		return fmt.Errorf("output already exists or is inaccessible")
	}
	cfg, err := study.LoadConfig()
	if err != nil {
		return err
	}
	if err := os.Mkdir(dst, 0700); err != nil {
		return err
	}
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink %s", rel)
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if strings.HasSuffix(path, ".db-wal") || strings.HasSuffix(path, ".db-shm") {
			return nil
		}
		if strings.HasSuffix(path, ".db") {
			db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = db.Exec("VACUUM INTO ?", target)
			if err == nil {
				err = os.Chmod(target, 0600)
			}
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		closeErr := out.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: dst})
	if err := app.Bootstrap(); err != nil {
		return err
	}
	defer app.ResetBootstrapState()
	if err := cutover.Convert(app, cfg); err != nil {
		return err
	}
	fmt.Println("Converted copy ready:", dst)
	fmt.Println("Source preserved. Start the new release against the converted directory.")
	return nil
}
