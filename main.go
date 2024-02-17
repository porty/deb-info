package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gabriel-vasile/mimetype"

	"github.com/porty/deb-info/ar"
)

func main() {
	if err := errmain(); err != nil {
		panic(err)
	}
}

const signature = "!<arch>\n"

type jsonResult struct {
	Control map[string]string `json:"control"`
	Data    []*FileInfo       `json:"data"`
}

func errmain() error {
	jsonOutput := flag.Bool("json", false, "Output as JSON")
	flag.Parse()

	filename := flag.Arg(0)

	var r io.Reader

	if filename == "" {
		r = os.Stdin
	} else if strings.HasPrefix(filename, "http://") || strings.HasPrefix(filename, "https://") {
		rc, err := openHTTP(filename)
		if err != nil {
			return err
		}
		defer rc.Close()
		r = rc
	} else {
		f, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("failed to open Debian package: %w", err)
		}
		defer f.Close()
		r = f
	}

	var buf bytes.Buffer
	_, err := io.Copy(&buf, io.LimitReader(r, 8))
	if err != nil {
		return fmt.Errorf("failed to read Debian package header: %w", err)
	}
	if !bytes.Equal(buf.Bytes(), []byte(signature)) {
		return errors.New("bad Debian package signature")
	}

	//fmt.Println("seems OK")

	ar := ar.NewReader(r)

	if err := readDebianBinary(ar); err != nil {
		return fmt.Errorf("failed to read debian-binary: %w", err)
	}

	control, err := readControl(ar)
	if err != nil {
		return fmt.Errorf("failed to read control file: %w", err)
	}

	if !*jsonOutput {
		fmt.Println(control)
		err = readDataToStdout(ar)
		if err != nil {
			return fmt.Errorf("failed to read data file: %w", err)
		}
	} else {
		controlMap, err := controlToMap(control)
		if err != nil {
			return err
		}
		files, err := readDataToSlice(ar)
		if err != nil {
			return fmt.Errorf("failed to read data file: %w", err)
		}

		out := jsonResult{
			Control: controlMap,
			Data:    files,
		}
		_ = json.NewEncoder(os.Stdout).Encode(out)
	}

	return nil
}

func openHTTP(filename string) (io.ReadCloser, error) {
	c := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			Proxy:                 http.ProxyFromEnvironment,
		},
	}
	resp, err := c.Get(filename)
	if err != nil {
		return nil, fmt.Errorf("http request failed for package: %w", err)
	}

	if access, domain := isCloudflareAccessRedirect(resp); access {
		log.Printf("Doing Cloudflare Access dance with %s", domain)
		resp.Body.Close()

		token, err := getAccessToken(domain)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequest(http.MethodGet, filename, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		log.Printf("THE TOKEN IS %q", token)
		req.Header.Set("cf-access-token", token)
		resp, err = c.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed for package behind Cloudflare Access: %w", err)
		}
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected HTTP status for package: %s", resp.Status)
	}

	return resp.Body, nil
}

func readDebianBinary(ar *ar.Reader) error {
	fi, err := ar.ReadFile()
	if err != nil {
		return err
	}
	const debianBinary = "debian-binary"
	if fi.Name != debianBinary {
		return fmt.Errorf("expected file %q, got %q", debianBinary, fi.Name)
	}
	if fi.Size != 4 {
		return fmt.Errorf("expected size %d, got %q", 4, fi.Size)
	}
	var buf bytes.Buffer

	read, err := io.Copy(&buf, fi.Reader)
	if err != nil {
		return fmt.Errorf("failed to read: %w", err)
	}
	if read != 4 {
		return fmt.Errorf("failed to read: expected %d bytes, read %d bytes", 4, read)
	}
	if buf.String() != "2.0\n" {
		return errors.New("invalid debian-binary value")
	}
	return nil
}

func readControl(ar *ar.Reader) (string, error) {
	fi, err := ar.ReadFile()
	if err != nil {
		return "", err
	}
	const controlTarGz = "control.tar.gz"
	if fi.Name != controlTarGz {
		return "", fmt.Errorf("expected file %q, got %q", controlTarGz, fi.Name)
	}
	if fi.Size > 100*1024 {
		return "", fmt.Errorf("control archive seems to large at %d bytes", fi.Size)
	}

	gr, err := gzip.NewReader(fi.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to initialize gzip reader: %w", err)
	}
	gr.Close()

	tr := tar.NewReader(gr)
	for {
		tarFile, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read file from control archive: %w", err)
		}

		if tarFile.Name == "./control" {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				return "", fmt.Errorf("failed to read control file: %w", err)
			}
			return buf.String(), nil
		}
	}

	return "", errors.New("failed to find control file in control archive")
}

func controlToMap(control string) (map[string]string, error) {

	lines := strings.Split(control, "\n")
	m := map[string]string{}
	var lastKey string

	for _, line := range lines {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, " ") {
			// multi line value
			if lastKey == "" {
				return nil, errors.New("bad continuation line")
			}
			m[lastKey] = m[lastKey] + "\n" + strings.TrimPrefix(line, " ")
			continue
		}

		// normal line
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("failed to read control: invalid line %q", line)
		}
		if _, exists := m[parts[0]]; exists {
			return nil, fmt.Errorf("failed to read control: duplicate %q", parts[0])
		}
		m[parts[0]] = strings.TrimSpace(parts[1])
		lastKey = parts[0]
	}

	return m, nil
}

func readDataToStdout(ar *ar.Reader) error {
	fi, err := ar.ReadFile()
	if err != nil {
		return err
	}
	// TODO: handle data.tar.bz
	const controlTarGz = "data.tar.gz"
	if fi.Name != controlTarGz {
		return fmt.Errorf("expected file %q, got %q", controlTarGz, fi.Name)
	}

	gr, err := gzip.NewReader(fi.Reader)
	if err != nil {
		return fmt.Errorf("failed to initialize gzip reader: %w", err)
	}
	gr.Close()

	w := tabwriter.NewWriter(os.Stdout, 10, 2, 4, ' ', 0)
	//fmt.Printf("%-100s %-13s %-13s %-20s\n", "Name", "Mode", "Size", "MIME")
	w.Write([]byte("Name\tMode\tSize\tMIME\n"))

	fileCount := 0
	dirCount := 0
	totalFileSize := int64(0)

	tr := tar.NewReader(gr)
	for {
		f, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read file from data archive: %w", err)
		}

		mimeStr := ""
		switch f.Typeflag {
		case tar.TypeDir:
			dirCount++
			mimeStr = "<DIR>"
		case tar.TypeSymlink, tar.TypeLink:
			mimeStr = "-> " + f.Linkname
		case tar.TypeReg:
			fileCount++
			totalFileSize += f.Size
			if mime, _ := mimetype.DetectReader(tr); mime != nil {
				mimeStr = mime.String()
			} else {
				mimeStr = "(unknown)"
			}
		default:
			mimeStr = "(not handled)"
		}

		fi := f.FileInfo()
		// fmt.Printf("%-100s %-13s %13s %-20s\n", f.Name, fi.Mode().String(), strconv.Itoa(int(f.Size)), mimeStr)
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", f.Name, fi.Mode().String(), f.Size, mimeStr)
	}
	w.Flush()

	fmt.Println()
	fmt.Printf("File count: %d\nDirectory count: %d\nTotal file size: %d\n", fileCount, dirCount, totalFileSize)

	return nil
}

type FileInfo struct {
	Name string `json:"name"`
	// directories will have Size=0, and will be omitted
	// empty files will also be omitted :(
	Size int64  `json:"size,omitempty"`
	Mode string `json:"mode,omitempty"`
	MIME string `json:"mime,omitempty"`
}

func readDataToSlice(ar *ar.Reader) ([]*FileInfo, error) {
	fi, err := ar.ReadFile()
	if err != nil {
		return nil, err
	}
	// TODO: handle data.tar.bz
	const controlTarGz = "data.tar.gz"
	if fi.Name != controlTarGz {
		return nil, fmt.Errorf("expected file %q, got %q", controlTarGz, fi.Name)
	}

	gr, err := gzip.NewReader(fi.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gzip reader: %w", err)
	}
	gr.Close()

	result := []*FileInfo{}

	tr := tar.NewReader(gr)
	for {
		f, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read file from data archive: %w", err)
		}

		mimeStr := ""
		switch f.Typeflag {
		case tar.TypeDir:
		case tar.TypeSymlink, tar.TypeLink:
			mimeStr = "-> " + f.Linkname
		case tar.TypeReg:
			if mime, _ := mimetype.DetectReader(tr); mime != nil {
				mimeStr = mime.String()
			}
		default:
			mimeStr = "???"
		}

		result = append(result, &FileInfo{
			Name: f.Name,
			Size: f.Size,
			Mode: f.FileInfo().Mode().String(),
			MIME: mimeStr,
		})
	}

	return result, nil
}
