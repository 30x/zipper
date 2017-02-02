package zipper

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"testing"
)

// Hash from http://www.mrwaggel.be/post/generate-md5-hash-of-a-file/
func hash_file_md5(filePath string) (string, error) {
	//Initialize variable returnMD5String now in case an error has to be returned
	var returnMD5String string

	//Open the passed argument and check for any error
	file, err := os.Open(filePath)
	if err != nil {
		return returnMD5String, err
	}

	//Tell the program to call the following function when the current function returns
	defer file.Close()

	//Open a new hash interface to write to
	hash := md5.New()

	//Copy the file in the hash interface and check for any error
	if _, err := io.Copy(hash, file); err != nil {
		return returnMD5String, err
	}

	//Get the 16 bytes hash
	hashInBytes := hash.Sum(nil)[:16]

	//Convert the bytes to a string
	returnMD5String = hex.EncodeToString(hashInBytes)

	return returnMD5String, nil

}

func TestArchive(t *testing.T) {
	err := Archive("zip-src/", "myProxy.zip", Options{})
	if err != nil {
		t.Fatalf("Failed to create zip archive.", err)
	}

	hash, _ := hash_file_md5("myProxy.zip")
	if hash != "fd1a1204cfc5d05e4ebcece6beb28288" {
		t.Fatalf("Zip hash did not match.", hash)
	}

}
