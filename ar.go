package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

type ArReader struct {
	r              io.Reader
	lastFileReader io.Reader
}

type FileInfo struct {
	Name  string
	Mod   time.Time
	Owner int
	Group int
	Mode  os.FileMode
	Size  int64

	Reader io.Reader
}

func NewArReader(r io.Reader) *ArReader {
	return &ArReader{
		r: r,
	}
}

func (a *ArReader) ReadFile() (*FileInfo, error) {
	// scan to the end of the previous file, if any
	if a.lastFileReader != nil {
		_, err := io.Copy(io.Discard, a.lastFileReader)
		if err != nil {
			return nil, err
		}
		a.lastFileReader = nil
	}

	// filename: 16
	// modification: 12
	// owner: 6
	// gid: 6
	// filemode: 8
	// size: 10
	// trailer: 2
	const headerSize = 60
	var fileHeader bytes.Buffer
	read, err := io.Copy(&fileHeader, io.LimitReader(a.r, headerSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	if read == 0 {
		return nil, io.EOF
	}
	if read != headerSize {
		return nil, fmt.Errorf("failed to read complete file header, expected %d bytes, got %d bytes", headerSize, read)
	}

	var fi FileInfo

	fi.Name = strings.TrimSuffix(strings.TrimSpace(string(fileHeader.Bytes()[0:16])), "/")

	mod, err := strconv.ParseInt(strings.TrimSpace(string(fileHeader.Bytes()[16:16+12])), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to read file mod: %w", err)
	}
	fi.Mod = time.Unix(mod, 0)

	fi.Owner, err = strconv.Atoi(strings.TrimSpace(string(fileHeader.Bytes()[28 : 28+6])))
	if err != nil {
		return nil, fmt.Errorf("failed to read file owner: %w", err)
	}

	fi.Group, err = strconv.Atoi(strings.TrimSpace(string(fileHeader.Bytes()[34 : 34+6])))
	if err != nil {
		return nil, fmt.Errorf("failed to read file group: %w", err)
	}

	mode, err := strconv.ParseInt(strings.TrimSpace(string(fileHeader.Bytes()[40:40+8])), 8, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to read file mode: %w", err)
	}
	fi.Mode = os.FileMode(mode)

	fi.Size, err = strconv.ParseInt(strings.TrimSpace(string(fileHeader.Bytes()[48:48+10])), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to read file size: %w", err)
	}

	if bytes.Compare(fileHeader.Bytes()[58:58+2], []byte{0x60, 0x0a}) != 0 {
		return nil, errors.New("invalid file info trailer")
	}

	fi.Reader = io.LimitReader(a.r, fi.Size)
	a.lastFileReader = fi.Reader

	return &fi, nil
}