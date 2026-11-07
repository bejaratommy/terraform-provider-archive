// Copyright IBM Corp. 2017, 2026
// SPDX-License-Identifier: MPL-2.0

package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	r "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestArchivePreserveFileModTime(t *testing.T) {
	for _, archiveType := range []string{"zip", "tar.gz"} {
		for _, source := range []string{"file", "dir"} {
			for _, preserve := range []bool{false, true} {
				t.Run(archiveType+"/"+source+"/"+strconv.FormatBool(preserve), func(t *testing.T) {
					dir := t.TempDir()
					filename := filepath.Join(dir, "content.txt")
					if err := os.WriteFile(filename, []byte("test content"), 0600); err != nil {
						t.Fatal(err)
					}
					want := time.Date(2020, time.January, 2, 3, 4, 6, 0, time.UTC)
					if err := os.Chtimes(filename, want, want); err != nil {
						t.Fatal(err)
					}
					output := filepath.Join(t.TempDir(), "archive."+archiveType)
					model := fileModel{
						Type:                types.StringValue(archiveType),
						OutputPath:          types.StringValue(output),
						SourceFile:          types.StringNull(),
						SourceDir:           types.StringNull(),
						PreserveFileModTime: types.BoolValue(preserve),
					}
					if source == "file" {
						model.SourceFile = types.StringValue(filename)
					} else {
						model.SourceDir = types.StringValue(dir)
					}
					if err := archive(context.Background(), model); err != nil {
						t.Fatal(err)
					}
					var got time.Time
					if archiveType == "zip" {
						reader, err := zip.OpenReader(output)
						if err != nil {
							t.Fatal(err)
						}
						defer reader.Close()
						if len(reader.File) != 1 {
							t.Fatalf("expected one file, got %d", len(reader.File))
						}
						got = reader.File[0].Modified
					} else {
						file, err := os.Open(output)
						if err != nil {
							t.Fatal(err)
						}
						defer file.Close()
						reader, err := gzip.NewReader(file)
						if err != nil {
							t.Fatal(err)
						}
						defer reader.Close()
						header, err := tar.NewReader(reader).Next()
						if err != nil {
							t.Fatal(err)
						}
						got = header.ModTime
					}
					if got.Equal(want) != preserve {
						t.Fatalf("preserve_file_modtime=%t: got timestamp %s, source timestamp %s", preserve, got, want)
					}
				})
			}
		}
	}
}

func TestAccArchiveFile_PreserveFileModTime(t *testing.T) {
	for _, kind := range []string{"data", "resource"} {
		for _, archiveType := range []string{"zip", "tar.gz"} {
			t.Run(kind+"/"+archiveType, func(t *testing.T) {
				filename := filepath.Join(t.TempDir(), "content.txt")
				if err := os.WriteFile(filename, []byte("test content"), 0600); err != nil {
					t.Fatal(err)
				}
				output := filepath.Join(t.TempDir(), "archive."+archiveType)
				address := "archive_file.test"
				if kind == "data" {
					address = "data." + address
				}
				steps := make([]r.TestStep, 0, 2)
				for _, preserve := range []bool{true, false} {
					steps = append(steps, r.TestStep{
						Config: fmt.Sprintf(`
%s "archive_file" "test" {
 type = %q
 source_file = %q
 output_path = %q
 preserve_file_modtime = %t
}
`, kind, archiveType, filepath.ToSlash(filename), filepath.ToSlash(output), preserve),
						Check: r.TestCheckResourceAttr(address, "preserve_file_modtime", strconv.FormatBool(preserve)),
					})
				}
				r.Test(t, r.TestCase{
					ProtoV5ProviderFactories: protoV5ProviderFactories(),
					Steps:                    steps,
				})
			})
		}
	}
}
