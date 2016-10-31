package zipper

import (
	"os"
	"io"
	"io/ioutil"
	"strings"

	"path/filepath"
	"archive/zip"
	"encoding/binary"
)




// Process to fix zip file so Java ZipInputStream can read file
// http://webmail.dev411.com/p/gg/golang-nuts/155g3s6g53/go-nuts-re-zip-files-created-with-archive-zip-arent-recognised-as-zip-files-by-java-util-zip
func Process(source, target string) error {

	b, err := ioutil.ReadFile(source)
	if err != nil {
		return err
	}

	// new output buffer
	bOut := make([]byte, 0)

	// Locate all Data descriptor blocks

	headerOffset, dataDescriptorOffset := -1, -1
	var startOfCentraDir uint32
	startOfCentraDir = 0
	bOutIdxOffsetForEod := 0
	for idx := 0; idx < len(b); idx++ {
		if (b[idx] == 0x50 && b[idx+1] == 0x4b && b[idx+2] == 0x03 && b[idx+3] == 0x04) {
			headerOffset = idx
		}
		
		if (b[idx] == 0x50 && b[idx+1] == 0x4b && b[idx+2] == 0x07 && b[idx+3] == 0x08) {
			dataDescriptorOffset = idx

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

			for j := headerOffset; j < dataDescriptorOffset; j++ {
				bOut = append(bOut, b[j])
			}
			
		}

		if (startOfCentraDir == 0 && b[idx] == 0x50 && b[idx+1] == 0x4b && b[idx+2] == 0x01 && b[idx+3] == 0x02) {
			// Write rest of file to bOut
			startOfCentraDir = (uint32)(idx)
			bOutIdxOffsetForEod = len(bOut)
			for j := idx; j < len(b); j++ {
				bOut = append(bOut, b[j])
			}
			break;
		}
	}

	for idx := bOutIdxOffsetForEod; idx < len(bOut); idx++ {
		if (bOut[idx] == 0x50 && bOut[idx+1] == 0x4b && bOut[idx+2] == 0x05 && bOut[idx+3] == 0x06) {
			bs := make([]byte, 4)
			binary.LittleEndian.PutUint32(bs, startOfCentraDir)
			bOut[idx+16] = bs[0]
			bOut[idx+17] = bs[1]
			bOut[idx+18] = bs[2]
			bOut[idx+19] = bs[3]
		}
	}

	
	err = ioutil.WriteFile(target, bOut, 0644)
	if (err != nil) {
		return err
	}
	
	return nil
}

func Archive(source, target string) error {
	err := zipsource(source, target)
	if err != nil {
		return err
	}

	return Process(target, target)
}

func zipsource(source, target string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
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
