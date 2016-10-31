# Zipper

Golang Lib to create a Zip Archive of a file or folder. That will be compatible with Java's ZipInputStream

See: http://webmail.dev411.com/p/gg/golang-nuts/155g3s6g53/go-nuts-re-zip-files-created-with-archive-zip-arent-recognised-as-zip-files-by-java-util-zip

### Usage

```go

import (
       "log"

       "github.com/30x/zipper"
)

func main() {
  err := zipper.Archive("~/Source/dir/", "./output.zip")
  if err != nil {
     log.Fatal(err)
  }
}

```
