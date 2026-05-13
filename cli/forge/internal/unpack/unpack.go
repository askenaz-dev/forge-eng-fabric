// Package unpack extracts an Agent Skills .tar.zst bundle into a target
// directory, rejecting unsafe entries (paths escaping root, symlinks, devices).
package unpack

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Into unpacks bundle bytes into destRoot. The bundle's single top-level
// folder is preserved (so destRoot becomes the parent containing
// <skill>/SKILL.md). Returns the name of that top-level folder.
func Into(bundle []byte, destRoot string) (string, error) {
	dec, err := zstd.NewReader(bytes.NewReader(bundle))
	if err != nil {
		return "", err
	}
	defer dec.Close()
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return "", err
	}
	tr := tar.NewReader(dec)
	cleanRoot, err := filepath.Abs(destRoot)
	if err != nil {
		return "", err
	}
	topLevel := ""
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeDir {
			return "", fmt.Errorf("refusing unsafe entry type %d for %s", hdr.Typeflag, hdr.Name)
		}
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return "", fmt.Errorf("entry %q escapes bundle root", hdr.Name)
		}
		// Track the top-level folder.
		parts := strings.SplitN(clean, string(os.PathSeparator), 2)
		if len(parts) > 0 && parts[0] != "" {
			if topLevel == "" {
				topLevel = parts[0]
			} else if topLevel != parts[0] {
				return "", fmt.Errorf("bundle has more than one top-level folder: %q and %q", topLevel, parts[0])
			}
		}
		target := filepath.Join(cleanRoot, clean)
		if !strings.HasPrefix(target, cleanRoot+string(os.PathSeparator)) && target != cleanRoot {
			return "", fmt.Errorf("entry %q escapes destRoot", hdr.Name)
		}
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		mode := os.FileMode(hdr.Mode & 0o755)
		if mode == 0 {
			mode = 0o644
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(f, tr); err != nil {
			_ = f.Close()
			return "", err
		}
		_ = f.Close()
	}
	if topLevel == "" {
		return "", errors.New("bundle is empty")
	}
	return topLevel, nil
}
