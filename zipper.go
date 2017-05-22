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

type ZipEndLocator struct {
	signaturePos		uint32
	elDirectorySize		uint32
	elDirectoryOffSet	uint32
	elCommentLength 	uint16
}

type ZipDirEntry struct {
	signaturePos		uint32
	deCompressedSize	uint32
	fileNameLength		uint16
	extraFieldLength	uint16
	fileCommentLength	uint16
	localHeaderOffset	uint32
}

type ZipFileHeader struct {
	signaturePos		uint32
	dataDescSignPos		uint32
	frCrc				uint32
	frCompressedSize	uint32
	frUncompressedSize	uint32
}

// Get a uint16 from a offset of a byte array
func getUint16Len(b []byte, offset int) uint16 {
	var size uint16
	bTmp := []byte{b[offset], b[offset+1]}
	buf := bytes.NewReader(bTmp)
	binary.Read(buf, binary.LittleEndian, &size)
	return size
}

// Get a uint16 from a 	offset of a byte array
func getUint32Len(b []byte, offset int) uint32 {
	var size uint32
	bTmp := []byte{b[offset], b[offset+1], b[offset+2], b[offset+3]}
	buf := bytes.NewReader(bTmp)
	binary.Read(buf, binary.LittleEndian, &size)
	return size
}

func MyProcess(source string, target string) error {
	b, err := ioutil.ReadFile(source)
	if err != nil {
		return err
	}

	endLocator := ZipEndLocator{}
	for idx := len(b) - 1; idx >= 0; idx-- {
		// Locate the Last End of central directory record (EOCD)
		if b[idx] == 0x06 && b[idx-1] == 0x05 && b[idx-2] == 0x4b && b[idx-3] == 0x50 {
			endLocator.signaturePos = uint32(idx - 3)
			endLocator.elDirectorySize = getUint32Len(b, int(endLocator.signaturePos + 12))
			endLocator.elDirectoryOffSet = getUint32Len(b, int(endLocator.signaturePos + 16))
			endLocator.elCommentLength = getUint16Len(b, int(endLocator.signaturePos + 20))

			// fmt.Printf("endLocator = %v\n", endLocator)
			break
		}
	}

	var dirEntrys []ZipDirEntry

	for idx := int(endLocator.elDirectoryOffSet); idx < int(endLocator.elDirectoryOffSet + endLocator.elDirectorySize); idx++ {
		// Find the first Central directory file header - Central directory file header signature = 0x02014b5
		if b[idx] == 0x50 && b[idx+1] == 0x4b && b[idx+2] == 0x01 && b[idx+3] == 0x02 {
			tmpEntry := ZipDirEntry{}
			tmpEntry.signaturePos = uint32(idx)
			tmpEntry.deCompressedSize = getUint32Len(b, idx + 20)
			tmpEntry.fileNameLength = getUint16Len(b, idx+28)
			tmpEntry.extraFieldLength = getUint16Len(b, idx+30)
			tmpEntry.fileCommentLength = getUint16Len(b, idx+32)
			tmpEntry.localHeaderOffset = getUint32Len(b, idx + 42)
			dirEntrys = append(dirEntrys, tmpEntry)

			// fmt.Printf("tmpEntry = %v\n", tmpEntry)
		}
	}

	var fileHeaders []ZipFileHeader
	for _, entry := range dirEntrys {
		idx := int(entry.localHeaderOffset + 30 + uint32(entry.fileNameLength) + uint32(entry.extraFieldLength) + entry.deCompressedSize)

		tmpFileHeader := ZipFileHeader{}
		tmpFileHeader.signaturePos = entry.localHeaderOffset
		tmpFileHeader.dataDescSignPos = uint32(idx)
		tmpFileHeader.frCrc = getUint32Len(b, idx + 4)
		tmpFileHeader.frCompressedSize = getUint32Len(b, idx + 8)
		tmpFileHeader.frUncompressedSize = getUint32Len(b, idx + 12)
		fileHeaders = append(fileHeaders, tmpFileHeader)

		// fmt.Printf("tmpFileHeader = %v\n", tmpFileHeader)
	}

	// new output buffer
	bOut := make([]byte, 0)
	var localHeaderOffsets []uint32

	for _, tmpHeader := range fileHeaders {
		headerOffset := tmpHeader.signaturePos

		// set byte 7 to 00
		b[headerOffset+6] = 0x00

		// modify CRC in local file header
		bs := make([]byte, 4)
		binary.LittleEndian.PutUint32(bs, tmpHeader.frCrc)

		b[headerOffset+14] = bs[0]
		b[headerOffset+15] = bs[1]
		b[headerOffset+16] = bs[2]
		b[headerOffset+17] = bs[3]

		// modify compressed len in local file header
		binary.LittleEndian.PutUint32(bs, tmpHeader.frCompressedSize)

		b[headerOffset+18] = bs[0]
		b[headerOffset+19] = bs[1]
		b[headerOffset+20] = bs[2]
		b[headerOffset+21] = bs[3]

		// modify uncompressed len in local file header
		binary.LittleEndian.PutUint32(bs, tmpHeader.frUncompressedSize)

		b[headerOffset+22] = bs[0]
		b[headerOffset+23] = bs[1]
		b[headerOffset+24] = bs[2]
		b[headerOffset+25] = bs[3]

		// Keep track of header offsets to rewrite "Relative offset of local file header" in Central directory file header
		localHeaderOffsets = append(localHeaderOffsets, uint32(len(bOut)))

		for j := headerOffset; j < tmpHeader.dataDescSignPos; j++ {
			bOut = append(bOut, b[j])
		}
	}

	startOfCentralDir := 0
	for _, tmpDirEntry := range dirEntrys {
		// Mark the start of the Central directory
		if startOfCentralDir == 0 {
			startOfCentralDir = len(bOut)
		}

		// Shift off the each header offset
		offset := localHeaderOffsets[0]
		localHeaderOffsets = localHeaderOffsets[1:]

		headerOffSet := tmpDirEntry.signaturePos

		// set General purpose bit flag to 0
		b[headerOffSet+8] = 0x00
		b[headerOffSet+9] = 0x00

		// Update it's relative offset of local file header.
		bs := make([]byte, 4)
		binary.LittleEndian.PutUint32(bs, offset)
		b[headerOffSet+42] = bs[0]
		b[headerOffSet+43] = bs[1]
		b[headerOffSet+44] = bs[2]
		b[headerOffSet+45] = bs[3]

		for j := headerOffSet; j < headerOffSet +
									CentralDirectoryFileHeaderLen +
									uint32(tmpDirEntry.fileNameLength) +
									uint32(tmpDirEntry.fileCommentLength) +
									uint32(tmpDirEntry.extraFieldLength); j++ {
			bOut = append(bOut, b[j])
		}
	}

	// Update the Offset of start of central directory, relative to start of archive
	headerOffSet := endLocator.signaturePos
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(startOfCentralDir))

	b[headerOffSet+16] = bs[0]
	b[headerOffSet+17] = bs[1]
	b[headerOffSet+18] = bs[2]
	b[headerOffSet+19] = bs[3]

	for j := headerOffSet; j < headerOffSet +
								EOCDLen +
								uint32(endLocator.elCommentLength); j++ {
		bOut = append(bOut, b[j])
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
