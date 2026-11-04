package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const releases = "https://api.github.com/repos/scpwikicloud/scpper/releases/latest"

type release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func assetName() string {
	name := fmt.Sprintf("scpper_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func update(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	force := fs.Bool("force", false, "install even when the latest release is this version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	var rel release
	if err := getJSON(client, releases, &rel); err != nil {
		return err
	}
	if rel.Tag == version && !*force {
		fmt.Println("scpper is up to date at", version)
		return nil
	}
	want := assetName()
	var binURL, sumURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case want:
			binURL = a.URL
		case "checksums.txt":
			sumURL = a.URL
		}
	}
	if binURL == "" || sumURL == "" {
		return fmt.Errorf("release %s has no %s or checksums.txt", rel.Tag, want)
	}
	sum, err := checksum(client, sumURL, want)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	tmp := exe + ".new"
	if err := download(client, binURL, tmp, sum); err != nil {
		os.Remove(tmp)
		return err
	}
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Rename(old, exe)
		return err
	}
	fmt.Printf("updated scpper from %s to %s; restart it to run the new version\n", version, rel.Tag)
	return nil
}

func getJSON(client *http.Client, url string, out any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func checksum(client *http.Client, url, name string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksums.txt does not list %s", name)
}

func download(client *http.Client, url, path, sum string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		return errors.New("the download does not match its published checksum")
	}
	return nil
}
