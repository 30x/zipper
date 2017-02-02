package zipper

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"archive/zip"
	"encoding/binary"
	"path/filepath"
)

const CentralDirectoryFileHeaderLen = 46
const EOCDLen = 22

type Options struct {
	ExcludeBaseDir bool
}

// // Get a uint32 from a offset of a byte array
func getSize(b []byte, offset int) uint32 {
	var size uint32
	bTmp := []byte{b[offset], b[offset+1], b[offset+2], b[offset+3]}
	buf := bytes.NewReader(bTmp)
	binary.Read(buf, binary.LittleEndian, &size)
	return size
}

// Get a uint16 from a offset of a byte array
func getFieldLen(b []byte, offset int) uint16 {
	var size uint16
	bTmp := []byte{b[offset], b[offset+1]}
	buf := bytes.NewReader(bTmp)
	binary.Read(buf, binary.LittleEndian, &size)
	return size
}

// Process to fix zip file so Java ZipInputStream can read file
// http://webmail.dev411.com/p/gg/golang-nuts/155g3s6g53/go-nuts-re-zip-files-created-with-archive-zip-arent-recognised-as-zip-files-by-java-util-zip
func Process(source, target string) error {

	b, err := ioutil.ReadFile(source)
	if err != nil {
		return err
	}

	// new output buffer
	bOut := make([]byte, 0)

	// Keep track of all Local Header offsets use them rewrite "Relative offset of local file header" in Central directory file header
	var localHeaderOffsets []uint32

	headerOffset := -1
	startOfCentralDir := 0
	for idx := 0; idx < len(b); idx++ {
		// Find each Local file header signature = 0x04034b50 (read as a little-endian number)
		if b[idx] == 0x50 && b[idx+1] == 0x4b && b[idx+2] == 0x03 && b[idx+3] == 0x04 {
			headerOffset = idx

		}

		// Find Optional data descriptor signature = 0x08074b50 then backtrack from last headerOffset
		if b[idx] == 0x50 && b[idx+1] == 0x4b && b[idx+2] == 0x07 && b[idx+3] == 0x08 {
			// set byte 7 to 00
			b[headerOffset+6] = 0x00

			// modify CRC in local file header
			b[headerOffset+14] = b[idx+4]
			b[headerOffset+15] = b[idx+5]
			b[headerOffset+16] = b[idx+6]
			b[headerOffset+17] = b[idx+7]

			// modify compressed len in local file header
			b[headerOffset+18] = b[idx+8]
			b[headerOffset+19] = b[idx+9]
			b[headerOffset+20] = b[idx+10]
			b[headerOffset+21] = b[idx+11]

			// modify uncompressed len in local file header
			b[headerOffset+22] = b[idx+12]
			b[headerOffset+23] = b[idx+13]
			b[headerOffset+24] = b[idx+14]
			b[headerOffset+25] = b[idx+15]

			// Keep track of header offsets to rewrite "Relative offset of local file header" in Central directory file header
			localHeaderOffsets = append(localHeaderOffsets, uint32(len(bOut)))

			for j := headerOffset; j < idx; j++ {
				bOut = append(bOut, b[j])
			}
		}

		// Find the first Central directory file header - Central directory file header signature = 0x02014b5
		if b[idx] == 0x50 && b[idx+1] == 0x4b && b[idx+2] == 0x01 && b[idx+3] == 0x02 {
			// Mark the start of the Central directory
			if startOfCentralDir == 0 {
				startOfCentralDir = len(bOut)
			}

			// Shift off the each header offset
			offset := localHeaderOffsets[0]
			localHeaderOffsets = localHeaderOffsets[1:]

			// Update it's relative offset of local file header.
			bs := make([]byte, 4)
			binary.LittleEndian.PutUint32(bs, offset)
			b[idx+42] = bs[0]
			b[idx+43] = bs[1]
			b[idx+44] = bs[2]
			b[idx+45] = bs[3]

			fileNameLength := getFieldLen(b, idx+28)
			extraFieldLength := getFieldLen(b, idx+30)
			fileCommentLength := getFieldLen(b, idx+32)

			for j := idx; j < idx+CentralDirectoryFileHeaderLen+int(fileNameLength)+int(extraFieldLength)+int(fileCommentLength); j++ {
				bOut = append(bOut, b[j])
			}
		}

		// Locate the End of central directory record (EOCD)
		if b[idx] == 0x50 && b[idx+1] == 0x4b && b[idx+2] == 0x05 && b[idx+3] == 0x06 {

			// Update the Offset of start of central directory, relative to start of archive
			bs := make([]byte, 4)
			binary.LittleEndian.PutUint32(bs, uint32(startOfCentralDir))
			b[idx+16] = bs[0]
			b[idx+17] = bs[1]
			b[idx+18] = bs[2]
			b[idx+19] = bs[3]

			commentLength := getFieldLen(b, idx+20)
			for j := idx; j < idx+EOCDLen+int(commentLength); j++ {
				bOut = append(bOut, b[j])
			}
		}
	}

	err = ioutil.WriteFile(target, bOut, 0644)
	if err != nil {
		return err
	}

	return nil
}

// Archive processes the file headers after zipping
func Archive(source, target string, options Options) error {
	err := zipsource(source, target, options)
	if err != nil {
		return err
	}

	return Process(target, target)
}

// ArchiveUnprocessed does not process the file headers after zipping.
func ArchiveUnprocessed(source, target string, options Options) error {
	return zipsource(source, target, options)
}

func zipsource(source, target string, options Options) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	var baseDir string

	if !options.ExcludeBaseDir {
		info, err := os.Stat(source)
		if err != nil {
			return err
		}

		if info.IsDir() {
			baseDir = filepath.Base(source)
		}
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})

	return err
}
