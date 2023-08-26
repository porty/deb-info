package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/gabriel-vasile/mimetype"
)

func main() {
	if err := errmain(); err != nil {
		panic(err)
	}
}

const signature = "!<arch>\n"

func errmain() error {
	flag.Parse()

	filename := flag.Arg(0)

	var r io.Reader

	if filename == "" {
		r = os.Stdin
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

	ar := NewArReader(r)

	if err := readDebianBinary(ar); err != nil {
		return fmt.Errorf("failed to read debian-binary: %w", err)
	}

	control, err := readControl(ar)
	if err != nil {
		return fmt.Errorf("failed to read control file: %w", err)
	}

	fmt.Println(control)

	err = readData(ar)
	if err != nil {
		return fmt.Errorf("failed to read data file: %w", err)
	}

	// for {
	// 	fi, err := ar.ReadFile()
	// 	if err == io.EOF {
	// 		break
	// 	}
	// 	if err != nil {
	// 		return err
	// 	}

	// 	fmt.Printf("File: %q, size %d\n", fi.Name, fi.Size)

	// 	if fi.Name == "debian-binary" {
	// 		buf.Reset()
	// 		read, err := io.Copy(&buf, fi.Reader)
	// 		if err != nil {
	// 			return fmt.Errorf("failed to read debian-binary: %w", err)
	// 		}
	// 		if read != 4 {
	// 			return fmt.Errorf("failed to read debian-binary: expected %d bytes, read %d bytes", 4, read)
	// 		}
	// 		if buf.String() != "2.0\n" {
	// 			return errors.New("invalid debian-binary value")
	// 		}
	// 		fmt.Println("hell yeah debian-binary looks good")
	// 	}
	// }

	return nil
}

func readDebianBinary(ar *ArReader) error {
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

func readControl(ar *ArReader) (string, error) {
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

func readData(ar *ArReader) error {
	fi, err := ar.ReadFile()
	if err != nil {
		return err
	}
	const controlTarGz = "data.tar.gz"
	if fi.Name != controlTarGz {
		return fmt.Errorf("expected file %q, got %q", controlTarGz, fi.Name)
	}

	gr, err := gzip.NewReader(fi.Reader)
	if err != nil {
		return fmt.Errorf("failed to initialize gzip reader: %w", err)
	}
	gr.Close()

	fmt.Printf("%-100s %-13s %-13s %-20s\n", "Name", "Mode", "Size", "MIME")

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
		fmt.Printf("%-100s %-13s %13s %-20s\n", f.Name, fi.Mode().String(), strconv.Itoa(int(f.Size)), mimeStr)
	}

	fmt.Println()
	fmt.Printf("File count: %d\nDirectory count: %d\nTotal file size: %d\n", fileCount, dirCount, totalFileSize)

	return nil
}
